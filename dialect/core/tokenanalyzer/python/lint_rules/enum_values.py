"""
Enum value lint rule:
  E001, string literal compared/assigned to an enum column is not one of
         the type's allowed values.

Enum values are resolved once on the Go side (core.EnrichEnumValues) and
projected into the request as `enum_dict` (schema -> table -> column ->
[values]), the same shape as schema_dict. This rule never re-derives them.

It only flags when the column resolves unambiguously to a single enum
column, the operand is a plain string literal, and the value matches no
allowed label even case-insensitively (so cross-dialect case differences
never produce a false positive, only clearly wrong values like 'activ').
"""
from __future__ import annotations

from sqlglot import exp

from analysis.schema import span

RULE_ID = "enum-value-mismatch"


def _table_enum_map(table_name: str, schema_name: str, enum_dict: dict):
    """Return {col_lower: (orig_col, allowed_list)} for a table, or None."""
    tables = enum_dict.get(schema_name.lower())
    if tables is None:
        return None
    raw = next((v for k, v in tables.items() if k.lower() == table_name.lower()), None)
    if not raw:
        return None
    return {c.lower(): (c, vals) for c, vals in raw.items()}


def _resolve(col_node, sources):
    """Resolve a Column to its (orig_col, allowed_list) or None.

    Unqualified refs only resolve when exactly one source defines that
    enum column with a single distinct value set (ambiguous -> skip).
    """
    name = col_node.name
    if not name or name == "*":
        return None
    qualifier = col_node.table.lower() if col_node.table else ""

    if qualifier:
        emap = sources.get(qualifier)
        return emap.get(name.lower()) if emap else None

    hits = [emap[name.lower()] for emap in sources.values() if emap and name.lower() in emap]
    if not hits:
        return None
    first = hits[0]
    if any(h[1] != first[1] for h in hits[1:]):
        return None
    return first


def _string_value(node):
    """Return the literal string value, or None if not a plain string literal."""
    if isinstance(node, exp.Literal) and node.is_string:
        return node.this
    return None


def analyze_enum_values(stmt, enum_dict: dict, default_schema: str) -> list[dict]:
    if not enum_dict or stmt is None:
        return []

    sources: dict[str, object] = {}
    for tbl in stmt.find_all(exp.Table):
        emap = _table_enum_map(tbl.name, (tbl.db or default_schema), enum_dict)
        if emap is None:
            continue
        sources[tbl.name.lower()] = emap
        if tbl.alias:
            sources[tbl.alias.lower()] = emap
    if not sources:
        return []

    results: list[dict] = []
    seen: set[tuple] = set()

    def _emit(value: str, orig_col: str, allowed: list[str], node) -> None:
        line, col, end_col = span(node)
        key = (line, col, value)
        if key in seen:
            return
        seen.add(key)
        shown = ", ".join(allowed[:12]) + (", …" if len(allowed) > 12 else "")
        results.append({
            "rule_id":    RULE_ID,
            "severity":   "warning",
            "message":    f"{value!r} is not a valid value for enum column {orig_col!r} (allowed: {shown})",
            "start_line": line, "start_col": col,
            "end_line":   line, "end_col":   end_col,
        })

    def _check(col_node, value_node) -> None:
        resolved = _resolve(col_node, sources) if isinstance(col_node, exp.Column) else None
        if resolved is None:
            return
        value = _string_value(value_node)
        if value is None:
            return
        orig_col, allowed = resolved
        lowered = {a.lower() for a in allowed}
        if value in allowed or value.lower() in lowered:
            return
        _emit(value, orig_col, allowed, value_node)

    # Comparisons: col = 'x', col <> 'x' (either operand order).
    for node in stmt.find_all(exp.EQ, exp.NEQ):
        _check(node.left, node.right)
        _check(node.right, node.left)

    # col IN ('a', 'b', …)
    for node in stmt.find_all(exp.In):
        target = node.this
        if isinstance(target, exp.Column):
            for val in node.args.get("expressions", []):
                _check(target, val)

    # UPDATE … SET col = 'x'
    if isinstance(stmt, exp.Update):
        for assignment in stmt.args.get("expressions", []):
            if isinstance(assignment, exp.EQ):
                _check(assignment.this, assignment.expression)

    # INSERT INTO t (cols…) VALUES (vals…)
    if isinstance(stmt, exp.Insert):
        raw_target = stmt.args.get("this")
        if isinstance(raw_target, exp.Schema):
            table = raw_target.this
            insert_cols = raw_target.args.get("expressions", [])
        else:
            table = raw_target
            insert_cols = []
        if isinstance(table, exp.Table) and insert_cols:
            emap = _table_enum_map(table.name, (table.db or default_schema), enum_dict)
            if emap:
                values = stmt.args.get("expression")
                tuples = values.args.get("expressions", []) if isinstance(values, exp.Values) else []
                for tup in tuples:
                    row = tup.args.get("expressions", []) if isinstance(tup, exp.Tuple) else []
                    for col_expr, val in zip(insert_cols, row):
                        cname = col_expr.name if isinstance(col_expr, (exp.Column, exp.Identifier)) else None
                        if not cname:
                            continue
                        entry = emap.get(cname.lower())
                        value = _string_value(val)
                        if entry and value is not None:
                            orig_col, allowed = entry
                            if value not in allowed and value.lower() not in {a.lower() for a in allowed}:
                                _emit(value, orig_col, allowed, val)

    return results
