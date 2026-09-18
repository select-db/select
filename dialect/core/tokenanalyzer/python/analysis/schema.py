"""
Schema construction and shared utilities used across lint rule modules.
"""
from __future__ import annotations

from sqlglot import exp
from sqlglot.dialects.dialect import Dialect as SqlglotDialect
from sqlglot.errors import TokenError
from sqlglot.schema import MappingSchema

# Map Go dialect names → sqlglot dialect names
DIALECT_MAP = {
    "postgresql": "postgres",
    "postgres":   "postgres",
    "mysql":      "mysql",
    "sqlite":     "sqlite",
}


def sqlglot_dialect_name(go_dialect: str) -> str:
    """Map a Go dialect name onto sqlglot's. An unknown name passes through so
    sqlglot raises, rather than being parsed as some other dialect in silence.
    """
    name = (go_dialect or "").lower()
    return DIALECT_MAP.get(name, name)


def tokenize(sql: str, sg_dialect: str) -> list:
    """Tokenize with the dialect's own tokenizer. The generic one knows only
    double quotes, so MySQL backticks and SQLite brackets arrive as UNKNOWN
    tokens and disappear from every scan that looks for an identifier.
    """
    if not sg_dialect:
        raise ValueError("tokenize requires a dialect name")

    # Dialect.tokenize binds the tokenizer to the dialect. Building the class
    # directly leaves it on the generic settings, which differ in more than
    # quoting: MySQL allows an identifier to start with a digit, so 2fa reads
    # as the number 2 followed by fa.
    dialect = SqlglotDialect.get_or_raise(sg_dialect)
    try:
        return list(dialect.tokenize(sql))
    except TokenError:
        # A quote the caret sits inside is unterminated, which is the normal
        # state of an identifier or a string being typed. Closing it costs
        # nothing: the added character lands past the caret, so no earlier
        # token moves.
        closers = {
            **dialect.tokenizer_class._IDENTIFIERS,
            **dialect.tokenizer_class._QUOTES,
        }
        for closer in dict.fromkeys(closers.values()):
            try:
                return list(dialect.tokenize(sql + closer))
            except TokenError:
                continue
        raise


def pos(node: exp.Expression) -> tuple[int, int]:
    """
    Return where a node starts, for a caller that wants a point rather than a
    range: a reference position, not something to underline. Rules that mark
    source use span, so that what a reader sees covers the clause.

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
        extent = _token_extent(candidate)
        if extent is not None:
            return extent
    return _subtree_extent(node)


def _token_extent(node: exp.Expression) -> tuple[int, int, int] | None:
    """Return the span of the token sqlglot consumed to build node, if it kept one."""
    m = node.meta
    if not m or "line" not in m:
        return None
    token_len = int(m["end"]) - int(m["start"]) + 1
    start_col = max(0, int(m["col"]) - token_len)
    return int(m["line"]), start_col, start_col + token_len


def _subtree_extent(node: exp.Expression) -> tuple[int, int, int]:
    """
    Return the span of node's tagged descendants, on the line the first one is on.

    sqlglot tags the token it consumed, so a node built from a keyword keeps
    none: OFFSET, LIMIT, HAVING and every operator carry their position on the
    operands underneath. Without this a rule about one of them reports 1:0,
    which puts the diagnostic at the start of the file rather than on the
    clause it is about.
    """
    if not isinstance(node, exp.Expression):
        return 1, 0, 0

    extents = [e for e in (_token_extent(d) for d in node.walk()) if e is not None]
    if not extents:
        return 1, 0, 0

    line, start_col, _ = min(extents)
    end_col = max(e[2] for e in extents if e[0] == line)
    return line, start_col, end_col


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
