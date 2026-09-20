"""
ORDER BY / pagination lint rules:
  O001, OFFSET without LIMIT
  O002, LIMIT without ORDER BY
  O003, ORDER BY in subquery without LIMIT
"""
from __future__ import annotations

from sqlglot import exp

from analysis.schema import offset_requires_limit, span


def analyze_orderby_rules(stmt: exp.Expression, sg_dialect: str = "") -> list[dict]:
    results: list[dict] = []
    results.extend(_o001_o002(stmt, sg_dialect))
    results.extend(_o003(stmt))
    return results


def _o001_o002(stmt: exp.Expression, sg_dialect: str) -> list[dict]:
    """O001 / O002, OFFSET without LIMIT or pagination without ORDER BY."""
    results = []
    for select in stmt.find_all(exp.Select):
        has_limit  = select.args.get("limit") is not None
        has_offset = select.args.get("offset") is not None
        has_order  = select.args.get("order") is not None

        if has_offset and not has_limit and offset_requires_limit(sg_dialect):
            line, col, end_line, end_col = span(select.args["offset"])
            results.append({
                "rule_id":    "offset-without-limit",
                "severity":   "error",
                "message":    "OFFSET is only valid as part of a LIMIT clause in this dialect",
                "start_line": line, "start_col": col,
                "end_line":   end_line, "end_col":   end_col,
            })

        # Which rows are skipped or kept is only defined once the order is,
        # so OFFSET raises this as much as LIMIT does.
        if (has_limit or has_offset) and not has_order:
            clause = "limit" if has_limit else "offset"
            line, col, end_line, end_col = span(select.args[clause])
            results.append({
                "rule_id":    "limit-without-order-by",
                "severity":   "hint",
                "message":    f"{clause.upper()} without ORDER BY returns a non-deterministic set of rows",
                "start_line": line, "start_col": col,
                "end_line":   end_line, "end_col":   end_col,
            })

    return results


def _o003(stmt: exp.Expression) -> list[dict]:
    """O003, ORDER BY inside a subquery without LIMIT."""
    results = []
    for subquery in stmt.find_all(exp.Subquery):
        inner = subquery.this
        if not isinstance(inner, exp.Select):
            continue
        if not inner.args.get("order"):
            continue
        if inner.args.get("limit"):
            continue
        order_node = inner.args["order"]
        line, col, end_line, end_col = span(order_node)
        results.append({
            "rule_id":    "subquery-order-by",
            "severity":   "hint",
            "message":    "ORDER BY inside a subquery without LIMIT is ignored in most dialects",
            "start_line": line, "start_col": col,
            "end_line":   end_line, "end_col":   end_col,
        })
    return results
