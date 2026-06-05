"""
Aggregation lint rules:
  A001, aggregate function used directly in WHERE
  A002, HAVING without GROUP BY
  A003, aggregate function in JOIN ON clause
"""
from __future__ import annotations

from sqlglot import exp

from analysis.schema import pos, span


def analyze_aggregation_rules(stmt: exp.Expression, user_agg_names: set[str]) -> list[dict]:
    results: list[dict] = []
    results.extend(_a001(stmt, user_agg_names))
    results.extend(_a002(stmt))
    results.extend(_a003(stmt, user_agg_names))
    return results


def _a001(stmt: exp.Expression, user_agg_names: set[str]) -> list[dict]:
    """A001, aggregate function used directly in a WHERE clause."""
    results = []
    for where in stmt.find_all(exp.Where):
        for node in where.find_all(exp.Anonymous, exp.AggFunc):
            if isinstance(node, exp.Anonymous):
                if node.name.lower() not in user_agg_names:
                    continue
            # Skip if inside a subquery nested within WHERE
            if _inside_subquery(node, where):
                continue
            line, col, end_col = span(node)
            name = node.name if hasattr(node, "name") and node.name else type(node).__name__
            results.append({
                "rule_id":    "agg-in-where",
                "severity":   "error",
                "message":    f"{name}() is not allowed in a WHERE clause; use HAVING instead",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   end_col,
            })
    return results


def _a002(stmt: exp.Expression) -> list[dict]:
    """A002, HAVING without GROUP BY at the same query level."""
    results = []
    for select in stmt.find_all(exp.Select):
        if select.args.get("having") and not select.args.get("group"):
            having = select.args["having"]
            line, col = pos(having)
            results.append({
                "rule_id":    "having-without-group-by",
                "severity":   "error",
                "message":    "HAVING without GROUP BY: use WHERE or add a GROUP BY clause",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
            })
    return results


def _a003(stmt: exp.Expression, user_agg_names: set[str]) -> list[dict]:
    """A003, aggregate function used in a JOIN ON clause."""
    results = []
    for join in stmt.find_all(exp.Join):
        on_clause = join.args.get("on")
        if not on_clause:
            continue
        for node in on_clause.find_all(exp.Anonymous, exp.AggFunc):
            if isinstance(node, exp.Anonymous):
                if node.name.lower() not in user_agg_names:
                    continue
            line, col, end_col = span(node)
            name = node.name if hasattr(node, "name") and node.name else type(node).__name__
            results.append({
                "rule_id":    "agg-in-join",
                "severity":   "error",
                "message":    f"{name}() is not allowed in a JOIN ON clause",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   end_col,
            })
    return results


def _inside_subquery(node: exp.Expression, boundary: exp.Expression) -> bool:
    """Return True if node is nested inside a Subquery within boundary."""
    parent = node.parent
    while parent is not None and parent is not boundary:
        if isinstance(parent, exp.Subquery):
            return True
        parent = parent.parent
    return False
