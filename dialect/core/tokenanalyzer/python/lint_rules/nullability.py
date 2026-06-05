"""
Null-safety lint rules:
  N001, = NULL (use IS NULL)
  N002, <> / != NULL (use IS NOT NULL)
  N003, NULL inside NOT IN list
  N004, COALESCE with only one argument
"""
from __future__ import annotations

from sqlglot import exp

from analysis.schema import pos


def analyze_null_rules(stmt: exp.Expression) -> list[dict]:
    results: list[dict] = []
    results.extend(_n001(stmt))
    results.extend(_n002(stmt))
    results.extend(_n003(stmt))
    results.extend(_n004(stmt))
    return results


def _n001(stmt: exp.Expression) -> list[dict]:
    """N001, expr = NULL should be IS NULL."""
    results = []
    for node in stmt.find_all(exp.EQ):
        for side in (node.left, node.right):
            if isinstance(side, exp.Null):
                line, col = pos(node.left)
                results.append({
                    "rule_id":    "null-equality",
                    "severity":   "error",
                    "message":    "Use IS NULL instead of = NULL",
                    "start_line": line, "start_col": col,
                    "end_line":   line, "end_col":   col,
                })
                break
    return results


def _n002(stmt: exp.Expression) -> list[dict]:
    """N002, expr <> NULL or != NULL should be IS NOT NULL."""
    results = []
    for node in stmt.find_all((exp.NEQ,)):
        for side in (node.left, node.right):
            if isinstance(side, exp.Null):
                line, col = pos(node.left)
                results.append({
                    "rule_id":    "null-inequality",
                    "severity":   "error",
                    "message":    "Use IS NOT NULL instead of <> NULL / != NULL",
                    "start_line": line, "start_col": col,
                    "end_line":   line, "end_col":   col,
                })
                break
    return results


def _n003(stmt: exp.Expression) -> list[dict]:
    """N003, NULL inside a NOT IN list never matches any row."""
    results = []
    for node in stmt.find_all(exp.Not):
        inner = node.this
        if not isinstance(inner, exp.In):
            continue
        if inner.args.get("query"):
            continue  # subquery, not a literal list
        for item in inner.expressions:
            if isinstance(item, exp.Null):
                line, col = pos(item)
                results.append({
                    "rule_id":    "null-in-not-in",
                    "severity":   "warning",
                    "message":    "NULL in NOT IN list causes the predicate to never match any row",
                    "start_line": line, "start_col": col,
                    "end_line":   line, "end_col":   col,
                })
    return results


def _n004(stmt: exp.Expression) -> list[dict]:
    """N004, COALESCE with a single argument always returns that argument."""
    results = []
    for node in stmt.find_all(exp.Coalesce):
        # sqlglot: Coalesce(this=first_arg, expressions=[rest...])
        extra = node.args.get("expressions", [])
        if len(extra) == 0:
            line, col = pos(node)
            results.append({
                "rule_id":    "coalesce-single-arg",
                "severity":   "warning",
                "message":    "COALESCE with only one argument always returns that argument; use the expression directly",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
            })
    return results
