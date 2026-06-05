"""
Type families and compatibility helpers used by type_checks.py.
"""
from __future__ import annotations

from sqlglot import exp

_DT = exp.DataType.Type
_UNKNOWN = _DT.UNKNOWN

# Type families, comparisons within a family are safe, across families are mismatches.
_NUMERIC = frozenset({
    _DT.INT, _DT.BIGINT, _DT.SMALLINT, _DT.TINYINT, _DT.MEDIUMINT,
    _DT.FLOAT, _DT.DOUBLE, _DT.DECIMAL, _DT.DECFLOAT,
    _DT.BIGDECIMAL, _DT.UINT, _DT.UBIGINT, _DT.USMALLINT, _DT.UTINYINT,
    _DT.INT128, _DT.INT256, _DT.UINT128, _DT.UINT256,
})
_TEXT = frozenset({
    _DT.VARCHAR, _DT.TEXT, _DT.CHAR, _DT.NVARCHAR, _DT.NCHAR,
    _DT.BPCHAR, _DT.LONGTEXT, _DT.MEDIUMTEXT, _DT.TINYTEXT,
})
_BOOLEAN = frozenset({_DT.BOOLEAN})
_TEMPORAL = frozenset({
    _DT.DATE, _DT.TIME, _DT.TIMETZ, _DT.DATETIME,
    _DT.TIMESTAMP, _DT.TIMESTAMPTZ, _DT.TIMESTAMPNTZ, _DT.TIMESTAMPLTZ,
    _DT.TIMESTAMP_S, _DT.TIMESTAMP_MS, _DT.TIMESTAMP_NS,
})
_INTERVAL = frozenset({_DT.INTERVAL})

# Which families can be used in arithmetic (+, -, *, /)
_ARITHMETIC_SAFE = _NUMERIC | _TEMPORAL | _INTERVAL

# T005, Maps sqlglot Func subclass → expected type family for its primary argument.
# Only covers functions where the constraint is unambiguous and the arg type
# is reliably resolved by annotate_types.
_FUNC_ARG_CONSTRAINTS: dict[type, str] = {
    # Text functions, primary arg must be text
    exp.Length:        "text",
    exp.Upper:         "text",
    exp.Lower:         "text",
    exp.Trim:          "text",
    exp.Substring:     "text",
    exp.Levenshtein:   "text",
    exp.StartsWith:    "text",
    exp.StrPosition:   "text",
    exp.RegexpLike:    "text",
    exp.RegexpReplace: "text",
    # Numeric functions, primary arg must be numeric
    exp.Abs:   "numeric",
    exp.Round: "numeric",
    exp.Ceil:  "numeric",
    exp.Floor: "numeric",
    exp.Sqrt:  "numeric",
    exp.Ln:    "numeric",
    exp.Log:   "numeric",
    exp.Exp:   "numeric",
    exp.Sign:  "numeric",
    # Boolean context, primary arg must be boolean
    exp.Not: "boolean",
}


def type_family(dt: exp.DataType.Type) -> str | None:
    if dt in _NUMERIC:  return "numeric"
    if dt in _TEXT:     return "text"
    if dt in _BOOLEAN:  return "boolean"
    if dt in _TEMPORAL: return "temporal"
    if dt in _INTERVAL: return "interval"
    return None


def resolved(dt) -> exp.DataType.Type | None:
    """Return the DataType.Type enum value, or None if unknown/unresolved."""
    if dt is None:
        return None
    t = dt.this if isinstance(dt, exp.DataType) else dt
    return None if t == _UNKNOWN else t


def compatible_for_comparison(lt: exp.DataType.Type, rt: exp.DataType.Type) -> bool:
    """True when lt and rt can be compared without a type error in standard SQL."""
    lf, rf = type_family(lt), type_family(rt)
    if lf is None or rf is None:
        return True  # unknown family → can't judge
    if lf == rf:
        return True
    # Temporal columns can be compared to text literals (implicit cast in PG)
    if lf == "temporal" and rf == "text":
        return True
    if lf == "text" and rf == "temporal":
        return True
    return False
