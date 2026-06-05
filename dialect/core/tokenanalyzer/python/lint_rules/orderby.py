"""
ORDER BY / pagination lint rules:
  O001, OFFSET without LIMIT
  O002, LIMIT without ORDER BY
  O003, ORDER BY in subquery without LIMIT
"""
from __future__ import annotations

from sqlglot import exp

from analysis.schema import pos


def analyze_orderby_rules(stmt: exp.Expression) -> list[dict]:
    results: list[dict] = []
    results.extend(_o001_o002(stmt))
    results.extend(_o003(stmt))
    return results


def _o001_o002(stmt: exp.Expression) -> list[dict]:
    """O001 / O002, OFFSET without LIMIT or LIMIT without ORDER BY."""
    results = []
    for select in stmt.find_all(exp.Select):
        has_limit  = select.args.get("limit") is not None
        has_offset = select.args.get("offset") is not None
        has_order  = select.args.get("order") is not None

        if has_offset and not has_limit:
            offset_node = select.args["offset"]
            line, col = pos(offset_node)
            results.append({
                "rule_id":    "offset-without-limit",
                "severity":   "error",
                "message":    "OFFSET without LIMIT produces undefined behavior in most dialects",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
            })

        if has_limit and not has_order:
            limit_node = select.args["limit"]
            line, col = pos(limit_node)
            results.append({
                "rule_id":    "limit-without-order-by",
                "severity":   "hint",
                "message":    "LIMIT without ORDER BY returns a non-deterministic set of rows",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
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
        line, col = pos(order_node)
        results.append({
            "rule_id":    "subquery-order-by",
            "severity":   "hint",
            "message":    "ORDER BY inside a subquery without LIMIT is ignored in most dialects",
            "start_line": line, "start_col": col,
            "end_line":   line, "end_col":   col,
        })
    return results
