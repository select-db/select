"""
stdin/stdout JSON server loop.

Protocol: newline-delimited JSON.

Request fields:
  action      , "lint" (default; future: "format", "rewrite", …)
  sql         , SQL string to analyse
  dialect     , "postgresql" | "mysql" | "sqlite"
  schema      , { schema: { table: { col: type } } }
  default_schema, optional, defaults to "public"
  functions   , optional list of user-defined function names

Response: JSON object whose shape depends on the action.

All positions are 1-based line, 0-based col (matching ANTLR convention used by
the Go layer).
"""
from __future__ import annotations

import json
import sys
import traceback


def _dispatch(req: dict) -> dict:
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


def _prepare_sql(req: dict, for_completion: bool = False) -> tuple[str, list, dict, str]:
    """Parse SQL from a request, returning (sql, stmts, schema_dict, default_schema).

    Handles $var replacement, dialect mapping, and empty-SQL short-circuit.
    Returns empty stmts list if SQL is blank.
    """
    import re
    from analysis.analyze import _parse_sql
    from analysis.schema import DIALECT_MAP

    sql = req.get("sql", "")
    dialect = req.get("dialect", "postgresql")
    schema_dict = req.get("schema", {})
    default_schema = req.get("default_schema", "public")

    if not sql or not sql.strip():
        return sql, [], schema_dict, default_schema

    var_re = re.compile(r"\$([A-Za-z_][A-Za-z0-9_]*)")
    sql = var_re.sub("NULL", sql)

    if for_completion:
        caret_line = req.get("caret_line", 1)
        caret_col = req.get("caret_col", 0)
        sql = _sanitize_for_completion(sql, caret_line, caret_col)

    sg_dialect = DIALECT_MAP.get(dialect.lower(), dialect.lower())
    stmts, _, _ = _parse_sql(sql, sg_dialect)
    return sql, stmts, schema_dict, default_schema


def _sanitize_for_completion(sql: str, caret_line: int, caret_col: int) -> str:
    """Patch incomplete SQL at the caret position so SQLGlot can parse it.

    Common patterns:
      - Trailing dot: "table.|" → "table.x" (add dummy column)
      - Trailing comma: "col1, |" → "col1, x" (add dummy identifier)
      - Mid-statement caret: strip everything after caret
    """
    import re
    # Convert caret to offset
    offset = 0
    current_line = 1
    for i, ch in enumerate(sql):
        if current_line == caret_line:
            if caret_col == 0:
                offset = i
                break
            caret_col -= 1
        if ch == '\n':
            current_line += 1
    else:
        offset = len(sql)

    before = sql[:offset]
    after = sql[offset:]

    # If the character before caret is a dot, add a dummy identifier
    stripped = before.rstrip()
    if stripped.endswith('.'):
        return stripped + '__placeholder__ ' + after

    # If the character before caret is a comma or open paren, add a dummy
    if stripped.endswith(',') or stripped.endswith('('):
        return stripped + ' __placeholder__ ' + after

    return sql


def _collect_references(req: dict) -> dict:
    from completion.scope_references import collect_references

    for_completion = "caret_line" in req
    sql, stmts, schema_dict, default_schema = _prepare_sql(req, for_completion=for_completion)
    if not stmts:
        return {"relations": [], "virtual_tables": []}
    return collect_references(sql, stmts, schema_dict, default_schema)


def _collect_column_refs(req: dict) -> dict:
    from completion.column_resolution import collect_resolved_column_refs, collect_column_aliases

    for_completion = "caret_line" in req
    sql, stmts, schema_dict, default_schema = _prepare_sql(req, for_completion=for_completion)
    if not stmts:
        return {"column_refs": [], "column_aliases": []}
    return {
        "column_refs":    collect_resolved_column_refs(stmts, schema_dict, default_schema),
        "column_aliases": collect_column_aliases(stmts, schema_dict, default_schema),
    }


def _complete_context(req: dict) -> dict:
    from completion.completion_context import detect_completion_context

    sql = req.get("sql", "")
    caret_line = req.get("caret_line", 1)
    caret_col = req.get("caret_col", 0)
    schema_names = list(req.get("schema", {}).keys())

    if not sql:
        return {"parts": [], "caret_after_dot": False, "targets": 0,
                "schema_filter": "", "target_table": "",
                "keyword_context": 0, "preceding_column": None,
                "insert_target_table": ""}

    return detect_completion_context(sql, caret_line, caret_col, schema_names)


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
