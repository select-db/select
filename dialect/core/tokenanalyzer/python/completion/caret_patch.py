"""Patching SQL so sqlglot can see the statement being typed, and undoing the
patch on the way out.

sqlglot parses statements, and a statement being typed is rarely one. The
caret sits after a dot, a comma or an open paren more often than not, so a
name is written in to give the parser something to hold. That name is this
module's invention, and nothing downstream may report it back as a relation or
a column somebody can complete.

A patch never moves what precedes the caret, because the caret is found by
offset; unwrap_explain moves nothing at all.
"""
from __future__ import annotations

import re

PLACEHOLDER = '__placeholder__'

# sqlglot reads what follows EXPLAIN as one opaque string on postgres and
# sqlite, so a caret anywhere inside sees no statement at all. Its options go
# with it, since what is left does not parse with them in front.
_EXPLAIN = re.compile(
    r"""(?:\A|(?<=;))                              # only where a statement may begin
        (?:\s|--[^\n]*|\#[^\n]*|/\*.*?\*/)*         # comments and space before it
        (EXPLAIN\b                                 # the group that gets blanked
         (?:\s*\([^)]*\))?                         # EXPLAIN (ANALYZE, VERBOSE)
         (?:\s+(?:ANALYZE|VERBOSE|COSTS|BUFFERS|TIMING|SUMMARY|WAL|EXTENDED
                  |QUERY\s+PLAN                     # sqlite
                  |FORMAT\s*=?\s*\w+))*)             # mysql writes FORMAT=JSON
    """,
    re.IGNORECASE | re.VERBOSE | re.DOTALL,
)


def unwrap_explain(sql: str) -> str:
    """Blank every statement's leading EXPLAIN and its options, leaving the
    statement under them to be read."""
    out = sql
    for match in _EXPLAIN.finditer(sql):
        out = out[:match.start(1)] + _blanked(match.group(1)) + out[match.end(1):]
    return out


def _blanked(text: str) -> str:
    """The same text as whitespace. Newlines survive, so a caret on a later
    line is still on it."""
    return ''.join(ch if ch == '\n' else ' ' for ch in text)


def sanitize(sql: str, caret_line: int, caret_col: int) -> str:
    """Patch incomplete SQL at the caret position so SQLGlot can parse it.

    Common patterns:
      - Trailing dot: "table.|" -> "table.x" (add dummy column)
      - Trailing comma: "col1, |" -> "col1, x" (add dummy identifier)
      - Mid-statement caret: strip everything after caret
    """
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
        return stripped + PLACEHOLDER + ' ' + after

    # If the character before caret is a comma or open paren, add a dummy
    if stripped.endswith(',') or stripped.endswith('('):
        return stripped + ' ' + PLACEHOLDER + ' ' + after

    return sql


def without_placeholders(response: dict) -> dict:
    """Drop everything the patch put there, at any depth.

    A collector names the placeholder in whichever field it names anything in,
    and a virtual table carries one among its columns, so this asks what an
    entry is called rather than where it is called that.
    """
    return _clean_mapping(response)


def _clean_mapping(value: dict) -> dict:
    return {key: _clean(item) for key, item in value.items()}


def _clean(value):
    if isinstance(value, dict):
        return _clean_mapping(value)
    if isinstance(value, list):
        return [_clean(item) for item in value if not _is_placeholder(item)]
    return value


def _is_placeholder(item) -> bool:
    return isinstance(item, dict) and PLACEHOLDER in (
        v for v in item.values() if isinstance(v, str)
    )
