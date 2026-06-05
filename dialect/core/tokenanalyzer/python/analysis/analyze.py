from __future__ import annotations

import re
from typing import Any

from sqlglot.dialects.dialect import Dialect as SqlglotDialect
from sqlglot.errors import ErrorLevel, ParseError, SqlglotError
from sqlglot.optimizer.scope import traverse_scope

from lint_rules.aggregation import analyze_aggregation_rules
from analysis.collect import collect_column_refs, collect_relations
from lint_rules.enum_values import analyze_enum_values
from lint_rules.functions import analyze_unknown_functions
from lint_rules.nullability import analyze_null_rules
from lint_rules.orderby import analyze_orderby_rules
from lint_rules.references import (
    analyze_ambiguous_columns,
    analyze_dead_ctes,
    analyze_delete_columns,
    analyze_insert_columns,
    analyze_merge_columns,
    analyze_unknown_columns,
    analyze_unknown_tables,
    analyze_update_columns,
)
from analysis.schema import DIALECT_MAP, build_schema, collect_virtual_names
from lint_rules.structure import (
    analyze_duplicate_columns,
    analyze_ordinal_violations,
    analyze_window_in_where,
)
from lint_rules.style import analyze_style_rules
from lint_rules.type_checks import analyze_arity_errors, analyze_type_mismatches

_MISSING_ARG_RE = re.compile(r"Required keyword: '\w+' missing")

# Matches selectDb $variable references (non-numeric to avoid $1 positional params).
# PostgreSQL's tokenizer treats $word as a dollar-quoted string start ($tag$...$tag$)
# which corrupts line numbers for every token that follows. Replace with NULL before
# parsing so positions are preserved (NULL has no newlines, just like $varname).
_SELECTDB_VAR_RE = re.compile(r"\$([A-Za-z_][A-Za-z0-9_]*)")


def _parse_sql(sql: str, sg_dialect: str) -> tuple[list, list[dict], list[dict]]:
    """Parse sql, returning (statements, parse_errors, arity_diagnostics).

    Missing-argument errors are separated into arity_diagnostics so T006
    surfaces as a lint diagnostic rather than a parse error.
    """
    parse_errors: list[dict] = []
    arity_diags: list[dict] = []

    try:
        d = SqlglotDialect.get_or_raise(sg_dialect)
        p = d.parser(error_level=ErrorLevel.WARN)
        statements = p.parse(d.tokenize(sql), sql)
    except SqlglotError as e:
        parse_errors.append({"message": str(e), "line": 1, "col": 0})
        return [], parse_errors, arity_diags
    except Exception as e:
        parse_errors.append({"message": f"Unexpected parse error: {e}", "line": 1, "col": 0})
        return [], parse_errors, arity_diags

    for exc in p.errors:
        if not isinstance(exc, ParseError):
            continue
        for err in exc.errors:
            desc = err.get("description", "")
            line = err.get("line", 1)
            col  = err.get("col", 0)  # exclusive-end column (sqlglot convention)

            if _MISSING_ARG_RE.search(desc):
                # start_context ends with "FUNCNAME(", extract the name to get its start col.
                start_ctx = err.get("start_context", "")
                last_line = start_ctx.split("\n")[-1]
                func_name = last_line.rstrip("(").split()[-1] if last_line.rstrip("(").strip() else "?"
                start_col = max(0, col - 2 - len(func_name))
                arity_diags.append({
                    "rule_id":    "missing-argument",
                    "severity":   "error",
                    "message":    f"{func_name.upper()} requires at least one argument",
                    "start_line": line, "start_col": start_col,
                    "end_line":   line, "end_col":   start_col + len(func_name),
                })
            else:
                parse_errors.append({"message": desc or str(exc), "line": line, "col": col})

    return statements, parse_errors, arity_diags


def analyze(
    sql: str,
    dialect: str = "postgresql",
    schema_dict: dict | None = None,
    default_schema: str = "public",
    functions: list[str] | None = None,
    enum_dict: dict | None = None,
) -> dict[str, Any]:
    result: dict[str, Any] = {
        "diagnostics": [],
        "relations":   [],
        "column_refs": [],
        "parse_errors": [],
    }

    user_functions: set[str] = {f.lower() for f in (functions or [])}

    if not sql or not sql.strip():
        return result

    # Replace $varname placeholders with NULL so sqlglot's PostgreSQL tokenizer
    # doesn't misparse them as dollar-quoted string delimiters ($tag$...$tag$),
    # which corrupts line numbers for all subsequent tokens.
    sql = _SELECTDB_VAR_RE.sub("NULL", sql)

    schema_dict = schema_dict or {}
    enum_dict   = enum_dict or {}
    sg_dialect  = DIALECT_MAP.get(dialect.lower(), dialect.lower())
    schema      = build_schema(schema_dict, sg_dialect)

    statements, parse_errors, arity_diags = _parse_sql(sql, sg_dialect)
    result["parse_errors"].extend(parse_errors)

    # Deduplicate by (rule_id, start_line, start_col), the parser and AST scan
    # can both emit T006 for the same call site.
    seen_diag: set[tuple] = set()

    def _add(items: list[dict]) -> None:
        for item in items:
            key = (item["rule_id"], item["start_line"], item["start_col"])
            if key not in seen_diag:
                seen_diag.add(key)
                result["diagnostics"].append(item)

    _add(arity_diags)

    if not statements:
        return result

    user_agg_names: set[str] = user_functions

    for stmt in statements:
        if stmt is None:
            continue

        try:
            scopes = list(traverse_scope(stmt))
        except Exception as e:
            result["parse_errors"].append({"message": f"Scope error: {e}", "line": 1, "col": 0})
            scopes = []

        virtual_names = collect_virtual_names(stmt)

        result["relations"].extend(collect_relations(scopes, default_schema))
        result["column_refs"].extend(collect_column_refs(stmt))

        _add(analyze_null_rules(stmt))
        _add(analyze_style_rules(stmt, sg_dialect, sql))
        _add(analyze_aggregation_rules(stmt, user_agg_names))
        _add(analyze_orderby_rules(stmt))

        _add(analyze_unknown_tables(stmt, schema_dict, default_schema, virtual_names))
        _add(analyze_unknown_columns(scopes, schema_dict, default_schema, sg_dialect))
        _add(analyze_update_columns(stmt, schema_dict, default_schema))
        _add(analyze_insert_columns(stmt, schema_dict, default_schema))
        _add(analyze_delete_columns(stmt, schema_dict, default_schema))
        _add(analyze_merge_columns(stmt, schema_dict, default_schema))
        _add(analyze_ambiguous_columns(scopes, schema_dict, default_schema))
        _add(analyze_dead_ctes(stmt))
        _add(analyze_unknown_functions(stmt, sg_dialect, user_functions))
        _add(analyze_ordinal_violations(stmt))
        _add(analyze_window_in_where(stmt))
        _add(analyze_duplicate_columns(stmt))
        _add(analyze_arity_errors(stmt))
        _add(analyze_enum_values(stmt, enum_dict, default_schema))

        if schema is not None:
            _add(analyze_type_mismatches(stmt, schema, sg_dialect))

    return result
