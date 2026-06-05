"""
Reference resolution lint rules:
  R001, unknown table
  R002, unknown column
  R004, dead CTE
  R008, ambiguous column
"""
from __future__ import annotations

from sqlglot import exp

from analysis.schema import span


# ---------------------------------------------------------------------------
# R001, Unknown table
# ---------------------------------------------------------------------------

def analyze_unknown_tables(
    stmt: exp.Expression,
    schema_dict: dict,
    default_schema: str,
    virtual_names: set[str],
) -> list[dict]:
    """R001, tables referenced in the query that are not in the schema."""
    if not schema_dict:
        return []

    results = []
    loaded_schemas = {s.lower() for s in schema_dict}

    for table in stmt.find_all(exp.Table):
        name = table.name.lower()
        if not name:
            continue
        if name in virtual_names:
            continue
        if table.catalog:  # cross-database reference is unvalidatable
            continue

        schema_name = (table.db or default_schema).lower()
        if schema_name not in loaded_schemas:
            continue  # schema not loaded, can't validate

        known_tables = {t.lower() for t in schema_dict.get(schema_name, {})}
        if name not in known_tables:
            line, col, end_col = span(table)
            results.append({
                "rule_id":    "unknown-table",
                "severity":   "warning",
                "message":    f"unknown table or view {table.name!r}",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   end_col,
            })

    return results


# ---------------------------------------------------------------------------
# R002 / R008 helpers
# ---------------------------------------------------------------------------

def _scope_visible_columns(
    scope,
    schema_dict: dict,
    default_schema: str,
) -> dict[str, set[str]] | None:
    # visible maps alias → original-case column names from the schema
    visible: dict[str, set[str]] = {}

    for alias, source in scope.sources.items():
        if isinstance(source, exp.Table):
            table_name  = source.name.lower()
            schema_name = (source.db or default_schema).lower()
            cols_for_schema = schema_dict.get(schema_name, {})
            matched = next(
                (v for k, v in cols_for_schema.items() if k.lower() == table_name), None
            )
            if matched is not None:
                visible[alias.lower()] = set(matched)
        else:
            inner_cols = _scope_output_columns(source)
            if inner_cols is not None:
                visible[alias.lower()] = inner_cols

    return visible if visible else None


def _scope_output_columns(scope) -> set[str] | None:
    try:
        cols: set[str] = set()
        for sel in scope.expression.selects:
            alias = sel.alias
            if alias:
                cols.add(alias)  # preserve original case (matches schema_dict convention)
            elif isinstance(sel, exp.Column):
                cols.add(sel.name)  # preserve original case for quoted identifiers
            elif isinstance(sel, exp.Star):
                return None  # SELECT *, can't enumerate
        return cols if cols else None
    except Exception:
        return None


# ---------------------------------------------------------------------------
# R002, Unknown column
# ---------------------------------------------------------------------------

def _scope_select_aliases(scope) -> set[str]:
    """Return lowercase SELECT-list aliases defined in this scope.

    PostgreSQL (and MySQL) allow GROUP BY / ORDER BY to reference aliases
    defined in the SELECT list. These are not real table columns, so they
    would otherwise be flagged as unknown-column false positives.
    """
    aliases: set[str] = set()
    try:
        for sel in scope.expression.selects:
            alias = sel.alias
            if alias:
                aliases.add(alias.lower())
    except Exception:
        pass
    return aliases


def analyze_unknown_columns(
    scopes: list,
    schema_dict: dict,
    default_schema: str,
    sg_dialect: str,
) -> list[dict]:
    """R002, column references that cannot be resolved in any visible table."""
    if not schema_dict:
        return []

    results = []
    seen: set[tuple] = set()

    for scope in scopes:
        visible = _scope_visible_columns(scope, schema_dict, default_schema)
        if visible is None:
            continue

        select_aliases = _scope_select_aliases(scope)

        for col in scope.columns:
            quoted    = col.this.quoted if isinstance(col.this, exp.Expression) else False
            qualifier = col.table.lower() if col.table else ""

            if not col.name or col.name == "*":
                continue

            def _col_found(orig_cols: set[str]) -> bool:
                # Quoted: exact match (case-sensitive, as the DB treats it).
                # Unquoted: the DB folds to lowercase, so compare the folded
                # name against original-case schema columns, if none match
                # exactly, the column won't be found at runtime.
                if quoted:
                    return col.name in orig_cols
                return col.name.lower() in orig_cols

            if qualifier:
                orig_cols = visible.get(qualifier)
                if orig_cols is None:
                    # qualifier not in visible, two cases:
                    # (a) alias IS in scope.sources but table absent from schema_dict → skip
                    # (b) alias is NOT defined in FROM at all → fall through to emit diagnostic
                    if qualifier in {k.lower() for k in scope.sources}:
                        continue  # can't validate columns for unknown table
                    # else: undefined alias, fall through to emit diagnostic
                elif _col_found(orig_cols):
                    continue
            else:
                if any(_col_found(orig_cols) for orig_cols in visible.values()):
                    continue
                # Unqualified reference matches a SELECT alias, valid in
                # GROUP BY / ORDER BY (PostgreSQL, MySQL).
                if col.name.lower() in select_aliases:
                    continue

            line, col_pos, end_col = span(col)
            key = (col.name, qualifier, line, col_pos)
            if key not in seen:
                seen.add(key)
                display = f"{col.table}.{col.name}" if col.table else col.name
                results.append({
                    "rule_id":    "unknown-column",
                    "severity":   "warning",
                    "message":    f"unknown column {display!r}",
                    "start_line": line, "start_col": col_pos,
                    "end_line":   line, "end_col":   end_col,
                })

    return results


# ---------------------------------------------------------------------------
# R003, Unknown column in UPDATE SET / WHERE
# ---------------------------------------------------------------------------

def _resolve_table_cols(table_name: str, schema_name: str, schema_dict: dict) -> tuple[set[str], set[str]] | None:
    """Return (lowercase_cols, original_case_cols) for a table, or None if not in schema."""
    cols_for_schema = schema_dict.get(schema_name.lower())
    if cols_for_schema is None:
        return None
    raw = next((v for k, v in cols_for_schema.items() if k.lower() == table_name.lower()), None)
    if raw is None:
        return None
    return {c.lower() for c in raw}, set(raw)


def analyze_update_columns(
    stmt,
    schema_dict: dict,
    default_schema: str,
) -> list[dict]:
    """R003, column refs in UPDATE SET / WHERE that don't exist on the target (or FROM) table."""
    from sqlglot import exp as _exp

    if not schema_dict or not isinstance(stmt, _exp.Update):
        return []

    target = stmt.this
    if target is None:
        return []

    # Resolve target table columns, bail if unknown.
    target_schema = (target.db or default_schema).lower()
    target_cols = _resolve_table_cols(target.name, target_schema, schema_dict)
    if target_cols is None:
        return []

    # Build sources: qualifier → (lowercase_cols, orig_cols) | None.
    # None means "table in FROM but not in schema, skip validation for that qualifier".
    sources: dict[str, tuple[set[str], set[str]] | None] = {}
    for key in ([target.alias.lower()] if target.alias else []) + [target.name.lower()]:
        sources[key] = target_cols

    from_ = stmt.args.get("from_")
    if from_:
        for tbl in from_.find_all(_exp.Table):
            qualifier = (tbl.alias or tbl.name).lower()
            tbl_schema = (tbl.db or default_schema).lower()
            cols = _resolve_table_cols(tbl.name, tbl_schema, schema_dict)
            sources[qualifier] = cols

    results = []
    seen: set[tuple] = set()

    def _emit(col_node):
        name = col_node.name
        if not name or name == "*":
            return
        line, col_pos, end_col = span(col_node)
        key = (name, line, col_pos)
        if key not in seen:
            seen.add(key)
            results.append({
                "rule_id":    "unknown-column",
                "severity":   "warning",
                "message":    f"unknown column {name!r}",
                "start_line": line, "start_col": col_pos,
                "end_line":   line, "end_col":   end_col,
            })

    def _col_matches(cols_pair: tuple[set[str], set[str]], name: str, quoted: bool) -> bool:
        lower_cols, orig_cols = cols_pair
        return (name in orig_cols) if quoted else (name.lower() in lower_cols)

    def _check(col_node, allowed_sources: dict[str, tuple[set[str], set[str]] | None]):
        quoted    = col_node.this.quoted if isinstance(col_node.this, _exp.Expression) else False
        name      = col_node.name
        qualifier = col_node.table.lower() if col_node.table else ""
        if not name or name == "*":
            return

        if qualifier:
            if qualifier not in allowed_sources:
                _emit(col_node)
                return
            cols = allowed_sources[qualifier]
            if cols is None:
                return  # table not in schema, can't validate
            if not _col_matches(cols, name, quoted):
                _emit(col_node)
        else:
            loaded = [c for c in allowed_sources.values() if c is not None]
            if not loaded:
                return
            if not any(_col_matches(c, name, quoted) for c in loaded):
                _emit(col_node)

    def _cols_not_in_subquery(node):
        """Yield Column nodes that are NOT inside a nested Subquery."""
        for col in node.find_all(_exp.Column):
            if col.find_ancestor(_exp.Subquery) is None:
                yield col

    # SET LHS always belongs to the target table only.
    target_only = {k: v for k, v in sources.items() if v is target_cols}
    for assignment in stmt.expressions:
        if not isinstance(assignment, _exp.EQ):
            continue
        if isinstance(assignment.this, _exp.Column):
            _check(assignment.this, target_only)
        # RHS can read from any source (target or FROM tables).
        if isinstance(assignment.expression, _exp.Column):
            _check(assignment.expression, sources)

    # JOIN ON conditions, all sources visible.
    if from_:
        for col_node in from_.find_all(_exp.Column):
            _check(col_node, sources)

    # WHERE, skip columns inside subqueries (validated by their own scope).
    where = stmt.args.get("where")
    if where:
        for col_node in _cols_not_in_subquery(where):
            _check(col_node, sources)

    # RETURNING, target table columns only.
    returning = stmt.args.get("returning")
    if returning:
        for col_node in returning.find_all(_exp.Column):
            if col_node.name and col_node.name != "*":
                _check(col_node, target_only)

    return results


# ---------------------------------------------------------------------------
# R004, Unknown column in INSERT
# ---------------------------------------------------------------------------

def analyze_insert_columns(
    stmt,
    schema_dict: dict,
    default_schema: str,
) -> list[dict]:
    """R004 - column refs in INSERT column list, RETURNING, and ON CONFLICT that don't exist on the target table."""
    from sqlglot import exp as _exp

    if not schema_dict or not isinstance(stmt, _exp.Insert):
        return []

    # Target is a Schema node (table + col list) or a bare Table node.
    raw_target = stmt.args.get('this')
    if raw_target is None:
        return []

    if isinstance(raw_target, _exp.Schema):
        table = raw_target.this
        insert_cols = raw_target.args.get('expressions', [])
    else:
        table = raw_target
        insert_cols = []

    table_schema = (table.db or default_schema).lower()
    cols = _resolve_table_cols(table.name, table_schema, schema_dict)
    if cols is None:
        return []
    lower_cols, orig_cols = cols

    results = []
    seen: set[tuple] = set()

    def _emit(name: str, node):
        line, col_pos, end_col = span(node)
        key = (name, line, col_pos)
        if key not in seen:
            seen.add(key)
            results.append({
                "rule_id":    "unknown-column",
                "severity":   "warning",
                "message":    f"unknown column {name!r}",
                "start_line": line, "start_col": col_pos,
                "end_line":   line, "end_col":   end_col,
            })

    def _check_col_name(name: str, quoted: bool, node):
        if not name or name == "*":
            return
        if quoted:
            if name not in orig_cols:
                _emit(name, node)
        else:
            if name.lower() not in lower_cols:
                _emit(name, node)

    # INSERT column list, entries are Identifier nodes, not Column nodes
    for ident in insert_cols:
        quoted = ident.args.get('quoted', False)
        _check_col_name(ident.name, quoted, ident)

    # RETURNING
    returning = stmt.args.get('returning')
    if returning:
        for col in returning.find_all(_exp.Column):
            if col.name and col.name != '*':
                quoted = col.this.quoted if isinstance(col.this, _exp.Expression) else False
                _check_col_name(col.name, quoted, col)

    # ON CONFLICT
    conflict = stmt.args.get('conflict')
    if conflict:
        # conflict keys (the (col) in ON CONFLICT (col))
        for ident in (conflict.args.get('conflict_keys') or []):
            name = ident.name if hasattr(ident, 'name') else str(ident)
            quoted = ident.quoted if hasattr(ident, 'quoted') else False
            _check_col_name(name, quoted, ident)

        # DO UPDATE SET assignments, LHS belongs to target, EXCLUDED mirrors target
        for eq in (conflict.args.get('expressions') or []):
            if not isinstance(eq, _exp.EQ):
                continue
            if isinstance(eq.this, _exp.Column):
                c = eq.this
                quoted = c.this.quoted if isinstance(c.this, _exp.Expression) else False
                _check_col_name(c.name, quoted, c)
            if isinstance(eq.expression, _exp.Column):
                c = eq.expression
                quoted = c.this.quoted if isinstance(c.this, _exp.Expression) else False
                # EXCLUDED is a virtual qualifier mirroring the target table
                if not c.table or c.table.upper() == 'EXCLUDED':
                    _check_col_name(c.name, quoted, c)

        # DO UPDATE WHERE
        conflict_where = conflict.args.get('where')
        if conflict_where:
            for col in conflict_where.find_all(_exp.Column):
                if col.table.upper() == 'EXCLUDED' or col.table.lower() == table.name.lower():
                    quoted = col.this.quoted if isinstance(col.this, _exp.Expression) else False
                    _check_col_name(col.name, quoted, col)

    return results


# ---------------------------------------------------------------------------
# R005, Unknown column in DELETE WHERE / USING / RETURNING
# ---------------------------------------------------------------------------

def analyze_delete_columns(
    stmt,
    schema_dict: dict,
    default_schema: str,
) -> list[dict]:
    """R005, column refs in DELETE WHERE / USING / RETURNING that don't exist on a visible source."""
    from sqlglot import exp as _exp

    if not schema_dict or not isinstance(stmt, _exp.Delete):
        return []

    # Target is always a bare Table node (no Schema wrapper).
    table = stmt.args.get('this')
    if table is None or not isinstance(table, _exp.Table):
        return []

    target_schema = (table.db or default_schema).lower()
    target_cols = _resolve_table_cols(table.name, target_schema, schema_dict)
    if target_cols is None:
        return []

    # Build sources: qualifier → cols | None.
    sources: dict[str, tuple[set[str], set[str]] | None] = {}
    for key in ([table.alias.lower()] if table.alias else []) + [table.name.lower()]:
        sources[key] = target_cols

    # USING clause: list of Table nodes or a containing node (PostgreSQL extension).
    using_raw = stmt.args.get('using') or []
    using_items = using_raw if isinstance(using_raw, list) else [using_raw]
    for item in using_items:
        tbls = item.find_all(_exp.Table) if hasattr(item, 'find_all') else ([item] if isinstance(item, _exp.Table) else [])
        for tbl in tbls:
            qualifier = (tbl.alias or tbl.name).lower()
            tbl_schema = (tbl.db or default_schema).lower()
            sources[qualifier] = _resolve_table_cols(tbl.name, tbl_schema, schema_dict)

    results = []
    seen: set[tuple] = set()

    def _emit(col_node):
        name = col_node.name
        if not name or name == "*":
            return
        line, col_pos, end_col = span(col_node)
        key = (name, line, col_pos)
        if key not in seen:
            seen.add(key)
            results.append({
                "rule_id":    "unknown-column",
                "severity":   "warning",
                "message":    f"unknown column {name!r}",
                "start_line": line, "start_col": col_pos,
                "end_line":   line, "end_col":   end_col,
            })

    def _col_matches(cols_pair: tuple[set[str], set[str]], name: str, quoted: bool) -> bool:
        lower_cols, orig_cols = cols_pair
        return (name in orig_cols) if quoted else (name.lower() in lower_cols)

    def _check(col_node, allowed_sources: dict[str, tuple[set[str], set[str]] | None]):
        quoted    = col_node.this.quoted if isinstance(col_node.this, _exp.Expression) else False
        name      = col_node.name
        qualifier = col_node.table.lower() if col_node.table else ""
        if not name or name == "*":
            return

        if qualifier:
            if qualifier not in allowed_sources:
                _emit(col_node)
                return
            cols = allowed_sources[qualifier]
            if cols is None:
                return  # table not in schema, can't validate
            if not _col_matches(cols, name, quoted):
                _emit(col_node)
        else:
            loaded = [c for c in allowed_sources.values() if c is not None]
            if not loaded:
                return
            if not any(_col_matches(c, name, quoted) for c in loaded):
                _emit(col_node)

    def _cols_not_in_subquery(node):
        for col in node.find_all(_exp.Column):
            if col.find_ancestor(_exp.Subquery) is None:
                yield col

    # WHERE, skip columns inside subqueries (validated by their own scope).
    where = stmt.args.get("where")
    if where:
        for col_node in _cols_not_in_subquery(where):
            _check(col_node, sources)

    # RETURNING, only target table columns visible.
    target_only = {k: v for k, v in sources.items() if v is target_cols}
    returning = stmt.args.get("returning")
    if returning:
        for col_node in returning.find_all(_exp.Column):
            if col_node.name and col_node.name != "*":
                _check(col_node, target_only)

    return results


# ---------------------------------------------------------------------------
# R006, Unknown column in MERGE
# ---------------------------------------------------------------------------

def analyze_merge_columns(
    stmt,
    schema_dict: dict,
    default_schema: str,
) -> list[dict]:
    """R006, column refs in MERGE ON / WHEN conditions / SET / INSERT / RETURNING."""
    from sqlglot import exp as _exp

    if not schema_dict or not isinstance(stmt, _exp.Merge):
        return []

    target = stmt.args.get('this')
    if not isinstance(target, _exp.Table):
        return []

    target_schema_name = (target.db or default_schema).lower()
    target_cols = _resolve_table_cols(target.name, target_schema_name, schema_dict)
    if target_cols is None:
        return []

    target_qualifiers: set[str] = {target.name.lower()}
    if target.alias:
        target_qualifiers.add(target.alias.lower())

    # Source may be a Table (known schema) or Subquery/CTE alias (unknown projection).
    using = stmt.args.get('using')
    source_cols: tuple[set[str], set[str]] | None = None
    source_qualifiers: set[str] = set()

    if isinstance(using, _exp.Table):
        src_schema = (using.db or default_schema).lower()
        source_cols = _resolve_table_cols(using.name, src_schema, schema_dict)
        source_qualifiers.add(using.name.lower())
        if using.alias:
            source_qualifiers.add(using.alias.lower())
    elif using is not None:
        # Subquery or CTE ref, can't validate source-qualified refs
        if hasattr(using, 'alias') and using.alias:
            source_qualifiers.add(using.alias.lower())

    target_sources: dict[str, tuple | None] = {q: target_cols for q in target_qualifiers}
    source_sources: dict[str, tuple | None] = {q: source_cols for q in source_qualifiers}
    all_sources: dict[str, tuple | None] = {**target_sources, **source_sources}

    results = []
    seen: set[tuple] = set()

    def _emit(col_node):
        name = col_node.name
        if not name or name == "*":
            return
        line, col_pos, end_col = span(col_node)
        key = (name, line, col_pos)
        if key not in seen:
            seen.add(key)
            results.append({
                "rule_id":    "unknown-column",
                "severity":   "warning",
                "message":    f"unknown column {name!r}",
                "start_line": line, "start_col": col_pos,
                "end_line":   line, "end_col":   end_col,
            })

    def _col_matches(cols_pair: tuple[set[str], set[str]], name: str, quoted: bool) -> bool:
        lower_cols, orig_cols = cols_pair
        return (name in orig_cols) if quoted else (name.lower() in lower_cols)

    def _check(col_node, allowed_sources: dict[str, tuple | None]):
        quoted    = col_node.this.quoted if isinstance(col_node.this, _exp.Expression) else False
        name      = col_node.name
        qualifier = col_node.table.lower() if col_node.table else ""
        if not name or name == "*":
            return

        if qualifier:
            if qualifier not in allowed_sources:
                _emit(col_node)
                return
            cols = allowed_sources[qualifier]
            if cols is None:
                return  # unknown projection, skip
            if not _col_matches(cols, name, quoted):
                _emit(col_node)
        else:
            loaded = [c for c in allowed_sources.values() if c is not None]
            if not loaded:
                return
            if not any(_col_matches(c, name, quoted) for c in loaded):
                _emit(col_node)

    # ON condition, both target and source visible.
    on_cond = stmt.args.get('on')
    if on_cond:
        for col_node in on_cond.find_all(_exp.Column):
            _check(col_node, all_sources)

    # WHEN branches.
    whens = stmt.args.get('whens')
    if whens:
        for when in whens.expressions:
            matched   = when.args.get('matched', False)
            by_source = when.args.get('source', False)
            condition = when.args.get('condition')
            then      = when.args.get('then')

            # Visibility for the WHEN AND condition.
            if matched:
                cond_sources = all_sources       # both rows exist
            elif by_source:
                cond_sources = target_sources    # only target row exists
            else:
                cond_sources = source_sources    # only source row exists

            if condition:
                for col_node in condition.find_all(_exp.Column):
                    _check(col_node, cond_sources)

            if isinstance(then, _exp.Update):
                rhs_sources = all_sources if matched else target_sources
                for eq in (then.args.get('expressions') or []):
                    if not isinstance(eq, _exp.EQ):
                        continue
                    if isinstance(eq.this, _exp.Column):
                        _check(eq.this, target_sources)
                    if isinstance(eq.expression, _exp.Column):
                        _check(eq.expression, rhs_sources)

            elif isinstance(then, _exp.Insert):
                # Column list belongs to target; VALUES reference source.
                col_list = then.args.get('this')
                if col_list:
                    for col_node in col_list.find_all(_exp.Column):
                        _check(col_node, target_sources)
                values = then.args.get('expression')
                if values:
                    for col_node in values.find_all(_exp.Column):
                        _check(col_node, source_sources)

    # RETURNING, target only.
    returning = stmt.args.get('returning')
    if returning:
        for col_node in returning.find_all(_exp.Column):
            if col_node.name and col_node.name != "*":
                _check(col_node, target_sources)

    return results


# ---------------------------------------------------------------------------
# R008, Ambiguous column
# ---------------------------------------------------------------------------

def analyze_ambiguous_columns(
    scopes: list,
    schema_dict: dict,
    default_schema: str,
) -> list[dict]:
    """R008, unqualified column refs that exist in more than one visible source."""
    if not schema_dict:
        return []

    results = []
    seen: set[tuple] = set()

    for scope in scopes:
        visible = _scope_visible_columns(scope, schema_dict, default_schema)
        if visible is None or len(visible) < 2:
            continue

        for col in scope.columns:
            if col.table:
                continue

            quoted = col.this.quoted if isinstance(col.this, exp.Expression) else False
            if not col.name or col.name == "*":
                continue

            matching_sources = {
                alias for alias, orig_cols in visible.items()
                if (col.name in orig_cols if quoted else col.name.lower() in orig_cols)
            }
            if len(matching_sources) > 1:
                line, col_pos, end_col = span(col)
                key = (col.name, line, col_pos)
                if key not in seen:
                    seen.add(key)
                    results.append({
                        "rule_id":    "ambiguous-column",
                        "severity":   "warning",
                        "message":    f"column {col.name!r} is ambiguous; qualify with a table alias",
                        "start_line": line, "start_col": col_pos,
                        "end_line":   line, "end_col":   end_col,
                    })

    return results


# ---------------------------------------------------------------------------
# R004, Dead CTE
# ---------------------------------------------------------------------------

def analyze_dead_ctes(stmt: exp.Expression) -> list[dict]:
    """R004, CTEs defined but never referenced in the query body."""
    with_clause = stmt.find(exp.With)
    if not with_clause:
        return []

    cte_names = {cte.alias.lower(): cte for cte in with_clause.expressions}
    if not cte_names:
        return []

    used: set[str] = {table.name.lower() for table in stmt.find_all(exp.Table)}

    results = []
    for name, cte_node in cte_names.items():
        if name not in used:
            alias_ident = cte_node.args.get("alias")
            pos_node = alias_ident.this if isinstance(alias_ident, exp.Expression) else cte_node
            line, col, end_col = span(pos_node)
            results.append({
                "rule_id":    "unused-cte",
                "severity":   "hint",
                "message":    f"CTE {cte_node.alias!r} is defined but never used",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   end_col,
            })

    return results
