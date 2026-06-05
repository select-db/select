"""
Resolve column references and SELECT-list aliases using SQLGlot scope analysis.

Produces two outputs:
  - column_refs:    columns in SELECT/WHERE/GROUP BY/ORDER BY/HAVING with resolution status
  - column_aliases: SELECT-list aliases with inferred types for completion
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.optimizer.scope import Scope, ScopeType, traverse_scope

from analysis.schema import pos, span


def collect_resolved_column_refs(
    stmts: list,
    schema_dict: dict,
    default_schema: str,
) -> list[dict]:
    """Collect column references with resolution status from all scopes."""
    results: list[dict] = []

    for stmt in stmts:
        if stmt is None:
            continue
        try:
            scopes = list(traverse_scope(stmt))
        except Exception:
            continue

        cte_columns = _build_cte_column_map(scopes, schema_dict, default_schema)

        for scope in scopes:
            nesting = _nesting_depth(scope)
            visible = _visible_columns(scope, schema_dict, default_schema, cte_columns)

            for col in scope.columns:
                name = col.name
                if not name:
                    continue

                # Skip columns that are arguments to aggregate/window functions
                # at expression-definition sites (not reference sites)
                qualifier = col.table or ""
                qualified = qualifier != ""
                resolved = _is_resolved(name, qualifier, visible)

                line, start_col = pos(col)
                _, _, end_col = span(col)

                results.append({
                    "column":        name,
                    "qualified":     qualified,
                    "resolved":      resolved,
                    "nesting_level": nesting,
                    "line":          line,
                    "col":           start_col,
                    "len":           end_col - start_col,
                })

    return results


def collect_column_aliases(
    stmts: list,
    schema_dict: dict,
    default_schema: str,
) -> list[dict]:
    """Extract SELECT-list aliases with type info for completion."""
    results: list[dict] = []

    for stmt in stmts:
        if stmt is None:
            continue
        try:
            scopes = list(traverse_scope(stmt))
        except Exception:
            continue

        cte_columns = _build_cte_column_map(scopes, schema_dict, default_schema)

        for scope in scopes:
            if scope.scope_type not in (ScopeType.ROOT, ScopeType.UNION):
                continue

            source_map = _source_column_map(scope, schema_dict, default_schema, cte_columns)

            for sel in scope.expression.selects:
                alias_info = _extract_alias(sel, source_map)
                if alias_info:
                    results.append(alias_info)

    return results


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _nesting_depth(scope) -> int:
    if scope.scope_type in (ScopeType.ROOT, ScopeType.UNION):
        return 0
    depth = 0
    s = scope
    while s.parent:
        depth += 1
        s = s.parent
    return max(depth, 1)


def _build_cte_column_map(
    scopes: list,
    schema_dict: dict,
    default_schema: str,
) -> dict[str, list[tuple[str, str]]]:
    """Map CTE name -> [(col_name, col_type), ...] by inspecting CTE scope SELECT lists."""
    cte_cols: dict[str, list[tuple[str, str]]] = {}

    for scope in scopes:
        if scope.scope_type != ScopeType.CTE:
            continue
        cte_node = scope.expression.parent
        if not isinstance(cte_node, exp.CTE):
            continue
        cte_name = (cte_node.alias or "").lower()
        if not cte_name:
            continue

        cols: list[tuple[str, str]] = []
        source_tables = _physical_source_map(scope, default_schema)

        for sel in scope.expression.selects:
            if isinstance(sel, exp.Star):
                # Expand * from physical sources
                for _, (schema, table) in source_tables.items():
                    for col_name, col_type in schema_dict.get(schema, {}).get(table, {}).items():
                        cols.append((col_name, col_type))
                break
            col_name, col_type = _select_item_name_type(sel, source_tables, schema_dict, cte_cols)
            if col_name:
                cols.append((col_name, col_type))

        cte_cols[cte_name] = cols

    return cte_cols


def _visible_columns(
    scope,
    schema_dict: dict,
    default_schema: str,
    cte_columns: dict[str, list[tuple[str, str]]],
) -> dict[str, set[str]]:
    """Map qualifier (or "") -> set of visible column names in this scope."""
    visible: dict[str, set[str]] = {"": set()}

    for alias, source in scope.sources.items():
        alias_lower = alias.lower()

        if isinstance(source, exp.Table):
            schema_name = source.db or default_schema
            table_name = source.name
            table_cols = schema_dict.get(schema_name, {}).get(table_name, {})
            col_names = {c.lower() for c in table_cols}
            visible[alias_lower] = col_names
            visible[""].update(col_names)

        elif isinstance(source, Scope):
            cte_name = alias_lower
            if cte_name in cte_columns:
                col_names = {c[0].lower() for c in cte_columns[cte_name]}
            else:
                col_names = _infer_scope_columns(source, schema_dict, default_schema, cte_columns)
            visible[alias_lower] = col_names
            visible[""].update(col_names)

    return visible


def _is_resolved(name: str, qualifier: str, visible: dict[str, set[str]]) -> bool:
    key = qualifier.lower() if qualifier else ""
    col_set = visible.get(key)
    if col_set is None:
        return False
    return name.lower() in col_set


def _infer_scope_columns(
    scope,
    schema_dict: dict,
    default_schema: str,
    cte_columns: dict[str, list[tuple[str, str]]],
) -> set[str]:
    """Infer column names from a subquery/derived table scope."""
    if not isinstance(scope, Scope):
        return set()
    cols: set[str] = set()
    for sel in scope.expression.selects:
        if isinstance(sel, exp.Star):
            # Expand from sources
            for alias, source in scope.sources.items():
                if isinstance(source, exp.Table):
                    schema_name = source.db or default_schema
                    table_cols = schema_dict.get(schema_name, {}).get(source.name, {})
                    cols.update(c.lower() for c in table_cols)
                elif alias.lower() in cte_columns:
                    cols.update(c[0].lower() for c in cte_columns[alias.lower()])
            break
        if isinstance(sel, exp.Alias):
            cols.add(sel.alias.lower())
        elif isinstance(sel, exp.Column):
            cols.add(sel.name.lower())
    return cols


def _physical_source_map(scope, default_schema: str) -> dict[str, tuple[str, str]]:
    """Map alias -> (schema, table) for physical tables in the FROM clause only."""
    result = {}
    from_clause = scope.expression.find(exp.From)
    from_names: set[str] = set()
    if from_clause:
        for t in from_clause.find_all(exp.Table):
            from_names.add(t.name.lower())
            if t.alias:
                from_names.add(t.alias.lower())
    for join in scope.expression.find_all(exp.Join):
        for t in join.find_all(exp.Table):
            from_names.add(t.name.lower())
            if t.alias:
                from_names.add(t.alias.lower())

    for alias, source in scope.sources.items():
        if isinstance(source, exp.Table) and (not from_names or alias.lower() in from_names):
            result[alias.lower()] = (source.db or default_schema, source.name)
    return result


def _source_column_map(
    scope,
    schema_dict: dict,
    default_schema: str,
    cte_columns: dict[str, list[tuple[str, str]]],
) -> dict[str, tuple[str, str, str]]:
    """Map column_name -> (type, schema, table) for all visible columns in the scope."""
    result: dict[str, tuple[str, str, str]] = {}

    for alias, source in scope.sources.items():
        if isinstance(source, exp.Table):
            schema_name = source.db or default_schema
            table_name = source.name
            for col_name, col_type in schema_dict.get(schema_name, {}).get(table_name, {}).items():
                result[col_name.lower()] = (col_type, schema_name, table_name)
        elif isinstance(source, Scope):
            cte_name = alias.lower()
            if cte_name in cte_columns:
                for col_name, col_type in cte_columns[cte_name]:
                    result[col_name.lower()] = (col_type, "", alias)

    return result


def _select_item_name_type(
    sel: exp.Expression,
    source_tables: dict[str, tuple[str, str]],
    schema_dict: dict,
    cte_columns: dict[str, list[tuple[str, str]]],
) -> tuple[str, str]:
    """Extract (name, type) from a single SELECT item."""
    if isinstance(sel, exp.Alias):
        name = sel.alias
        inner = sel.this
        col_type = _resolve_expr_type(inner, source_tables, schema_dict, cte_columns)
        return name, col_type

    if isinstance(sel, exp.Column):
        name = sel.name
        qualifier = (sel.table or "").lower()
        col_type = _lookup_column_type(name, qualifier, source_tables, schema_dict, cte_columns)
        return name, col_type

    return "", "unknown"


def _extract_alias(
    sel: exp.Expression,
    source_map: dict[str, tuple[str, str, str]],
) -> dict | None:
    """Extract alias info from a SELECT item, returning None for non-aliased items."""
    if isinstance(sel, exp.Alias):
        alias_name = sel.alias
        inner = sel.this
        if isinstance(inner, exp.Column):
            col_name = inner.name
            info = source_map.get(col_name.lower())
            if info:
                col_type, schema, table = info
                return {
                    "alias": alias_name, "column": col_name,
                    "type": col_type, "schema": schema, "table": table,
                    "nullable": True,
                }
        return {
            "alias": alias_name, "column": alias_name,
            "type": "unknown", "schema": "", "table": "",
            "nullable": True,
        }
    return None


def _resolve_expr_type(
    expr: exp.Expression,
    source_tables: dict[str, tuple[str, str]],
    schema_dict: dict,
    cte_columns: dict[str, list[tuple[str, str]]],
) -> str:
    if isinstance(expr, exp.Column):
        return _lookup_column_type(expr.name, (expr.table or "").lower(), source_tables, schema_dict, cte_columns)
    if isinstance(expr, exp.Literal):
        return "numeric" if expr.is_number else "text"
    if isinstance(expr, exp.Boolean):
        return "boolean"
    return "unknown"


def _lookup_column_type(
    col_name: str,
    qualifier: str,
    source_tables: dict[str, tuple[str, str]],
    schema_dict: dict,
    cte_columns: dict[str, list[tuple[str, str]]],
) -> str:
    targets = [qualifier] if qualifier else list(source_tables.keys())
    col_lower = col_name.lower()

    for alias in targets:
        # CTE columns
        if alias in cte_columns:
            for cn, ct in cte_columns[alias]:
                if cn.lower() == col_lower:
                    return ct
            continue
        # Physical tables
        table_info = source_tables.get(alias)
        if not table_info:
            continue
        schema_name, table_name = table_info
        for k, v in schema_dict.get(schema_name, {}).get(table_name, {}).items():
            if k.lower() == col_lower:
                return v

    return "unknown"
