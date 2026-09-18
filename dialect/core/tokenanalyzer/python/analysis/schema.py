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
    """Where a node starts. Use span to mark a range. Falls back to (1, 0)."""
    line, col, _, _ = span(node)
    return line, col


def span(node: exp.Expression) -> tuple[int, int, int, int]:
    """
    Return (start_line, start_col, end_line, end_col) covering node in the source.

    The end column comes from the token's own length, so a quoted identifier
    includes its quotes. Falls back to (1, 0, 1, 0) when nothing is tagged.
    """
    if not isinstance(node, exp.Expression):
        return 1, 0, 1, 0

    for candidate in (node, node.this):
        if isinstance(candidate, exp.Expression):
            extent = _token_extent(candidate)
            if extent is not None:
                return extent

    return _subtree_extent(node)


def _token_extent(node: exp.Expression) -> tuple[int, int, int, int] | None:
    """The span of the token sqlglot consumed to build node, if it kept one."""
    m = node.meta
    if not m or "line" not in m:
        return None
    token_len = m["end"] - m["start"] + 1
    start_col = max(0, m["col"] - token_len)
    return m["line"], start_col, m["line"], start_col + token_len


def _subtree_extent(node: exp.Expression) -> tuple[int, int, int, int]:
    """
    The span of node's tagged descendants, first token to last.

    sqlglot tags the token it consumed, so a node built from a keyword keeps
    none: OFFSET, HAVING and every operator carry theirs on the operands below.
    """
    starts, ends = [], []
    for descendant in node.walk():
        extent = _token_extent(descendant)
        if extent is not None:
            starts.append((extent[0], extent[1]))
            ends.append((extent[2], extent[3]))
    if not starts:
        return 1, 0, 1, 0

    start_line, start_col = min(starts)
    end_line, end_col = max(ends)
    return start_line, start_col, end_line, end_col


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
