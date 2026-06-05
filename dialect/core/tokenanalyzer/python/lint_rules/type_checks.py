"""
Type-aware lint rules (require a typed MappingSchema):
  T001, comparison type mismatch
  T002, arithmetic on non-numeric / non-temporal type
  T003, COALESCE arguments with inconsistent types
  T004, IN list value incompatible with column type
  T005, function argument of wrong type
  T006, function called with missing required arguments
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.optimizer import annotate_types, qualify_columns
from sqlglot.schema import MappingSchema

from analysis.schema import pos, span
from analysis.types import (
    _ARITHMETIC_SAFE,
    _FUNC_ARG_CONSTRAINTS,
    compatible_for_comparison,
    resolved,
    type_family,
)


def analyze_type_mismatches(
    stmt: exp.Expression,
    schema: MappingSchema,
    sg_dialect: str,
) -> list[dict]:
    results: list[dict] = []
    seen: set[tuple] = set()

    def _emit(rule_id: str, severity: str, message: str, node: exp.Expression) -> None:
        line, col = pos(node)
        key = (rule_id, str(node)[:80])
        if key not in seen:
            seen.add(key)
            results.append({
                "rule_id":    rule_id,
                "severity":   severity,
                "message":    message,
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   col,
            })

    try:
        qualified = qualify_columns.qualify_columns(stmt.copy(), schema=schema, dialect=sg_dialect)
        annotated = annotate_types.annotate_types(qualified, schema=schema, dialect=sg_dialect)
    except Exception:
        return []  # incomplete SQL or unresolvable refs, skip type analysis

    # T001, comparison type mismatches
    for node in annotated.find_all(exp.EQ, exp.NEQ, exp.LT, exp.GT, exp.LTE, exp.GTE):
        lt = resolved(node.left.type if node.left else None)
        rt = resolved(node.right.type if node.right else None)
        if lt is None or rt is None:
            continue
        if not compatible_for_comparison(lt, rt):
            lf, rf = type_family(lt), type_family(rt)
            _emit("type-mismatch-comparison", "warning", f"Type mismatch in comparison: {lf} ({lt.value}) vs {rf} ({rt.value})", node)

    # T002, arithmetic on non-arithmetic types
    for node in annotated.find_all(exp.Add, exp.Sub, exp.Mul, exp.Div, exp.Mod):
        for operand in (node.left, node.right):
            if operand is None:
                continue
            t = resolved(operand.type)
            if t is None:
                continue
            if t not in _ARITHMETIC_SAFE:
                family = type_family(t) or t.value
                _emit("type-mismatch-arithmetic", "error", f"Arithmetic on non-numeric column: type is {family} ({t.value})", operand)

    # T003, COALESCE with inconsistent argument types
    for node in annotated.find_all(exp.Coalesce):
        all_args = ([node.this] if node.this else []) + list(node.expressions)
        types    = [resolved(a.type) for a in all_args]
        families = [type_family(t) for t in types if t is not None]
        known    = [f for f in families if f is not None]
        if len(set(known)) > 1:
            _emit("coalesce-type-mismatch", "warning", f"COALESCE arguments have inconsistent types: {', '.join(dict.fromkeys(known))}", node)

    # T004, IN list value type mismatch
    for node in annotated.find_all(exp.In):
        col_t = resolved(node.this.type if node.this else None)
        if col_t is None:
            continue
        col_family = type_family(col_t)
        if col_family is None:
            continue
        for val in node.args.get("expressions", []):
            vt = resolved(val.type)
            if vt is None:
                continue
            vf = type_family(vt)
            if vf is not None and vf != col_family:
                _emit("in-list-type-mismatch", "warning", f"IN list value type {vf} ({vt.value}) incompatible with column type {col_family} ({col_t.value})", val)

    # T005, function argument type mismatch
    for func_class, expected_family in _FUNC_ARG_CONSTRAINTS.items():
        for node in annotated.find_all(func_class):
            arg = node.args.get("this") or node.args.get("expression")
            if not isinstance(arg, exp.Expression):
                continue
            t = resolved(arg.type)
            if t is None:
                continue
            actual_family = type_family(t)
            if actual_family is None:
                continue
            if actual_family != expected_family:
                func_name = type(node).__name__.upper()
                _emit("type-mismatch-argument", "warning", f"{func_name} expects a {expected_family} argument but received {actual_family} ({t.value})", arg)

    return results


# ---------------------------------------------------------------------------
# T006, Missing required function arguments
# ---------------------------------------------------------------------------

def analyze_arity_errors(stmt: exp.Expression) -> list[dict]:
    """
    T006, function called with no argument where at least one is required.

    Two generic signals from sqlglot's own machinery (no hardcoded name list):

    1. arg_types scan, if any slot is marked required=True and is absent, the
       parser would already have emitted a "Required keyword missing" warning
       (handled by _parse_sql in analyze.py).  This walk catches any stragglers
       that slipped through without a parser warning.

    2. AggFunc class hierarchy, every sqlglot aggregate (COUNT, SUM, AVG, …)
       subclasses exp.AggFunc.  An aggregate with neither a primary expression
       (this) nor a variadic list (expressions) has no argument at all.
    """
    results: list[dict] = []
    seen: set[tuple] = set()

    for func in stmt.find_all(exp.Func):
        missing = False

        # Signal 1: required arg slot is absent (generic arg_types check).
        for arg_name, required in func.arg_types.items():
            if required and func.args.get(arg_name) is None:
                missing = True
                break

        # Signal 2: aggregate with no expression at all (uses class hierarchy).
        if not missing and isinstance(func, exp.AggFunc):
            has_primary = func.args.get("this") is not None
            has_variadic = bool(func.args.get("expressions"))
            if not has_primary and not has_variadic:
                missing = True

        if not missing:
            continue

        try:
            func_name = func.sql_name().upper()
        except Exception:
            func_name = type(func).__name__.upper()

        line, col, end_col = span(func)
        key = (func_name, line, col)
        if key not in seen:
            seen.add(key)
            results.append({
                "rule_id":    "missing-argument",
                "severity":   "error",
                "message":    f"{func_name} requires at least one argument",
                "start_line": line, "start_col": col,
                "end_line":   line, "end_col":   end_col,
            })

    return results
