"""
stdin/stdout JSON server loop.

Protocol: newline-delimited JSON.

Request fields:
  action      , "lint" (default; future: "format", "rewrite", ...)
  sql         , SQL string to analyse
  dialect     , "postgresql" | "mysql" | "sqlite"
  schema      , { schema: { table: { col: type } } }
  default_schema, optional, defaults to "public"
  functions   , optional list of user-defined function names

Response: JSON object whose shape depends on the action. "error" is reserved:
a response carrying it is a failure, with "traceback" set when an exception
produced it, and no successful response has either key.

All positions are 1-based line, 0-based col (matching ANTLR convention used by
the Go layer).
"""
from __future__ import annotations

import json
import sys
import traceback

from completion import caret_patch


def _dispatch(req: dict) -> dict:
    # A completion request is patched on the way in and cleaned on the way
    # out, once here rather than in each handler: a handler added later cannot
    # forget either half.
    if "caret_line" in req:
        req = {**req, "sql": caret_patch.unwrap_explain(req.get("sql", ""))}
        return caret_patch.without_placeholders(_handle(req))
    return _handle(req)


def _handle(req: dict) -> dict:
    action = req.get("action", "lint")

    if action == "ping":
        return {"ok": True}

    if action == "lint":
        from analysis import analyze
        return analyze(
            sql=req.get("sql", ""),
            dialect=req.get("dialect", "postgresql"),
            schema_dict=req.get("schema", {}),
            default_schema=req.get("default_schema", "public"),
            functions=req.get("functions", []),
            enum_dict=req.get("enums", {}),
        )

    if action == "collect_references":
        return _collect_references(req)

    if action == "collect_column_refs":
        return _collect_column_refs(req)

    if action == "complete_context":
        return _complete_context(req)

    return {"error": f"Unknown action: {action!r}"}


def _prepare_sql(req: dict, for_completion: bool = False) -> tuple[str, list, dict, str, str]:
    """Parse SQL from a request, returning (sql, stmts, schema_dict, default_schema, sg_dialect).

    Handles $var replacement, dialect mapping, and empty-SQL short-circuit.
    Returns empty stmts list if SQL is blank.
    """
    import re
    from analysis.analyze import _parse_sql
    from analysis.schema import sqlglot_dialect_name

    sql = req.get("sql", "")
    dialect = req.get("dialect", "postgresql")
    schema_dict = req.get("schema", {})
    default_schema = req.get("default_schema", "public")

    if not sql or not sql.strip():
        return sql, [], schema_dict, default_schema, sqlglot_dialect_name(dialect)

    var_re = re.compile(r"\$([A-Za-z_][A-Za-z0-9_]*)")
    sql = var_re.sub("NULL", sql)

    if for_completion:
        caret_line = req.get("caret_line", 1)
        caret_col = req.get("caret_col", 0)
        sql = caret_patch.sanitize(sql, caret_line, caret_col)

    sg_dialect = sqlglot_dialect_name(dialect)
    stmts, _, _ = _parse_sql(sql, sg_dialect)
    return sql, stmts, schema_dict, default_schema, sg_dialect


def _collect_references(req: dict) -> dict:
    from completion.scope_references import collect_references

    for_completion = "caret_line" in req
    sql, stmts, schema_dict, default_schema, sg_dialect = _prepare_sql(req, for_completion=for_completion)
    if not stmts:
        return {"relations": [], "virtual_tables": []}
    return collect_references(sql, stmts, schema_dict, default_schema, sg_dialect)


def _collect_column_refs(req: dict) -> dict:
    from completion.column_resolution import collect_resolved_column_refs, collect_column_aliases

    for_completion = "caret_line" in req
    sql, stmts, schema_dict, default_schema, _ = _prepare_sql(req, for_completion=for_completion)
    if not stmts:
        return {"column_refs": [], "column_aliases": []}
    return {
        "column_refs":    collect_resolved_column_refs(stmts, schema_dict, default_schema),
        "column_aliases": collect_column_aliases(stmts, schema_dict, default_schema),
    }


def _complete_context(req: dict) -> dict:
    from analysis.schema import sqlglot_dialect_name
    from completion.completion_context import detect_completion_context

    sql = req.get("sql", "")
    caret_line = req.get("caret_line", 1)
    caret_col = req.get("caret_col", 0)
    schema_names = list(req.get("schema", {}).keys())
    sg_dialect = sqlglot_dialect_name(req.get("dialect", "postgresql"))

    return detect_completion_context(sql, caret_line, caret_col, schema_names, sg_dialect)


def serve() -> None:
    """Read newline-delimited JSON from stdin, write responses to stdout."""
    for raw_line in sys.stdin:
        line = raw_line.strip()
        if not line:
            continue
        try:
            req: dict = json.loads(line)
            response = _dispatch(req)
        except json.JSONDecodeError as e:
            response = {"error": f"Invalid JSON input: {e}"}
        except Exception as e:
            response = {"error": str(e), "traceback": traceback.format_exc()}

        print(json.dumps(response), flush=True)
