"""
Schema construction and shared utilities used across lint rule modules.
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.schema import MappingSchema

# Map Go dialect names → sqlglot dialect names
DIALECT_MAP = {
    "postgresql": "postgres",
    "postgres":   "postgres",
    "mysql":      "mysql",
    "sqlite":     "sqlite",
}


def pos(node: exp.Expression) -> tuple[int, int]:
    """
    Return the start (line, col) for a node using sqlglot token metadata.

    sqlglot stores position on the token that was consumed when the node was
    created. For compound nodes (Column, Table, Window, …) the meta lives on
    the primary child Identifier (node.this), not on the parent itself.
    meta["col"] is the *exclusive-end* column offset; we derive start col as:
        start_col = meta["col"] - (meta["end"] - meta["start"] + 1)
    Falls back to (1, 0) when no position is available.
    """
    line, start_col, _ = span(node)
    return line, start_col


def span(node: exp.Expression) -> tuple[int, int, int]:
    """
    Return (line, start_col, end_col) for a node.

    end_col is derived from the actual token length in the source, so quoted
    identifiers like "typo" correctly include both surrounding quotes.
    Falls back to (1, 0, 0) when no position is available.
    """
    for candidate in (node, node.this if isinstance(node, exp.Expression) else None):
        if not isinstance(candidate, exp.Expression):
            continue
        m = candidate.meta
        if m and "line" in m:
            token_len = m["end"] - m["start"] + 1
            start_col = m["col"] - token_len
            start_col = max(0, int(start_col))
            return int(m["line"]), start_col, start_col + token_len
    return 1, 0, 0


def build_schema(schema_dict: dict, dialect: str) -> MappingSchema | None:
    """
    Build a sqlglot MappingSchema from:
      { schema_name: { table_name: [col, ...] | { col: type, ... } } }
    Returns None if schema_dict is empty.
    """
    if not schema_dict:
        return None
    normalised: dict[str, dict[str, dict[str, str]]] = {}
    for schema_name, tables in schema_dict.items():
        normalised[schema_name] = {}
        for table_name, cols in tables.items():
            if isinstance(cols, list):
                normalised[schema_name][table_name] = {c: "TEXT" for c in cols}
            else:
                normalised[schema_name][table_name] = cols
    try:
        return MappingSchema(normalised, dialect=dialect)
    except Exception:
        return None


def collect_virtual_names(stmt: exp.Expression) -> set[str]:
    """
    Return the set of names that are virtual (CTEs + subquery aliases) so rule
    modules can skip them during unknown-table validation.
    sqlglot 25+ stores the WITH clause under "with_" internally; use find() to
    be version-agnostic.
    """
    virtuals: set[str] = set()
    with_clause = stmt.find(exp.With)
    if with_clause:
        for cte in with_clause.expressions:
            virtuals.add(cte.alias.lower())
    return virtuals
