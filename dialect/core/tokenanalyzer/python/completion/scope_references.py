"""
Scope-aware relation reference collection for completion and IDE features.

This module produces relation references with scope positions and CTE column
inference, matching the Go RelationRef structure. It replaces the Go-side
ReferencesStrategy.CollectReferences for dialects that use the Python analyzer.

Response shape:
  {
    "relations":      [...],   # physical + CTE usage refs
    "virtual_tables": [...],   # CTE/subquery definitions with inferred columns
  }
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.optimizer.scope import Scope, ScopeType, traverse_scope
from sqlglot.tokens import TokenType

from analysis.schema import pos, span, tokenize


class _ScopeBounds:
    """Pre-computed scope boundary character offsets from the SQLGlot tokenizer."""

    def __init__(self, sql: str, sg_dialect: str) -> None:
        self.sql = sql
        tokens = tokenize(sql, sg_dialect)

        # Matching paren pairs: maps open_offset -> close_offset and vice versa
        self.paren_pairs: dict[int, int] = {}
        stack: list[int] = []
        for t in tokens:
            if t.token_type == TokenType.L_PAREN:
                stack.append(t.start)
            elif t.token_type == TokenType.R_PAREN and stack:
                open_off = stack.pop()
                self.paren_pairs[open_off] = t.start
                self.paren_pairs[t.start] = open_off

        # Where every parenthesis stands, so a depth can be counted the way the
        # caller counts the caret's: over the text, not over sqlglot's scopes.
        self._opens = [t.start for t in tokens if t.token_type == TokenType.L_PAREN]

        # Semicolons
        self.semicolons = [t.start for t in tokens if t.token_type == TokenType.SEMICOLON]

        # SELECT/UPDATE keyword positions
        self.keywords: list[tuple[int, str]] = []
        for t in tokens:
            if t.token_type in (TokenType.SELECT, TokenType.UPDATE, TokenType.INSERT, TokenType.DELETE):
                self.keywords.append((t.start, t.token_type.name))

    def find_cte_body_range(self, cte_name_offset: int) -> tuple[int, int]:
        """Find the (open_paren_offset, close_paren_offset) of the CTE body AS (...)."""
        # Scan forward from the CTE name to find the first ( after AS
        i = cte_name_offset
        sql = self.sql
        while i < len(sql):
            if sql[i] == '(':
                close = self.paren_pairs.get(i, -1)
                return i, close
            i += 1
        return -1, -1

    def enclosing_paren(self, offset: int) -> int:
        """The innermost parenthesis still open at this offset, or -1. A CTE
        body closes before the statement that reads it begins, so the nearest
        one before the offset is a different question."""
        enclosing = -1
        for start in self._opens:
            if start >= offset:
                break
            close = self.paren_pairs.get(start, -1)
            if close < 0 or close > offset:
                enclosing = start
        return enclosing

    def range_from(self, offset: int) -> tuple[int, int]:
        """Where the scope opening at this offset runs: the parenthesis holding
        it, or the rest of the statement when nothing holds it."""
        open_paren = self.enclosing_paren(offset)
        if open_paren >= 0:
            return open_paren + 1, self.paren_pairs.get(open_paren, -1)
        return offset, self.find_statement_end(offset)

    def depth_at(self, offset: int) -> int:
        """How many parentheses stand open at this offset."""
        depth = 0
        for start in self._opens:
            if start >= offset:
                break
            close = self.paren_pairs.get(start, -1)
            if close < 0 or close > offset:
                depth += 1
        return depth

    def find_statement_end(self, offset: int) -> int:
        """Find the semicolon offset that ends the statement containing offset, or -1."""
        for sc in self.semicolons:
            if sc > offset:
                return sc
        return -1

    def find_select_before(self, offset: int) -> int:
        """Find the nearest SELECT/UPDATE keyword offset before the given offset."""
        best = -1
        # Also respect statement boundaries
        stmt_start = 0
        for sc in self.semicolons:
            if sc < offset:
                stmt_start = sc + 1
            else:
                break

        for kw_off, _ in self.keywords:
            if stmt_start <= kw_off < offset:
                best = kw_off
        return best if best >= 0 else 0


def collect_references(
    sql: str,
    stmts: list,
    schema_dict: dict,
    default_schema: str,
    sg_dialect: str,
) -> dict:
    """Collect scope-aware relation references from parsed statements."""
    relations: list[dict] = []
    virtual_tables: list[dict] = []
    seen_rels: set[tuple] = set()
    seen_vtabs: set[str] = set()
    bounds = _ScopeBounds(sql, sg_dialect)

    for stmt_idx, stmt in enumerate(stmts):
        if stmt is None:
            continue
        try:
            scopes = list(traverse_scope(stmt))
        except Exception:
            scopes = []

        root_refs_start = len(relations)
        if scopes:
            root_refs_start = _collect_from_scopes(
                scopes, schema_dict, default_schema,
                relations, virtual_tables, seen_rels, seen_vtabs,
                stmt_idx, bounds,
            )

        _collect_dml_target(
            stmt, scopes, schema_dict, default_schema,
            relations, virtual_tables, seen_rels, seen_vtabs, stmt_idx, bounds,
            insert_index=root_refs_start,
        )

    return {"relations": relations, "virtual_tables": virtual_tables}


def _collect_from_scopes(
    scopes: list,
    schema_dict: dict,
    default_schema: str,
    relations: list[dict],
    virtual_tables: list[dict],
    seen_rels: set[tuple],
    seen_vtabs: set[str],
    stmt_idx: int = 0,
    bounds: _ScopeBounds | None = None,
) -> int:
    """Collect relation references. Returns the index in relations where this
    statement's root level refs begin, which is where a DML target belongs.
    """
    # First pass: build CTE definitions and compute their body ranges
    cte_defs: dict[str, dict] = {}
    cte_ranges: dict[str, tuple[int, int]] = {}  # cte_name -> (open_paren_offset, close_paren_offset)
    for scope in scopes:
        if scope.scope_type == ScopeType.CTE:
            cte_node = scope.expression.parent
            if isinstance(cte_node, exp.CTE):
                cte_name = cte_node.alias
                if cte_name:
                    vtab = _build_virtual_table(
                        cte_name, scope, schema_dict, default_schema, cte_defs,
                        bounds,
                    )
                    cte_defs[cte_name.lower()] = vtab

                    # Find CTE body paren range using the CTE alias position
                    if bounds:
                        alias_node = cte_node.args.get("alias")
                        if alias_node and alias_node.this:
                            meta = alias_node.this.meta
                            if meta and "start" in meta:
                                open_off, close_off = bounds.find_cte_body_range(meta["start"])
                                cte_ranges[cte_name.lower()] = (open_off, close_off)

    # Second pass: collect relations in Go-compatible order.
    # Go processes: CTE bodies → ROOT FROM clause (subquery aliases first,
    # then recurse into subquery bodies) → CTE usage refs last.
    #
    # We achieve this in three sub-passes:
    # 1. CTE/UNION scopes (CTE body refs)
    # 2. ROOT scope: subquery usage refs first, then physical table refs
    # 3. DERIVED/SUBQUERY scopes (nested physical refs)
    # 4. Deferred CTE usage refs

    cte_usage_refs: list[tuple] = []

    def _owning_cte_range(scope) -> tuple[int, int]:
        """Get (open, close) paren offsets for the CTE that owns this scope."""
        cte_parent = scope.expression.find_ancestor(exp.CTE)
        if cte_parent:
            name = cte_parent.alias.lower() if cte_parent.alias else ""
            if name in cte_ranges:
                return cte_ranges[name]
        return -1, -1

    def _scope_range(scope) -> tuple[int, int]:
        """Where this scope runs. Anchored at the word it opens with, since a
        projection can be parenthesised and the range would clamp to it."""
        if not bounds:
            return -1, -1
        first_meta = _first_token_meta(scope.expression)
        if not first_meta:
            return -1, -1
        return bounds.range_from(bounds.find_select_before(first_meta["start"]))

    # Sub-pass 1: CTE/UNION scopes, process body refs, then add CTE vtab
    for scope in scopes:
        if scope.scope_type not in (ScopeType.CTE, ScopeType.UNION):
            continue
        nesting = _nesting_level(scope, bounds)
        cte_open, cte_close = _owning_cte_range(scope)
        # The body starts one character after the parenthesis that opens it.
        body_start, body_close = cte_open + 1, cte_close
        if cte_open < 0:
            # A union branch owned by no CTE: what holds it is the parenthesis
            # around it, not the whole statement, or its names leak out.
            body_start, body_close = _scope_range(scope)
        for alias, source in _ordered_sources(scope):
            if isinstance(source, exp.Table):
                _add_table_ref(
                    source, alias, nesting, default_schema, relations, seen_rels, stmt_idx,
                    scope_start_offset=body_start, scope_end_offset=body_close,
                )
            elif isinstance(source, Scope):
                source_name = (_cte_name_for(source) or alias).lower()
                if source_name in cte_defs:
                    continue
                _add_virtual_usage_ref(
                    alias, "", nesting, relations, seen_rels, stmt_idx,
                    scope_start_offset=body_start, scope_end_offset=body_close,
                )
                if source_name not in seen_vtabs:
                    vtab = _build_virtual_table(alias, source, schema_dict, default_schema, cte_defs, bounds)
                    vtab["scope_start_offset"] = body_start
                    vtab["scope_end_offset"] = body_close
                    seen_vtabs.add(source_name)
                    virtual_tables.append(vtab)

        # Add the CTE's own vtab: scope starts after closing paren, nesting=0
        if scope.scope_type == ScopeType.CTE:
            cte_parent = scope.expression.parent
            if isinstance(cte_parent, exp.CTE):
                cte_name = cte_parent.alias.lower() if cte_parent.alias else ""
                if cte_name and cte_name in cte_defs and cte_name not in seen_vtabs:
                    vtab = cte_defs[cte_name]
                    vtab["nesting_level"] = 0
                    if cte_close >= 0:
                        vtab["scope_start_offset"] = cte_close + 1
                        first_meta = _first_token_meta(scope.expression)
                        vtab["scope_end_offset"] = bounds.find_statement_end(first_meta["start"]) if bounds and first_meta else -1
                    seen_vtabs.add(cte_name)
                    virtual_tables.append(vtab)

    # Everything added from here on is root level, so this is where an UPDATE
    # or DELETE target goes: after the CTE bodies, before the FROM tables.
    root_refs_start = len(relations)

    # Sub-pass 2: ROOT scopes, subquery refs first, then tables
    for scope in scopes:
        if scope.scope_type != ScopeType.ROOT:
            continue
        nesting = _nesting_level(scope, bounds)
        root_start, root_end = _scope_range(scope)
        for alias, source in _ordered_sources(scope):
            if isinstance(source, Scope):
                source_name = (_cte_name_for(source) or alias).lower()
                if source_name in cte_defs:
                    cte_table = cte_defs[source_name]["table"]
                    effective_alias = alias if alias.lower() != cte_table.lower() else ""
                    cte_usage_refs.append((
                        cte_table, effective_alias, nesting, stmt_idx,
                        root_start, root_end,
                    ))
                else:
                    _add_virtual_usage_ref(
                        alias, "", nesting, relations, seen_rels, stmt_idx,
                        scope_start_offset=root_start, scope_end_offset=root_end,
                    )
                    if source_name not in seen_vtabs:
                        vtab = _build_virtual_table(alias, source, schema_dict, default_schema, cte_defs, bounds)
                        vtab["scope_start_offset"] = root_start
                        vtab["scope_end_offset"] = root_end
                        vtab["nesting_level"] = 0  # available at root level
                        seen_vtabs.add(source_name)
                        virtual_tables.append(vtab)
            elif isinstance(source, exp.Table):
                _add_table_ref(
                    source, alias, nesting, default_schema, relations, seen_rels, stmt_idx,
                    scope_start_offset=root_start, scope_end_offset=root_end,
                )

    # Sub-pass 3: DERIVED/SUBQUERY scopes (nested)
    for scope in reversed(scopes):
        if scope.scope_type in (ScopeType.CTE, ScopeType.UNION, ScopeType.ROOT):
            continue
        nesting = _nesting_level(scope, bounds)
        scope_start, scope_end = _scope_range(scope)

        # Nesting level where the vtab is available = one level up from where it's defined
        parent_nesting = max(0, nesting - 1)
        nested_sources = _ordered_sources(scope)
        for alias, source in nested_sources:
            if isinstance(source, Scope):
                source_name = (_cte_name_for(source) or alias).lower()
                if source_name in cte_defs:
                    # CTE referenced inside a subquery
                    cte_table = cte_defs[source_name]["table"]
                    effective_alias = alias if alias.lower() != cte_table.lower() else ""
                    _add_virtual_usage_ref(
                        cte_table, effective_alias, nesting, relations, seen_rels, stmt_idx,
                        scope_start_offset=scope_start, scope_end_offset=scope_end,
                    )
                else:
                    _add_virtual_usage_ref(
                        alias, "", nesting, relations, seen_rels, stmt_idx,
                        scope_start_offset=scope_start, scope_end_offset=scope_end,
                    )
                    if source_name not in seen_vtabs:
                        vtab = _build_virtual_table(alias, source, schema_dict, default_schema, cte_defs, bounds)
                        vtab["scope_start_offset"] = scope_start
                        vtab["scope_end_offset"] = scope_end
                        vtab["nesting_level"] = parent_nesting
                        seen_vtabs.add(source_name)
                        virtual_tables.append(vtab)
        # A relation a scope reads is in scope for the whole of it.
        for alias, source in nested_sources:
            if isinstance(source, exp.Table):
                _add_table_ref(
                    source, alias, nesting, default_schema, relations, seen_rels, stmt_idx,
                    scope_start_offset=scope_start, scope_end_offset=scope_end,
                )

    # Sub-pass 4: CTE usage refs (deferred to appear after nested refs)
    for table, alias, nesting, sidx, sstart, send in cte_usage_refs:
        _add_virtual_usage_ref(
            table, alias, nesting, relations, seen_rels, sidx,
            scope_start_offset=sstart, scope_end_offset=send,
        )

    return root_refs_start


def _from_clause_keys(expression) -> list[str]:
    """The names a statement's FROM and JOIN clauses reference, in source order.

    Subquery bodies are not descended into, so only top level references are
    returned. See _ordered_sources for when this is used instead of sqlglot.
    """
    keys: list[str] = []

    def add_entry(node) -> None:
        if node is None:
            return
        name = node.alias_or_name
        if name not in keys:
            keys.append(name)
        for join in node.args.get("joins") or []:
            add_entry(join.this)

    from_node = expression.args.get("from_") or expression.args.get("from")
    if from_node is not None:
        add_entry(from_node.this)
    elif isinstance(expression, (exp.Table, exp.Subquery)):
        # The scope expression is the FROM entry itself rather than a statement
        # that has one, which is the shape UPDATE and DELETE USING produce.
        add_entry(expression)
    for join in expression.args.get("joins") or []:
        add_entry(join.this)
    return keys


def _ordered_sources(scope) -> list[tuple]:
    """The sources a scope's FROM and JOIN clauses select from, in source order.

    scope.sources is not that set. It also carries every CTE visible to the
    statement, used or not, and an aliased reference appears twice, under the
    CTE name and under the alias.

    sqlglot answers this with selected_sources, which is defined as exactly
    this question, and gets cases a FROM walk misses, such as a parenthesized
    join. It reports nothing useful for DML, whose root scope expression is the
    FROM entry itself rather than a statement that has one, and it raises when
    a scope cannot be resolved, which ordinary half typed SQL produces here.
    """
    if isinstance(scope.expression, (exp.Table, exp.Subquery)):
        keys = _from_clause_keys(scope.expression)
    else:
        try:
            return [(name, source) for name, (_, source) in scope.selected_sources.items()]
        except Exception:
            keys = _from_clause_keys(scope.expression)
    return [(key, scope.sources[key]) for key in keys if key in scope.sources]


def _cte_name_for(source) -> str:
    """The CTE a source is, when it is one. An aliased reference arrives keyed
    by its alias, which does not say which CTE it names.
    """
    expression = getattr(source, "expression", None)
    owner = expression.parent if expression is not None else None
    return owner.alias if isinstance(owner, exp.CTE) else ""


def _dml_sources(source: exp.Expression):
    """The relations one item of a FROM or USING names, each on its own.

    Only what the clause names, never what a subquery reads: a table inside a
    derived table belongs to that query's scope, not to the statement holding
    it, and lifting it here reports it twice.
    """
    node = source.this if isinstance(source, exp.From) else source
    if isinstance(node, (exp.Table, exp.Subquery)):
        yield node
    for join in source.args.get("joins") or []:
        yield from _dml_sources(join.this)


def _add_derived_source(
    subquery: exp.Subquery,
    schema_dict: dict,
    default_schema: str,
    relations: list[dict],
    virtual_tables: list[dict],
    seen: set[tuple],
    seen_vtabs: set[str],
    stmt_idx: int,
) -> None:
    """Report a derived table a DML statement names, the way a query's FROM
    reports one: a relation to refer to and the columns behind it."""
    name = subquery.alias_or_name
    if not name or name in seen_vtabs:
        return
    seen_vtabs.add(name)
    virtual_tables.append(
        _build_virtual_table(name, subquery.this, schema_dict, default_schema, {})
    )
    _add_virtual_usage_ref(name, "", 0, relations, seen, stmt_idx)


def _collect_dml_target(
    stmt: exp.Expression,
    scopes: list,
    schema_dict: dict,
    default_schema: str,
    relations: list[dict],
    virtual_tables: list[dict],
    seen: set[tuple],
    seen_vtabs: set[str],
    stmt_idx: int = 0,
    bounds: _ScopeBounds | None = None,
    insert_index: int | None = None,
) -> None:
    """Extract tables from UPDATE/INSERT/DELETE when traverse_scope misses them.

    traverse_scope does not report the target of an UPDATE or DELETE, only the
    tables its FROM clause reads, so appending here would put the target after
    the tables it is being updated from. Callers pass insert_index to place it
    in source order instead, which matters because an unqualified column
    resolves to the first reference that has it.
    """
    if not isinstance(stmt, (exp.Update, exp.Insert, exp.Delete)):
        return

    # Collect CTE names so we can set schema="" for CTE references
    cte_names: set[str] = set()
    with_clause = stmt.find(exp.With)
    if with_clause:
        for cte in with_clause.expressions:
            cte_names.add(cte.alias.lower())

    # Collect all tables to add: target table + any FROM clause tables
    tables_to_add: list[exp.Table] = []

    table_node = stmt.this
    if isinstance(table_node, exp.Schema):
        table_node = table_node.this
    if isinstance(table_node, exp.Table):
        tables_to_add.append(table_node)

    # PostgreSQL UPDATE...FROM, DELETE...USING. A USING carries a list of
    # relations where a FROM carries one node, so both are read as a list.
    sources = stmt.args.get("from_") or stmt.args.get("using") or []
    if not isinstance(sources, list):
        sources = [sources]
    for source in sources:
        for named in _dml_sources(source):
            if isinstance(named, exp.Table):
                tables_to_add.append(named)
            else:
                _add_derived_source(
                    named, schema_dict, default_schema,
                    relations, virtual_tables, seen, seen_vtabs, stmt_idx,
                )

    # Build set of tables captured by ROOT scopes (not CTE-internal tables)
    scoped_tables: set[str] = set()
    for s in scopes:
        if s.scope_type not in (ScopeType.ROOT, ScopeType.UNION):
            continue
        for _, source in s.sources.items():
            if isinstance(source, exp.Table):
                scoped_tables.add(source.name.lower())

    insert_at = len(relations) if insert_index is None else insert_index
    for tbl in tables_to_add:
        table_name = tbl.name
        if not table_name or table_name.lower() in scoped_tables:
            continue

        is_cte = table_name.lower() in cte_names
        schema_name = "" if is_cte else (tbl.db or default_schema)
        alias_str = tbl.alias or ""
        if alias_str.lower() == table_name.lower():
            alias_str = ""

        key = (schema_name.lower(), table_name.lower(), alias_str.lower(), 0, stmt_idx)
        if key in seen:
            continue

        line, col, _, end_col = span(tbl)

        # DML target scope: from statement start to end
        dml_start, dml_end = -1, -1
        if bounds:
            dml_start = bounds.find_select_before(col)
            dml_end = bounds.find_statement_end(col)

        seen.add(key)
        relations.insert(insert_at, {
            "table":              table_name,
            "schema":             schema_name,
            "database":           tbl.catalog or "",
            "alias":              alias_str,
            "is_virtual":         False,
            "nesting_level":      0,
            "line":               line,
            "col":                col,
            "end_col":            end_col,
            "scope_start_offset": dml_start,
            "scope_end_offset":   dml_end,
        })
        insert_at += 1


def _first_token_meta(expr: exp.Expression) -> dict | None:
    """Find the first node with position metadata in the expression tree."""
    for node in expr.walk():
        if isinstance(node, exp.Expression) and node.meta.get("start") is not None:
            return node.meta
    return None


def _nesting_level(scope, bounds) -> int:
    """How deep in parentheses this scope sits, counted the way the caret's
    depth is. Sqlglot's scope tree is a different shape: a LATERAL is a level
    the text has no parenthesis for."""
    if scope.scope_type in (ScopeType.ROOT, ScopeType.UNION):
        return 0
    first_meta = _first_token_meta(scope.expression)
    if not first_meta:
        return 1
    return max(bounds.depth_at(first_meta["start"]), 1)


def _add_table_ref(
    source: exp.Table,
    alias: str,
    nesting: int,
    default_schema: str,
    relations: list[dict],
    seen: set[tuple],
    stmt_idx: int = 0,
    scope_start_offset: int = -1,
    scope_end_offset: int = -1,
) -> None:
    table_name = source.name
    schema_name = source.db or default_schema
    db_name = source.catalog or ""
    effective_alias = alias if alias.lower() != table_name.lower() else ""
    line, col, _, end_col = span(source)

    key = (schema_name.lower(), table_name.lower(), effective_alias.lower(), nesting, stmt_idx)
    if key in seen:
        return
    seen.add(key)

    relations.append({
        "table":              table_name,
        "schema":             schema_name,
        "database":           db_name,
        "alias":              effective_alias,
        "is_virtual":         False,
        "nesting_level":      nesting,
        "line":               line,
        "col":                col,
        "end_col":            end_col,
        "scope_start_offset": scope_start_offset,
        "scope_end_offset":   scope_end_offset,
    })


def _add_virtual_usage_ref(
    table: str,
    alias: str,
    nesting: int,
    relations: list[dict],
    seen: set[tuple],
    stmt_idx: int = 0,
    scope_start_offset: int = -1,
    scope_end_offset: int = -1,
) -> None:
    """Add a relation ref for CTE/subquery usage (schema is always empty)."""
    key = ("", table.lower(), alias.lower(), nesting, stmt_idx)
    if key in seen:
        return
    seen.add(key)

    relations.append({
        "table":              table,
        "schema":             "",
        "database":           "",
        "alias":              alias,
        "is_virtual":         False,
        "nesting_level":      nesting,
        "line":               1,
        "col":                0,
        "end_col":            0,
        "scope_start_offset": scope_start_offset,
        "scope_end_offset":   scope_end_offset,
    })


def _usable_nesting(scope, bounds) -> int:
    """The depth a virtual table can be named at: one out from its body."""
    if not bounds or not isinstance(scope, Scope):
        return 0
    return max(0, _nesting_level(scope, bounds) - 1)


def _explicit_columns(projected, wrapper) -> list[str]:
    """The names a relation's own column list gives it, as in "(SELECT 1) s(x)".

    The list sits on whichever wrapper carries the alias: a subquery's own, or
    the LATERAL holding it.
    """
    for node in (projected.parent, wrapper, wrapper.parent):
        if node is None:
            continue
        alias = node.args.get("alias")
        if isinstance(alias, exp.TableAlias) and alias.columns:
            return [c.name for c in alias.columns]
    return []


def _build_virtual_table(
    name: str,
    scope_or_scope_obj,
    schema_dict: dict,
    default_schema: str,
    cte_defs: dict[str, dict],
    bounds=None,
) -> dict:
    """Build a virtual table entry with inferred columns."""
    scope = scope_or_scope_obj
    if isinstance(scope, Scope):
        expr = scope.expression
    else:
        expr = scope
    # A LATERAL holds the query whose columns these are, in a subquery, and
    # neither wrapper projects anything itself. Half written SQL can leave a
    # wrapper holding nothing at all.
    projected = expr
    while isinstance(projected, (exp.Lateral, exp.Subquery)) and projected.this is not None:
        projected = projected.this

    columns = _infer_columns(projected, scope, schema_dict, default_schema, cte_defs)

    explicit_names = _explicit_columns(projected, expr)
    for i, ename in enumerate(explicit_names):
        if i < len(columns):
            columns[i] = {**columns[i], "name": ename}
        else:
            columns.append({"name": ename, "type": "unknown", "nullable": True})

    # Try to get position from CTE alias node
    line, col = 1, 0
    if isinstance(expr.parent, exp.CTE):
        alias_node = expr.parent.args.get("alias")
        if alias_node and alias_node.this:
            line, col = pos(alias_node)

    return {
        "table":              name,
        "schema":             "",
        "database":           "",
        "alias":              name,
        "is_virtual":         True,
        # A virtual table is named one level out from the body that defines
        # it, which is the level a caret can reach it at.
        "nesting_level":      _usable_nesting(scope, bounds),
        "line":               line,
        "col":                col,
        "end_col":            col + len(name),
        "columns":            columns,
        "scope_start_offset": -1,
        "scope_end_offset":   -1,
    }


def _infer_columns(
    expr: exp.Expression,
    scope,
    schema_dict: dict,
    default_schema: str,
    cte_defs: dict[str, dict],
) -> list[dict]:
    """Infer output columns from a SELECT expression."""
    if not hasattr(expr, "selects"):
        return []

    selects = expr.selects

    # SELECT * requires expanding from source tables
    if any(isinstance(s, exp.Star) for s in selects):
        return _expand_star(scope, schema_dict, default_schema, cte_defs)

    columns = []
    source_tables = _source_table_map(scope, default_schema) if isinstance(scope, Scope) else {}

    for sel in selects:
        col_info = _infer_single_column(sel, source_tables, schema_dict, default_schema, cte_defs)
        if col_info:
            columns.append(col_info)

    return columns


def _infer_single_column(
    sel: exp.Expression,
    source_tables: dict[str, tuple[str, str]],
    schema_dict: dict,
    default_schema: str,
    cte_defs: dict[str, dict],
) -> dict | None:
    """Infer name and type for a single SELECT item."""
    if isinstance(sel, exp.Alias):
        name = sel.alias
        inner = sel.this
        col_type = _resolve_expr_type(inner, source_tables, schema_dict, default_schema, cte_defs)
        return {"name": name, "type": col_type, "nullable": True}

    if isinstance(sel, exp.Column):
        name = sel.name
        qualifier = sel.table or ""
        col_type = _resolve_column_type(
            name, qualifier, source_tables, schema_dict, default_schema, cte_defs,
        )
        return {"name": name, "type": col_type, "nullable": True}

    # Expression without alias (e.g., length('x'))
    # Use the original SQL text via sql() but lowercase to match Go's token-based output
    return {"name": sel.sql(dialect="postgres").lower(), "type": "unknown", "nullable": True}


def _resolve_expr_type(
    expr: exp.Expression,
    source_tables: dict[str, tuple[str, str]],
    schema_dict: dict,
    default_schema: str,
    cte_defs: dict[str, dict],
) -> str:
    """Resolve the type of an expression (best effort)."""
    if isinstance(expr, exp.Column):
        return _resolve_column_type(
            expr.name, expr.table or "", source_tables, schema_dict, default_schema, cte_defs,
        )
    if isinstance(expr, exp.Literal):
        if expr.is_number:
            return "numeric"
        return "text"
    if isinstance(expr, exp.Boolean):
        return "boolean"
    if isinstance(expr, exp.Null):
        return "unknown"
    return "unknown"


def _resolve_column_type(
    col_name: str,
    qualifier: str,
    source_tables: dict[str, tuple[str, str]],
    schema_dict: dict,
    default_schema: str,
    cte_defs: dict[str, dict],
) -> str:
    """Look up column type from schema dict or CTE definitions."""
    if qualifier:
        targets = [qualifier.lower()]
    else:
        targets = list(source_tables.keys())

    for alias in targets:
        # Check CTE definitions first
        cte = cte_defs.get(alias)
        if cte:
            for c in cte.get("columns", []):
                if c["name"].lower() == col_name.lower():
                    return c["type"]
            continue

        # Check physical tables via schema_dict
        table_info = source_tables.get(alias)
        if not table_info:
            continue
        schema_name, table_name = table_info
        table_cols = schema_dict.get(schema_name, {}).get(table_name, {})
        # Case-insensitive lookup
        for k, v in table_cols.items():
            if k.lower() == col_name.lower():
                return v

    return "unknown"


def _source_table_map(scope, default_schema: str = "") -> dict[str, tuple[str, str]]:
    """Map alias -> (schema, table) for physical sources in the scope."""
    result = {}
    if not isinstance(scope, Scope):
        return result
    for alias, source in scope.sources.items():
        if isinstance(source, exp.Table):
            schema_name = source.db or default_schema
            table_name = source.name
            result[alias.lower()] = (schema_name, table_name)
    return result


def _expand_star(
    scope,
    schema_dict: dict,
    default_schema: str,
    cte_defs: dict[str, dict],
) -> list[dict]:
    """Expand SELECT * by listing columns from FROM-clause sources only."""
    columns = []
    if not isinstance(scope, Scope):
        return columns

    for alias, source in _ordered_sources(scope):
        if isinstance(source, exp.Table):
            schema_name = source.db or default_schema
            table_name = source.name
            table_cols = schema_dict.get(schema_name, {}).get(table_name, {})
            for col_name, col_type in table_cols.items():
                columns.append({"name": col_name, "type": col_type, "nullable": True})
        elif isinstance(source, Scope):
            cte = cte_defs.get((_cte_name_for(source) or alias).lower())
            if cte:
                columns.extend(cte.get("columns", []))
            else:
                columns.extend(_infer_columns(
                    source.expression, source, schema_dict, default_schema, cte_defs,
                ))

    return columns
