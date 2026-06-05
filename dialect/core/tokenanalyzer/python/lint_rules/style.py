"""
Style / anti-pattern lint rules:
  S001, LIKE/ILIKE with no wildcard
  S002, LIKE/ILIKE with leading wildcard (non-sargable)
  S003, tautological predicate (1=1, true=true)
  S004, empty IN / NOT IN list
  S005, BETWEEN with reversed bounds
  S006, duplicate literals in IN list
  S007, NULL in plain IN list
  S008, literal division by zero
  S009, contradictory numeric predicates (col > 5 AND col < 3)
  S010, trailing comma (before keyword or closing paren)
  S011, unquoted mixed-case identifier
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.dialects.dialect import Dialect as SqlglotDialect
from sqlglot.tokens import TokenType

from analysis.schema import pos, span

_TRAILING_COMMA_NEXT = {
    TokenType.FROM, TokenType.WHERE, TokenType.GROUP_BY, TokenType.ORDER_BY,
    TokenType.HAVING, TokenType.LIMIT, TokenType.UNION, TokenType.INTERSECT,
    TokenType.EXCEPT, TokenType.R_PAREN, TokenType.SEMICOLON,
}


def analyze_style_rules(stmt: exp.Expression, sg_dialect: str = "", sql: str = "") -> list[dict]:
    results: list[dict] = []
    results.extend(_s010(sql, sg_dialect))
    results.extend(_s001_s002(stmt))
    results.extend(_s003(stmt))
    results.extend(_s004(stmt))
    results.extend(_s005(stmt))
    results.extend(_s006_s007(stmt))
    results.extend(_s008(stmt))
    results.extend(_s009(stmt))
    results.extend(_s011(stmt, sg_dialect))
    return results


def _s010(sql: str, sg_dialect: str) -> list[dict]:
    """S010, trailing comma before a keyword or closing parenthesis."""
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


# Dialects that do not fold unquoted identifiers to lowercase.
_NO_FOLD_DIALECTS = {"mysql"}


def _s011(stmt: exp.Expression, sg_dialect: str) -> list[dict]:
    """S011, unquoted identifier with mixed/upper case will be silently lowercased."""
    if sg_dialect in _NO_FOLD_DIALECTS:
        return []

    results = []
    seen: set[tuple] = set()

    for ident in stmt.find_all(exp.Identifier):
        if ident.quoted:
            continue
        name = ident.name
        if not name or name == name.lower():
            continue
        # Skip short single-word aliases like table alias 't', 'u', already lowercase
        line, col, end_col = span(ident)
        key = (line, col)
        if key in seen:
            continue
        seen.add(key)
        results.append({
            "rule_id":    "unquoted-uppercase",
            "severity":   "warning",
            "message":    f"unquoted identifier {name!r} contains uppercase letters; the database will lowercase it. Use \"{name}\" to preserve case",
            "start_line": line, "start_col": col,
            "end_line":   line, "end_col":   end_col,
        })

    return results


def _s001_s002(stmt: exp.Expression) -> list[dict]:
    results = []
    for node in stmt.find_all((exp.Like, exp.ILike)):
        pattern = node.expression
        if not isinstance(pattern, exp.Literal) or not pattern.is_string:
            continue
        val = pattern.this
        has_wildcard = "%" in val or "_" in val
        leading_wildcard = val and val[0] in ("%", "_")
        line, col, end_col = span(pattern)
        if not has_wildcard:
            results.append({
                "rule_id":    "like-no-wildcard",
                "severity":   "warning",
                "message":    "LIKE pattern has no wildcard; use = for a plain equality check",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   end_col,
            })
        elif leading_wildcard:
            results.append({
                "rule_id":    "like-leading-wildcard",
                "severity":   "hint",
                "message":    "LIKE pattern starts with a wildcard, preventing index use",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   end_col,
            })
    return results


def _s003(stmt: exp.Expression) -> list[dict]:
    """S003, tautological predicate like 1=1 or true=true."""
    results = []
    for node in stmt.find_all(exp.EQ):
        left, right = node.left, node.right
        if not isinstance(left, exp.Literal) or not isinstance(right, exp.Literal):
            if not (isinstance(left, exp.Boolean) and isinstance(right, exp.Boolean)):
                continue
        if isinstance(left, exp.Boolean) and isinstance(right, exp.Boolean):
            if left.this != right.this:
                continue
        elif isinstance(left, exp.Literal) and isinstance(right, exp.Literal):
            if left.this != right.this or not left.is_number:
                continue
        else:
            continue
        line, col = pos(left)
        results.append({
            "rule_id":    "tautological-predicate",
            "severity":   "warning",
            "message":    f"Tautological predicate: {left.sql()} = {right.sql()} is always true",
            "start_line": line, "start_col": col,
            "end_line":   line, "end_col":   col,
        })
    return results


def _s004(stmt: exp.Expression) -> list[dict]:
    """S004, empty IN () or NOT IN () list."""
    results = []
    for node in stmt.find_all(exp.In):
        if node.args.get("query"):
            continue
        if not node.expressions:
            line, col = pos(node)
            results.append({
                "rule_id":    "empty-in-list",
                "severity":   "error",
                "message":    "Empty IN list never matches any row",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
            })
    return results


def _s005(stmt: exp.Expression) -> list[dict]:
    """S005, BETWEEN with numeric lower bound > upper bound."""
    results = []
    for node in stmt.find_all(exp.Between):
        lo = node.args.get("low")
        hi = node.args.get("high")
        if not (isinstance(lo, exp.Literal) and lo.is_number):
            continue
        if not (isinstance(hi, exp.Literal) and hi.is_number):
            continue
        try:
            lo_val, hi_val = float(lo.this), float(hi.this)
        except ValueError:
            continue
        if lo_val > hi_val:
            line, col = pos(lo)
            results.append({
                "rule_id":    "reversed-between",
                "severity":   "error",
                "message":    f"BETWEEN bounds are reversed: {lo.this} > {hi.this}; this condition can never match",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
            })
    return results


def _s006_s007(stmt: exp.Expression) -> list[dict]:
    """S006, duplicate literals in IN list. S007, NULL in plain IN list."""
    results = []
    for node in stmt.find_all(exp.In):
        if node.args.get("query"):
            continue
        # Check if this is inside a NOT, if so, N003 handles the NULL case
        is_not_in = isinstance(node.parent, exp.Not)
        seen: dict[str, tuple[int, int]] = {}
        for item in node.expressions:
            if isinstance(item, exp.Null):
                if not is_not_in:
                    line, col = pos(item)
                    results.append({
                        "rule_id":    "null-in-list",
                        "severity":   "warning",
                        "message":    "NULL in IN list never matches via IN; use OR col IS NULL explicitly",
                        "start_line": line, "start_col": col,
                        "end_line":   line, "end_col":   col,
                    })
                continue
            if not isinstance(item, exp.Literal):
                continue
            key = item.this.upper()
            line, col = pos(item)
            if key in seen:
                results.append({
                    "rule_id":    "duplicate-in-value",
                    "severity":   "warning",
                    "message":    f"Duplicate value {item.sql()} in IN list",
                    "start_line": line, "start_col": col,
                    "end_line":   line, "end_col":   col,
                })
            else:
                seen[key] = (line, col)
    return results


def _s008(stmt: exp.Expression) -> list[dict]:
    """S008, literal division by zero."""
    results = []
    for node in stmt.find_all(exp.Div):
        divisor = node.right
        if isinstance(divisor, exp.Literal) and divisor.is_number:
            try:
                if float(divisor.this) == 0:
                    line, col = pos(node)
                    results.append({
                        "rule_id":    "division-by-zero",
                        "severity":   "error",
                        "message":    "Division by zero literal",
                        "start_line": line, "start_col": col,
                        "end_line":   line, "end_col":   col,
                    })
            except ValueError:
                pass
    return results


_COMP_OPS = (exp.GT, exp.GTE, exp.LT, exp.LTE)


def _s009(stmt: exp.Expression) -> list[dict]:
    """S009, contradictory predicates: col > 5 AND col < 3."""
    results = []
    for node in stmt.find_all(exp.And):
        left, right = node.left, node.right
        if not (isinstance(left, _COMP_OPS) and isinstance(right, _COMP_OPS)):
            continue
        # Both sides must be: column op literal
        lc, lv = _col_op_lit(left)
        rc, rv = _col_op_lit(right)
        if lc is None or rc is None or lc != rc:
            continue
        if _contradictory(type(left), lv, type(right), rv):
            line, col = pos(left)
            results.append({
                "rule_id":    "contradictory-predicate",
                "severity":   "error",
                "message":    f"Contradictory predicate: {left.sql()} AND {right.sql()} can never both be true",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
            })
    return results


def _col_op_lit(node: exp.Expression) -> tuple[str | None, float | None]:
    """Return (column_name, literal_value) if node is col op lit, else (None, None)."""
    col_node = node.left if isinstance(node.left, exp.Column) else None
    lit_node = node.right if isinstance(node.right, exp.Literal) else None
    if col_node is None or lit_node is None or not lit_node.is_number:
        return None, None
    try:
        return col_node.name.lower(), float(lit_node.this)
    except ValueError:
        return None, None


def _contradictory(op1, v1: float, op2, v2: float) -> bool:
    """Return True when col op1 v1 AND col op2 v2 is unsatisfiable."""
    def _check(lo_op, lo: float, hi_op, hi: float) -> bool:
        if lo_op not in (exp.GT, exp.GTE):
            return False
        if hi_op not in (exp.LT, exp.LTE):
            return False
        strict = lo_op is exp.GT or hi_op is exp.LT
        return lo >= hi if strict else lo > hi

    return _check(op1, v1, op2, v2) or _check(op2, v2, op1, v1)
