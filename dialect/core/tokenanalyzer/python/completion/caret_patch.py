"""Patching incomplete SQL at the caret, and undoing the patch on the way out.

sqlglot parses statements, and a statement being typed is rarely one. The
caret sits after a dot, a comma or an open paren more often than not, so a
name is written in to give the parser something to hold. That name is this
module's invention, and nothing downstream may report it back as a relation or
a column somebody can complete.
"""
from __future__ import annotations

PLACEHOLDER = '__placeholder__'


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
