"""
Structural lint rules:
  G001 / G002, GROUP BY / ORDER BY ordinal out of range
  W001      , window function in WHERE
  S010      , duplicate SELECT output column names
"""
from __future__ import annotations

from sqlglot import exp

from analysis.schema import pos, span


def analyze_ordinal_violations(stmt: exp.Expression) -> list[dict]:
    """G001 / G002, GROUP BY or ORDER BY integer ordinal exceeds SELECT column count."""
    results = []

    for select in stmt.find_all(exp.Select):
        select_cols = select.args.get("expressions", [])
        col_count = len(select_cols)
        if col_count == 0:
            continue

        def _check_clause(clause_node, clause_name: str, rule_id: str) -> None:
            if clause_node is None:
                return
            expressions = (
                clause_node.expressions if hasattr(clause_node, "expressions") else []
            )
            for item in expressions:
                inner = item.this if isinstance(item, exp.Ordered) else item
                if isinstance(inner, exp.Literal) and inner.is_number:
                    try:
                        ordinal = int(inner.this)
                    except ValueError:
                        continue
                    if ordinal > col_count:
                        line, col, end_col = span(inner)
                        results.append({
                            "rule_id":    rule_id,
                            "severity":   "error",
                            "message":    f"{clause_name} ordinal {ordinal} exceeds the number of SELECT columns ({col_count})",
                            "start_line": line, "start_col": col,
                            "end_line":   line, "end_col":   end_col,
                        })

        _check_clause(select.args.get("group"), "GROUP BY", "ordinal-out-of-range")
        _check_clause(select.args.get("order"), "ORDER BY", "ordinal-out-of-range")

    return results


def analyze_window_in_where(stmt: exp.Expression) -> list[dict]:
    """W001, window function (OVER clause) used inside a WHERE predicate."""
    results = []
    seen: set[tuple] = set()

    for where in stmt.find_all(exp.Where):
        for window in where.find_all(exp.Window):
            line, col = pos(window)
            key = (line, col)
            if key not in seen:
                seen.add(key)
                results.append({
                    "rule_id":    "window-in-where",
                    "severity":   "error",
                    "message":    "window functions are not allowed in WHERE clauses",
                    "start_line": line, "start_col": col,
                    "end_line":   line, "end_col":   col,
                })

    return results


def analyze_duplicate_columns(stmt: exp.Expression) -> list[dict]:
    """S010, two or more SELECT output columns share the same name in the same SELECT list."""
    results = []

    for select in stmt.find_all(exp.Select):
        seen: dict[str, tuple[int, int]] = {}
        for expr in select.args.get("expressions", []):
            name = expr.alias or (expr.name if hasattr(expr, "name") else "")
            if not name:
                continue
            key = name.lower()
            line, col, end_col = span(expr)
            if key in seen:
                results.append({
                    "rule_id":    "duplicate-column",
                    "severity":   "warning",
                    "message":    f"duplicate output column name {name!r}",
                    "start_line": line, "start_col": col,
                    "end_line":   line, "end_col":   end_col,
                })
            else:
                seen[key] = (line, col)

    return results
