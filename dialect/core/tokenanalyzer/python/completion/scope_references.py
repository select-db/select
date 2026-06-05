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
from sqlglot.tokens import Tokenizer, TokenType

from analysis.schema import pos, span


class _ScopeBounds:
    """Pre-computed scope boundary character offsets from the SQLGlot tokenizer."""

    def __init__(self, sql: str) -> None:
        self.sql = sql
        tokens = list(Tokenizer().tokenize(sql))
        self._tokens = tokens

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

    def find_from_keyword(self, scope_start: int, scope_end: int) -> int:
        """Find the FROM keyword offset within the given scope range."""
        for t in self._tokens:
            if t.token_type == TokenType.FROM and t.start >= scope_start:
                if scope_end < 0 or t.start <= scope_end:
                    return t.start
        return -1

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
) -> dict:
    """Collect scope-aware relation references from parsed statements."""
    relations: list[dict] = []
    virtual_tables: list[dict] = []
    seen_rels: set[tuple] = set()
    seen_vtabs: set[str] = set()
    bounds = _ScopeBounds(sql)

    for stmt_idx, stmt in enumerate(stmts):
        if stmt is None:
            continue
        try:
            scopes = list(traverse_scope(stmt))
        except Exception:
            scopes = []

        if scopes:
            _collect_from_scopes(
                scopes, schema_dict, default_schema,
                relations, virtual_tables, seen_rels, seen_vtabs,
                stmt_idx, bounds,
            )

        _collect_dml_target(stmt, scopes, default_schema, relations, seen_rels, stmt_idx, bounds)

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
) -> None:
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

    def _root_scope_range(scope) -> tuple[int, int]:
        """Get (start, end) offsets for a ROOT-level scope."""
        if not bounds:
            return -1, -1
        first_meta = _first_token_meta(scope.expression)
        if not first_meta:
            return -1, -1
        start = bounds.find_select_before(first_meta["start"])
        end = bounds.find_statement_end(first_meta["start"])
        return start, end

    # Sub-pass 1: CTE/UNION scopes, process body refs, then add CTE vtab
    for scope in scopes:
        if scope.scope_type not in (ScopeType.CTE, ScopeType.UNION):
            continue
        nesting = _nesting_level(scope)
        cte_open, cte_close = _owning_cte_range(scope)
        # Scope starts one char AFTER the opening paren (inside the body)
        cte_body_start = cte_open + 1 if cte_open >= 0 else -1
        for alias, source in scope.sources.items():
            if isinstance(source, exp.Table):
                _add_table_ref(
                    source, alias, nesting, default_schema, relations, seen_rels, stmt_idx,
                    scope_start_offset=cte_body_start, scope_end_offset=cte_close,
                )
            elif isinstance(source, Scope):
                source_name = alias.lower()
                if source_name in cte_defs:
                    continue
                _add_virtual_usage_ref(
                    alias, "", nesting, relations, seen_rels, stmt_idx,
                    scope_start_offset=cte_body_start, scope_end_offset=cte_close,
                )
                if source_name not in seen_vtabs:
                    vtab = _build_virtual_table(alias, source, schema_dict, default_schema, cte_defs)
                    vtab["scope_start_offset"] = cte_body_start
                    vtab["scope_end_offset"] = cte_close
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

    # Sub-pass 2: ROOT scopes, subquery refs first, then tables
    for scope in scopes:
        if scope.scope_type != ScopeType.ROOT:
            continue
        nesting = _nesting_level(scope)
        root_start, root_end = _root_scope_range(scope)
        for alias, source in scope.sources.items():
            if isinstance(source, Scope):
                source_name = alias.lower()
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
                        vtab = _build_virtual_table(alias, source, schema_dict, default_schema, cte_defs)
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
        nesting = _nesting_level(scope)
        scope_start, scope_end = -1, -1
        # For physical table refs in FROM, use the FROM keyword position as scope
        # start so they aren't visible before FROM (e.g. at "SELECT |" before FROM).
        from_start = -1
        if bounds:
            first_meta = _first_token_meta(scope.expression)
            if first_meta:
                open_paren = bounds.sql.rfind("(", 0, first_meta["start"])
                if open_paren >= 0:
                    scope_start = open_paren + 1
                    scope_end = bounds.paren_pairs.get(open_paren, -1)
                else:
                    scope_start = bounds.find_select_before(first_meta["start"])
                    scope_end = bounds.find_statement_end(first_meta["start"])
                # Find FROM keyword position for table ref visibility.
                # Table refs should only be visible from the FROM keyword onward,
                # not at the SELECT position before FROM.
                if scope_start >= 0 and scope_end >= 0:
                    from_start = bounds.find_from_keyword(scope_start, scope_end)

        # Nesting level where the vtab is available = one level up from where it's defined
        parent_nesting = max(0, nesting - 1)
        for alias, source in scope.sources.items():
            if isinstance(source, Scope):
                source_name = alias.lower()
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
                        vtab = _build_virtual_table(alias, source, schema_dict, default_schema, cte_defs)
                        vtab["scope_start_offset"] = scope_start
                        vtab["scope_end_offset"] = scope_end
                        vtab["nesting_level"] = parent_nesting
                        seen_vtabs.add(source_name)
                        virtual_tables.append(vtab)
        # Physical table refs: scope starts at FROM (not at SELECT) so they
        # aren't visible before the FROM clause in the same scope
        table_scope_start = from_start if from_start >= 0 else scope_start
        for alias, source in scope.sources.items():
            if isinstance(source, exp.Table):
                _add_table_ref(
                    source, alias, nesting, default_schema, relations, seen_rels, stmt_idx,
                    scope_start_offset=table_scope_start, scope_end_offset=scope_end,
                )

    # Sub-pass 4: CTE usage refs (deferred to appear after nested refs)
    for table, alias, nesting, sidx, sstart, send in cte_usage_refs:
        _add_virtual_usage_ref(
            table, alias, nesting, relations, seen_rels, sidx,
            scope_start_offset=sstart, scope_end_offset=send,
        )


def _collect_dml_target(
    stmt: exp.Expression,
    scopes: list,
    default_schema: str,
    relations: list[dict],
    seen: set[tuple],
    stmt_idx: int = 0,
    bounds: _ScopeBounds | None = None,
) -> None:
    """Extract tables from UPDATE/INSERT/DELETE when traverse_scope misses them."""
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

    # PostgreSQL UPDATE...FROM, DELETE...USING
    from_clause = stmt.args.get("from_") or stmt.args.get("using")
    if from_clause:
        tables_to_add.extend(from_clause.find_all(exp.Table))

    # Build set of tables captured by ROOT scopes (not CTE-internal tables)
    scoped_tables: set[str] = set()
    for s in scopes:
        if s.scope_type not in (ScopeType.ROOT, ScopeType.UNION):
            continue
        for _, source in s.sources.items():
            if isinstance(source, exp.Table):
                scoped_tables.add(source.name.lower())

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

        line, col = pos(tbl)
        _, _, end_col = span(tbl)

        # DML target scope: from statement start to end
        dml_start, dml_end = -1, -1
        if bounds:
            dml_start = bounds.find_select_before(col)
            dml_end = bounds.find_statement_end(col)

        seen.add(key)
        relations.append({
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


def _first_token_meta(expr: exp.Expression) -> dict | None:
    """Find the first node with position metadata in the expression tree."""
    for node in expr.walk():
        if isinstance(node, exp.Expression) and node.meta.get("start") is not None:
            return node.meta
    return None


def _nesting_level(scope) -> int:
    """Compute nesting depth from the scope's parent chain."""
    if scope.scope_type in (ScopeType.ROOT, ScopeType.UNION):
        return 0
    depth = 0
    s = scope
    while s.parent:
        depth += 1
        s = s.parent
    return max(depth, 1)


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
    line, col = pos(source)
    _, _, end_col = span(source)

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


def _build_virtual_table(
    name: str,
    scope_or_scope_obj,
    schema_dict: dict,
    default_schema: str,
    cte_defs: dict[str, dict],
) -> dict:
    """Build a virtual table entry with inferred columns."""
    scope = scope_or_scope_obj
    if isinstance(scope, Scope):
        expr = scope.expression
    else:
        expr = scope

    columns = _infer_columns(expr, scope, schema_dict, default_schema, cte_defs)

    # Check for explicit column list: (SELECT ...) subq(col1, col2)
    parent = expr.parent
    if parent:
        table_alias = parent.args.get("alias")
        if isinstance(table_alias, exp.TableAlias) and table_alias.columns:
            explicit_names = [c.name for c in table_alias.columns]
            # Rename inferred columns with explicit names
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
        "nesting_level":      _nesting_level(scope) if isinstance(scope, Scope) else 0,
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

    # Only expand sources that are in the FROM clause, not just visible in scope
    from_names: set[str] = set()
    from_clause = scope.expression.find(exp.From)
    if from_clause:
        for t in from_clause.find_all(exp.Table):
            from_names.add(t.name.lower())
            if t.alias:
                from_names.add(t.alias.lower())
        # Subquery aliases in FROM
        for sq in from_clause.find_all(exp.Subquery):
            if sq.alias:
                from_names.add(sq.alias.lower())
    # Also include JOIN tables and subqueries
    for join in scope.expression.find_all(exp.Join):
        for t in join.find_all(exp.Table):
            from_names.add(t.name.lower())
            if t.alias:
                from_names.add(t.alias.lower())
        for sq in join.find_all(exp.Subquery):
            if sq.alias:
                from_names.add(sq.alias.lower())

    for alias, source in scope.sources.items():
        if alias.lower() not in from_names and from_names:
            continue

        if isinstance(source, exp.Table):
            schema_name = source.db or default_schema
            table_name = source.name
            table_cols = schema_dict.get(schema_name, {}).get(table_name, {})
            for col_name, col_type in table_cols.items():
                columns.append({"name": col_name, "type": col_type, "nullable": True})
        elif isinstance(source, Scope):
            cte = cte_defs.get(alias.lower())
            if cte:
                columns.extend(cte.get("columns", []))
            else:
                columns.extend(_infer_columns(
                    source.expression, source, schema_dict, default_schema, cte_defs,
                ))

    return columns
