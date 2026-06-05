"""
Completion context detection via SQLGlot tokenizer.
Scans tokens backward from caret to determine what to complete.
"""
from __future__ import annotations

from sqlglot.tokens import Tokenizer, TokenType

# Must match Go constants
TARGET_SCHEMA   = 1 << 0
TARGET_TABLE    = 1 << 1
TARGET_COLUMN   = 1 << 2
_REF_ONLY_FLAG  = 1 << 3
_ALL_FLAG       = 1 << 4
TARGET_OPERATOR = 1 << 5
TARGET_ENUM_VALUE = 1 << 6
TARGET_SETTING = 1 << 7

TARGET_SCHEMA_AND_TABLE      = TARGET_SCHEMA | TARGET_TABLE
TARGET_SCHEMA_AND_TABLE_ALL  = TARGET_SCHEMA | TARGET_TABLE | _ALL_FLAG
TARGET_TABLE_AND_COLUMN      = TARGET_TABLE | TARGET_COLUMN
TARGET_ALL                   = TARGET_SCHEMA | TARGET_TABLE | TARGET_COLUMN


def detect_completion_context(
    sql: str,
    caret_line: int,
    caret_col: int,
    schema_names: list[str],
) -> dict:
    caret_offset = _line_col_to_offset(sql, caret_line, caret_col)
    tokens = _tokenize_up_to(sql, caret_offset)

    if _detect_setting_context(tokens, sql, caret_offset):
        return {
            "parts":              [],
            "caret_after_dot":    False,
            "targets":            TARGET_SETTING,
            "schema_filter":      "",
            "target_table":       "",
            "keyword_context":    0,
            "preceding_column":   None,
            "insert_target_table": "",
            "value_position":     False,
        }

    parts, caret_after_dot = _parse_qualified_parts(tokens)
    keyword_ctx = _detect_keyword_context(tokens)

    targets = keyword_ctx
    schema_filter = ""
    target_table = ""

    if parts:
        targets, schema_filter, target_table = _determine_targets(
            parts, caret_after_dot, schema_names,
        )

    insert_target = ""
    preceding_column = None
    value_position = False

    if not caret_after_dot and not parts:
        insert_target = _detect_insert_column_list(tokens)
        if insert_target:
            targets = TARGET_COLUMN
        else:
            preceding_column = _detect_preceding_column(tokens, caret_offset)
            if preceding_column:
                if keyword_ctx == TARGET_TABLE_AND_COLUMN:
                    targets = TARGET_OPERATOR
                elif keyword_ctx == TARGET_ALL:
                    targets = 0
            else:
                value_col = _detect_value_position(tokens, caret_offset)
                if value_col:
                    preceding_column = value_col
                    value_position = True
                    targets = TARGET_ENUM_VALUE

    return {
        "parts":              parts,
        "caret_after_dot":    caret_after_dot,
        "targets":            targets,
        "schema_filter":      schema_filter,
        "target_table":       target_table,
        "keyword_context":    keyword_ctx,
        "preceding_column":   preceding_column,
        "insert_target_table": insert_target,
        "value_position":     value_position,
    }


# --- Tokenization ---

def _line_col_to_offset(sql: str, line: int, col: int) -> int:
    current_line = 1
    for i, ch in enumerate(sql):
        if current_line == line:
            if col == 0:
                return i
            col -= 1
        if ch == '\n':
            current_line += 1
    return len(sql)


def _tokenize_up_to(sql: str, caret_offset: int) -> list:
    all_tokens = list(Tokenizer().tokenize(sql))
    return [t for t in all_tokens if t.start < caret_offset]


# --- Qualified identifier parsing ---

def _parse_qualified_parts(tokens: list) -> tuple[list[str], bool]:
    """Parse a qualified chain ending at caret: "schema.table." → (["schema","table"], True)"""
    if not tokens:
        return [], False

    last = tokens[-1]
    if last.token_type != TokenType.DOT:
        return [], False

    parts = []
    i = len(tokens) - 1

    while i >= 0:
        if tokens[i].token_type != TokenType.DOT:
            break
        i -= 1
        if i < 0:
            break

        tok = tokens[i]
        if not _is_identifier_token(tok):
            break
        parts.append(_normalize_identifier(tok.text))
        i -= 1

    parts.reverse()
    return parts, True


# --- Setting (runtime parameter) detection ---

def _detect_setting_context(tokens: list, sql: str, caret_offset: int) -> bool:
    """True when the caret is positioned to complete a runtime parameter:
    MySQL @@var, Postgres SHOW <param>, SQLite PRAGMA <param>."""
    i = caret_offset
    while i > 0 and (sql[i-1].isalnum() or sql[i-1] == '_'):
        i -= 1
    partial_start = i

    if partial_start >= 2 and sql[partial_start-2:partial_start] == '@@':
        return True

    prev = [t for t in tokens if t.start < partial_start]
    if not prev:
        return False
    return (prev[-1].text or "").upper() in ("SHOW", "PRAGMA")


# --- Keyword context detection ---

# (keyword_text, target, top_level_only). First match wins.
# SQLGlot combines compound keywords into single tokens.
_KEYWORD_MATCHERS: list[tuple[str, int, bool]] = [
    ("ORDER BY",    TARGET_TABLE_AND_COLUMN, False),
    ("GROUP BY",    TARGET_TABLE_AND_COLUMN, False),
    ("INSERT INTO", TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("AS",          0, False),
    ("AND",         TARGET_TABLE_AND_COLUMN, False),
    ("OR",          TARGET_TABLE_AND_COLUMN, False),
    ("HAVING",      TARGET_TABLE_AND_COLUMN, False),
    ("WHERE",       TARGET_TABLE_AND_COLUMN, False),
    ("ON",          TARGET_TABLE_AND_COLUMN, False),
    ("FROM",        TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("USING",       TARGET_COLUMN, False),
    ("INNER",       TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("LEFT",        TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("RIGHT",       TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("FULL",        TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("CROSS",       TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("NATURAL",     TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("JOIN",        TARGET_SCHEMA_AND_TABLE_ALL, False),
    ("SET",         TARGET_COLUMN, False),
    ("VALUES",      TARGET_COLUMN, False),
    ("SELECT",      TARGET_ALL, True),
    ("UPDATE",      TARGET_SCHEMA_AND_TABLE_ALL, True),
]


def _detect_keyword_context(tokens: list) -> int:
    if not tokens:
        return TARGET_SCHEMA_AND_TABLE_ALL

    paren_depth = 0
    seen_close_paren = False
    i = len(tokens) - 1

    if i >= 0 and tokens[i].token_type == TokenType.DOT:
        i -= 1

    while i >= 0:
        tok = tokens[i]
        tt = tok.token_type
        upper = tok.text.upper()

        if tt == TokenType.R_PAREN:
            paren_depth += 1
            seen_close_paren = True
            i -= 1
            continue

        if tt == TokenType.L_PAREN:
            if paren_depth > 0:
                paren_depth -= 1
                i -= 1
                continue
            if i > 0 and tokens[i - 1].text.upper() == "VALUES":
                return TARGET_COLUMN
            return TARGET_SCHEMA_AND_TABLE_ALL

        if tt == TokenType.SEMICOLON:
            break

        if paren_depth > 0:
            i -= 1
            continue

        for kw_text, target, top_level_only in _KEYWORD_MATCHERS:
            if top_level_only and paren_depth > 0:
                continue
            if upper == kw_text:
                return target

        if paren_depth == 0 and upper in ("SELECT", "UPDATE", "INSERT", "DELETE",
                                           "INSERT INTO"):
            break

        i -= 1

    return TARGET_SCHEMA_AND_TABLE_ALL


# --- INSERT column list detection ---

def _detect_insert_column_list(tokens: list) -> str:
    """Detect INSERT INTO <table> (col1, |) and return the table name."""
    paren_depth = 0
    i = len(tokens) - 1

    while i >= 0:
        tok = tokens[i]
        tt = tok.token_type
        upper = tok.text.upper()

        if tt == TokenType.R_PAREN:
            paren_depth += 1
        elif tt == TokenType.L_PAREN:
            if paren_depth > 0:
                paren_depth -= 1
            else:
                if i >= 2:
                    table_tok = tokens[i - 1]
                    prev_tok = tokens[i - 2]
                    if (_is_identifier_token(table_tok)
                            and prev_tok.text.upper() in ("INSERT INTO", "INTO")):
                        return _normalize_identifier(table_tok.text)
                return ""
        elif tt == TokenType.SEMICOLON:
            return ""
        elif upper == "VALUES":
            return ""
        elif paren_depth == 0 and upper in ("SELECT", "UPDATE", "DELETE", "INSERT"):
            return ""

        i -= 1

    return ""


# --- Preceding column detection ---

def _detect_preceding_column(tokens: list, caret_offset: int) -> dict | None:
    """Return the column ref if the last token is a finished identifier, else None."""
    if not tokens:
        return None

    last = tokens[-1]
    if not _is_identifier_token(last):
        return None

    if last.end + 1 >= caret_offset:
        return None

    name = _normalize_identifier(last.text)
    if not name:
        return None

    if len(tokens) >= 3:
        dot = tokens[-2]
        qualifier = tokens[-3]
        if dot.token_type == TokenType.DOT and _is_identifier_token(qualifier):
            table_name = _normalize_identifier(qualifier.text)
            return {"name": f"{table_name}.{name}"}

    return {"name": name}


# --- Value-position detection (enum completion) ---

def _column_ref_at(tokens: list, idx: int) -> dict | None:
    if idx < 0 or idx >= len(tokens):
        return None
    tok = tokens[idx]
    if not _is_identifier_token(tok):
        return None
    name = _normalize_identifier(tok.text)
    if idx >= 2 and tokens[idx - 1].token_type == TokenType.DOT \
            and _is_identifier_token(tokens[idx - 2]):
        return {"name": f"{_normalize_identifier(tokens[idx - 2].text)}.{name}"}
    return {"name": name}


def _detect_value_position(tokens: list, caret_offset: int) -> dict | None:
    """Detect caret in enum value slot: col = '|', col IN ('|'), etc."""
    if not tokens:
        return None

    i = len(tokens) - 1

    last = tokens[i]
    if last.token_type == TokenType.STRING:
        if not (last.start < caret_offset <= last.end + 1):
            return None
        i -= 1

    if i < 0:
        return None

    if tokens[i].token_type in (TokenType.EQ, TokenType.NEQ):
        return _column_ref_at(tokens, i - 1)

    j = i
    while j >= 0 and tokens[j].token_type in (TokenType.STRING, TokenType.COMMA):
        j -= 1
    if j >= 0 and tokens[j].token_type == TokenType.L_PAREN \
            and j >= 1 and tokens[j - 1].token_type == TokenType.IN:
        return _column_ref_at(tokens, j - 2)

    return None


# --- Token utilities ---

_IDENTIFIER_TYPES = {
    TokenType.VAR, TokenType.IDENTIFIER,
}

_TYPE_KEYWORD_TOKENS = {
    TokenType.UUID, TokenType.BOOLEAN, TokenType.INT, TokenType.TEXT,
    TokenType.FLOAT, TokenType.DOUBLE, TokenType.DECIMAL, TokenType.BIGINT,
    TokenType.SMALLINT, TokenType.TIMESTAMP, TokenType.DATE,
    TokenType.BINARY, TokenType.CHAR, TokenType.VARCHAR,
}

_KEYWORD_AS_IDENTIFIER = {
    "NAME", "TYPE", "KEY", "VALUE", "COMMENT", "STATUS",
    "ROLE", "OWNER", "DATA", "FORMAT", "LANGUAGE",
}


def _is_identifier_token(tok) -> bool:
    if tok.token_type in _IDENTIFIER_TYPES:
        return True
    if tok.token_type in _TYPE_KEYWORD_TOKENS:
        return True
    if tok.text.startswith('"') and tok.text.endswith('"'):
        return True
    if tok.text.upper() in _KEYWORD_AS_IDENTIFIER:
        return True
    return False


def _normalize_identifier(text: str) -> str:
    if text.startswith('"') and text.endswith('"') and len(text) >= 2:
        return text[1:-1]
    return text


def _determine_targets(
    parts: list[str],
    caret_after_dot: bool,
    schema_names: list[str],
) -> tuple[int, str, str]:
    if not caret_after_dot:
        return TARGET_ALL, "", ""

    schema_set = {s.lower() for s in schema_names}

    if len(parts) == 0:
        return TARGET_COLUMN, "", ""

    if len(parts) == 1:
        if parts[0].lower() in schema_set:
            return TARGET_TABLE, parts[0], ""
        return TARGET_COLUMN, "", parts[0]

    schema_filter = parts[0] if parts[0].lower() in schema_set else ""
    return TARGET_COLUMN, schema_filter, parts[-1]
