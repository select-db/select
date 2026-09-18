"""
Rules that read the whole file rather than one statement.

A rule here scans the source text or its tokens, so it has no statement to be
given and costs the same whatever it is passed. Running one of these from the
per-statement rules makes a lint pass quadratic: the file is scanned once per
statement, and the copies are then thrown away by the diagnostic dedupe.

  F001, trailing comma (before a keyword or a closing paren)
"""
from __future__ import annotations

from sqlglot.dialects.dialect import Dialect as SqlglotDialect
from sqlglot.tokens import TokenType

_TRAILING_COMMA_NEXT = {
    TokenType.FROM, TokenType.WHERE, TokenType.GROUP_BY, TokenType.ORDER_BY,
    TokenType.HAVING, TokenType.LIMIT, TokenType.UNION, TokenType.INTERSECT,
    TokenType.EXCEPT, TokenType.R_PAREN, TokenType.SEMICOLON,
}


def analyze_file_rules(sql: str, sg_dialect: str = "") -> list[dict]:
    """Run every whole-file rule once over sql."""
    return _f001(sql, sg_dialect)


def _f001(sql: str, sg_dialect: str) -> list[dict]:
    """F001, trailing comma before a keyword or closing parenthesis."""
    if not sql:
        return []
    try:
        d = SqlglotDialect.get_or_raise(sg_dialect or "postgres")
        tokens = d.tokenize(sql)
    except Exception:
        return []

    results = []
    for i, tok in enumerate(tokens[:-1]):
        if tok.token_type is not TokenType.COMMA:
            continue
        nxt = tokens[i + 1]
        if nxt.token_type not in _TRAILING_COMMA_NEXT:
            continue
        col = tok.col - 1  # tok.col is exclusive-end, convert to 0-indexed start
        results.append({
            "rule_id":    "trailing-comma",
            "severity":   "error",
            "message":    f"Trailing comma before {nxt.text.upper() if nxt.text else 'keyword'}",
            "start_line": tok.line, "start_col": col,
            "end_line":   tok.line, "end_col":   col + 1,
        })
    return results
