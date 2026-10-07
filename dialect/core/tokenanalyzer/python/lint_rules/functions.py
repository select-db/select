"""
Function catalog lint rule:
  R009, unknown function call
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.dialects.dialect import Dialect

from analysis.schema import span


def analyze_unknown_functions(
    stmt: exp.Expression,
    sg_dialect: str,
    user_functions: set[str],
) -> list[dict]:
    """
    R009, function calls not in the dialect's built-in registry and not in
    the user-provided catalog.

    sqlglot parses known functions as typed nodes (exp.Count, exp.Sum, …) and
    unknown/anonymous ones as exp.Anonymous. We flag Anonymous nodes whose name
    is not in the user-provided catalog.
    """
    results = []
    # An unquoted reserved word is never a user function: a call sqlglot left
    # untyped there is the dialect's own syntax, such as MySQL's ROW(...).
    reserved = Dialect.get_or_raise(sg_dialect).generator_class.RESERVED_KEYWORDS
    seen: set[str] = set()

    for func in stmt.find_all(exp.Anonymous):
        name = func.name.lower() if func.name else ""
        if not name or name in user_functions or name in seen:
            continue
        quoted = isinstance(func.this, exp.Identifier) and func.this.quoted
        if name in reserved and not quoted:
            continue
        seen.add(name)
        line, col, end_line, end_col = span(func)
        results.append({
            "rule_id":    "unknown-function",
            "severity":   "warning",
            "message":    f"unknown function {func.name!r} (not in loaded catalog or dialect builtins)",
            "start_line": line, "start_col": col,
            "end_line":   end_line, "end_col":   end_col,
        })

    return results
