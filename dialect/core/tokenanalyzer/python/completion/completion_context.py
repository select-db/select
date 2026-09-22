"""
Completion context detection via SQLGlot tokenizer.
Scans tokens backward from caret to determine what to complete.
"""
from __future__ import annotations

from typing import NamedTuple

from sqlglot.tokens import TokenType

from analysis.schema import tokenize

# Must match Go constants
TARGET_SCHEMA   = 1 << 0
TARGET_TABLE    = 1 << 1
TARGET_COLUMN   = 1 << 2
_REF_ONLY_FLAG  = 1 << 3
_ALL_FLAG       = 1 << 4
TARGET_OPERATOR = 1 << 5
TARGET_ENUM_VALUE = 1 << 6
TARGET_SETTING = 1 << 7
TARGET_KEYWORD = 1 << 8

TARGET_SCHEMA_AND_TABLE      = TARGET_SCHEMA | TARGET_TABLE
TARGET_SCHEMA_AND_TABLE_ALL  = TARGET_SCHEMA | TARGET_TABLE | _ALL_FLAG
TARGET_TABLE_AND_COLUMN      = TARGET_TABLE | TARGET_COLUMN
TARGET_ALL                   = TARGET_SCHEMA | TARGET_TABLE | TARGET_COLUMN


def detect_completion_context(
    sql: str,
    caret_line: int,
    caret_col: int,
    schema_names: list[str],
    sg_dialect: str,
) -> dict:
    caret_offset = _line_col_to_offset(sql, caret_line, caret_col)
    tokens = _tokenize_up_to(sql, caret_offset, sg_dialect)

    if _detect_setting_context(tokens, sql, caret_offset):
        return {
            "parts":              [],
            "caret_after_dot":    False,
            "targets":            TARGET_SETTING,
            "schema_filter":      "",
            "target_table":       "",
            "keyword_context":    0,
            "preceding_column":   None,
            "column_list_relation": "",
            "value_position":     False,
            "shared_columns":     False,
            "keyword_group":      "",
        }

    parts, caret_after_dot = _parse_qualified_parts(tokens)
    clause = _walk_to_clause(tokens)
    keyword_ctx = clause.target

    if _at_a_statement_start(tokens, caret_offset):
        # Nothing names a relation or a column yet, so the only thing that can
        # be written is the word the statement opens with.
        return {
            "parts":              [],
            "caret_after_dot":    False,
            "targets":            TARGET_KEYWORD,
            "schema_filter":      "",
            "target_table":       "",
            "keyword_context":    TARGET_KEYWORD,
            "preceding_column":   None,
            "column_list_relation": "",
            "value_position":     False,
            "shared_columns":     False,
            "keyword_group":      "statement",
        }

    keyword_group = "" if parts or caret_after_dot else _keyword_group_after(
        tokens, caret_offset, clause)
    targets = keyword_ctx
    schema_filter = ""
    target_table = ""

    if parts:
        targets, schema_filter, target_table = _determine_targets(
            parts, caret_after_dot, schema_names,
        )

    column_list_relation = ""
    preceding_column = None
    value_position = False
    shared_columns = False

    if not caret_after_dot and not parts:
        shared_columns = _in_a_join_using_list(tokens)
        column_list_relation = _detect_column_list(tokens)
        if column_list_relation:
            targets = TARGET_COLUMN
        else:
            preceding_column = _detect_preceding_column(tokens, caret_offset)
            if preceding_column:
                if keyword_ctx == TARGET_TABLE_AND_COLUMN:
                    targets = TARGET_OPERATOR
                elif keyword_ctx == TARGET_ALL:
                    targets = 0
            else:
                slot = _detect_value_position(tokens, caret_offset)
                if slot.column:
                    preceding_column = slot.column
                    value_position = True
                    targets = TARGET_ENUM_VALUE
                    # Only inside a literal is a value the one thing that fits.
                    # Anywhere else an expression does too, so the clause still
                    # says what else may be written.
                    if not slot.quoted:
                        targets |= keyword_ctx

    if keyword_group:
        # A finished item leaves the clause waiting for a word, not for another
        # name: "SELECT c1 " takes FROM, and offered nothing at all before.
        targets = TARGET_KEYWORD

    return {
        "parts":              parts,
        "caret_after_dot":    caret_after_dot,
        "targets":            targets,
        "schema_filter":      schema_filter,
        "target_table":       target_table,
        "keyword_context":    keyword_ctx,
        "preceding_column":   preceding_column,
        "column_list_relation": column_list_relation,
        "value_position":     value_position,
        "shared_columns":     shared_columns,
        "keyword_group":      keyword_group,
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


def _tokenize_up_to(sql: str, caret_offset: int, sg_dialect: str) -> list:
    return [t for t in tokenize(sql, sg_dialect) if t.start < caret_offset]


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

# (keyword_text, target). First match wins.
# SQLGlot combines compound keywords into single tokens.
_KEYWORD_MATCHERS: list[tuple[str, int]] = [
    ("ORDER BY",    TARGET_TABLE_AND_COLUMN),
    ("GROUP BY",    TARGET_TABLE_AND_COLUMN),
    ("INSERT INTO", TARGET_SCHEMA_AND_TABLE_ALL),
    ("AS",          0),
    ("AND",         TARGET_TABLE_AND_COLUMN),
    ("OR",          TARGET_TABLE_AND_COLUMN),
    ("HAVING",      TARGET_TABLE_AND_COLUMN),
    ("WHERE",       TARGET_TABLE_AND_COLUMN),
    ("FROM",        TARGET_SCHEMA_AND_TABLE_ALL),
    ("INNER",       TARGET_SCHEMA_AND_TABLE_ALL),
    ("LEFT",        TARGET_SCHEMA_AND_TABLE_ALL),
    ("RIGHT",       TARGET_SCHEMA_AND_TABLE_ALL),
    ("FULL",        TARGET_SCHEMA_AND_TABLE_ALL),
    ("CROSS",       TARGET_SCHEMA_AND_TABLE_ALL),
    ("NATURAL",     TARGET_SCHEMA_AND_TABLE_ALL),
    ("JOIN",        TARGET_SCHEMA_AND_TABLE_ALL),
    ("SET",         TARGET_COLUMN),
    # A row count takes an expression, so a column stands there. Relations are
    # listed only to qualify one, never to be selected from.
    ("LIMIT",       TARGET_TABLE_AND_COLUMN),
    ("OFFSET",      TARGET_TABLE_AND_COLUMN),
    ("VALUES",      TARGET_COLUMN),
    ("RETURNING",   TARGET_COLUMN),
    ("SELECT",      TARGET_ALL),
    ("UPDATE",      TARGET_SCHEMA_AND_TABLE_ALL),
]


# The tokens a nested query may follow. A paren after any of them opens one;
# a paren anywhere else is part of an expression.
#
# IN is not one of them. All three of a value, a column and a subquery are
# legal in an IN list, so the clause around it says more than "a query starts
# here" would.
_QUERY_OPENING_TOKENS = {
    TokenType.FROM, TokenType.JOIN, TokenType.EXISTS,
    TokenType.ALIAS, TokenType.UNION, TokenType.INTERSECT, TokenType.EXCEPT,
    TokenType.ANY, TokenType.ALL, TokenType.SOME,
    TokenType.EQ, TokenType.NEQ, TokenType.GT, TokenType.LT,
    TokenType.GTE, TokenType.LTE,
}


def _opens_query(tokens: list, paren_idx: int) -> bool:
    """Report the paren at paren_idx as opening a nested query rather than an
    expression. A CTE body may carry a materialization hint before its paren,
    which reads as a name and is not a call: only AS can precede one."""
    if paren_idx == 0:
        return True
    if _renames_a_relation(tokens, paren_idx):
        return False
    if tokens[paren_idx - 1].token_type in _QUERY_OPENING_TOKENS:
        # A window definition borrows the CTE's spelling, "WINDOW w AS (", and
        # what goes inside it is a partition and an order over this query.
        return not _names_a_window(tokens, paren_idx - 1)
    i = paren_idx - 1
    while i >= 0 and (_is_identifier_token(tokens[i]) or tokens[i].token_type == TokenType.NOT):
        i -= 1
    return i >= 0 and tokens[i].token_type == TokenType.ALIAS


# What a word is followed by when it names a column rather than opening a
# clause: an assignment, a comparison, or the dot of a qualified name.
_NAME_FOLLOWERS = {
    TokenType.EQ, TokenType.NEQ, TokenType.GT, TokenType.LT,
    TokenType.GTE, TokenType.LTE, TokenType.DOT,
}


# The words that begin a statement. The walk stops at one, because anything
# before it belongs to a statement the caret is not in.
_STATEMENT_WORDS = frozenset({
    "SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "GRANT", "REVOKE", "WITH",
})


# --- Join USING list detection ---

def _in_a_join_using_list(tokens: list) -> bool:
    """Report the caret as inside a join's USING list, where only a name every
    side carries is legal, and only unqualified.

    The clause the USING opens is asked for rather than decided again here: a
    merge's and a delete's USING are the same word and name a relation.
    """
    depth = 0
    for i in range(len(tokens) - 1, -1, -1):
        token_type = tokens[i].token_type
        if token_type == TokenType.R_PAREN:
            depth += 1
        elif token_type == TokenType.L_PAREN:
            if depth > 0:
                depth -= 1
                continue
            return (i > 0 and tokens[i - 1].token_type == TokenType.USING
                    and _contextual_target(tokens, i - 1, "USING") == TARGET_COLUMN)
    return False


def _contextual_target(tokens: list, idx: int, upper: str) -> int | None:
    """The target for a word the rest of the statement gives a meaning to, or
    None when this word is not one of those and the table can answer."""
    if upper == "AS":
        return TARGET_TABLE_AND_COLUMN if _names_a_window(tokens, idx) else None
    if upper == "UPDATE":
        # MySQL spells an upsert "ON DUPLICATE KEY UPDATE <column> = ...".
        if idx > 0 and tokens[idx - 1].text.upper() == "KEY":
            return TARGET_COLUMN
        return None
    if upper in ("FIRST", "NEXT"):
        # Either word is a row count after FETCH and a name anywhere else.
        if idx > 0 and tokens[idx - 1].token_type == TokenType.FETCH:
            return TARGET_TABLE_AND_COLUMN
        return None
    if upper not in ("ON", "USING"):
        return None
    if upper == "ON" and idx + 1 < len(tokens) \
            and tokens[idx + 1].text.upper() == "CONFLICT":
        return TARGET_COLUMN
    joined = _join_kind_before(tokens, idx)
    if upper == "USING":
        # A merge's USING names the relation it merges from, a join's names
        # the columns the two sides share.
        return TARGET_COLUMN if joined == TokenType.JOIN else TARGET_SCHEMA_AND_TABLE_ALL
    return TARGET_TABLE_AND_COLUMN if joined else TARGET_SCHEMA_AND_TABLE_ALL


def _join_kind_before(tokens: list, idx: int) -> TokenType | None:
    """The kind of thing that put two relations together before idx: a join, a
    merge, or None.

    A joined subquery holds a statement of its own, so what is inside a paren
    is skipped rather than read as this statement's words.
    """
    for i in _walk_back(tokens, idx):
        if tokens[i].token_type in (TokenType.JOIN, TokenType.MERGE):
            return tokens[i].token_type
    return None


def _names_a_window(tokens: list, alias_idx: int) -> bool:
    """Report the AS at alias_idx as naming a window rather than a CTE."""
    if tokens[alias_idx].token_type != TokenType.ALIAS:
        return False
    for i in _walk_back(tokens, alias_idx):
        if tokens[i].token_type == TokenType.WINDOW:
            return True
        if tokens[i].token_type in (TokenType.WITH, TokenType.FROM):
            return False
    return False


def _walk_back(tokens: list, idx: int):
    """The indexes before idx that belong to this statement and this nesting,
    nearest first, ending with the word the statement begins with.

    A parenthesised group is skipped whole, since a statement inside one is not
    the statement the caret is in. The opening word is yielded because it can
    be the answer: a merge is what puts two relations together in a MERGE.
    """
    depth = 0
    for i in range(idx - 1, -1, -1):
        token_type = tokens[i].token_type
        if token_type == TokenType.R_PAREN:
            depth += 1
            continue
        if token_type == TokenType.L_PAREN:
            if depth > 0:
                depth -= 1
            continue
        if depth > 0:
            continue
        if token_type == TokenType.SEMICOLON:
            return
        yield i
        if tokens[i].text.upper() in _STATEMENT_WORDS:
            return


def _reads_as_a_name(tokens: list, idx: int) -> bool:
    """Report the token at idx as a column name rather than a clause word.
    The tokenizer has no context, so "UPDATE t SET limit = 1" gives LIMIT the
    same token as a row count does, and the clause behind it would be lost."""
    return idx + 1 < len(tokens) and tokens[idx + 1].token_type in _NAME_FOLLOWERS


def _offers_new_relations(target: int) -> bool:
    """Whether the clause names a relation the query does not hold yet."""
    return bool(target & _ALL_FLAG)


def _narrowed_to_a_call(target: int, inside_call: bool) -> int:
    """The clause's target, less what a call's arguments cannot hold. A FROM
    names relations, but a call standing in one takes values, so a caret
    between its parens reads the columns rather than the catalogue."""
    if inside_call and _offers_new_relations(target):
        return TARGET_TABLE_AND_COLUMN
    return target


# What may be written once the current clause holds a finished item. The words
# are named here; which of them a dialect has is the dialect's list to answer.
_CLAUSE_FOLLOWERS = {
    "SELECT":      "select_item",
    "FROM":        "relation",
    "JOIN":        "joined_relation",
    "INNER":       "joined_relation",
    "LEFT":        "joined_relation",
    "RIGHT":       "joined_relation",
    "FULL":        "joined_relation",
    "CROSS":       "joined_relation",
    "NATURAL":     "joined_relation",
    "USING":       "relation",
    "ON":          "predicate",
    "WHERE":       "predicate",
    "HAVING":      "predicate",
    "AND":         "predicate",
    "OR":          "predicate",
    "ORDER BY":    "sort_item",
    "GROUP BY":    "group_item",
    "SET":         "assignment",
    "VALUES":      "values",
    "LIMIT":       "row_count",
    "OFFSET":      "row_count",
}

# The clauses whose finished item is the whole item, so a keyword follows it.
# In a WHERE a finished name is the left side of a predicate and an operator
# follows instead, which is why those clauses are not here.
_ITEM_IS_COMPLETE_AT_A_NAME = frozenset({
    "select_item", "relation", "joined_relation", "sort_item", "group_item",
})

# What stands between the two sides of a predicate.
_COMPARISONS = frozenset({
    TokenType.EQ, TokenType.NEQ, TokenType.GT, TokenType.LT,
    TokenType.GTE, TokenType.LTE, TokenType.IS, TokenType.IN,
    TokenType.LIKE, TokenType.ILIKE, TokenType.BETWEEN,
})

# What reads as a finished item: a name, a literal, a closing paren, a star.
_FINISHED_ITEM_TOKENS = frozenset({
    TokenType.NUMBER, TokenType.STRING, TokenType.R_PAREN, TokenType.STAR,
})


def _alias_index(tokens: list) -> int:
    """Where the AS nearest the caret stands, so the clause behind it can be
    asked what follows. A CTE's AS is followed by its whole body, which the
    walk steps over."""
    for i in _walk_back(tokens, len(tokens)):
        if tokens[i].token_type == TokenType.ALIAS:
            return i
    return len(tokens)


def _keyword_group_after(tokens: list, caret_offset: int, clause: Clause) -> str:
    """The group of words that may follow what the clause already holds, or ""
    when the caret is not standing after a finished item."""
    if not tokens:
        return ""
    if clause.word == "AS":
        alias_at = _alias_index(tokens)
        if _names_a_cte(tokens, alias_at):
            # The CTE is defined; what follows is the statement that reads it.
            group = "after_cte"
        else:
            # An alias belongs to the item it renames, so what may follow it is
            # what may follow that item.
            group = _CLAUSE_FOLLOWERS.get(_walk_to_clause(tokens[:alias_at]).word, "")
    else:
        group = _CLAUSE_FOLLOWERS.get(clause.word, "")
    if not group:
        return ""
    last = tokens[-1]
    if _caret_touches(last, caret_offset):
        # The word is being typed, so the item before it is what decides: the
        # first keystroke of AND must not take the answer back to columns.
        if not _is_identifier_token(last) or len(tokens) == 1:
            return ""
        tokens = tokens[:-1]
        last = tokens[-1]
    if last.token_type in _FINISHED_ITEM_TOKENS:
        return group
    if not _is_identifier_token(last):
        return ""
    if group in _ITEM_IS_COMPLETE_AT_A_NAME:
        return group
    # A name in a predicate is its left side, and an operator comes next,
    # unless one already stands between the clause and here.
    if group == "predicate" and _compared_since_the_clause(tokens):
        return group
    return ""


def _names_a_cte(tokens: list, alias_idx: int) -> bool:
    """Report the AS at alias_idx as defining a CTE."""
    if alias_idx >= len(tokens):
        return False
    for i in _walk_back(tokens, alias_idx):
        if tokens[i].token_type == TokenType.WITH:
            return True
        if tokens[i].token_type in (TokenType.FROM, TokenType.SELECT):
            return False
    return False


def _compared_since_the_clause(tokens: list) -> bool:
    """Whether a comparison already stands between the clause word and the
    caret, which is what makes the predicate whole rather than half written."""
    for i in _walk_back(tokens, len(tokens)):
        if tokens[i].token_type in _COMPARISONS:
            return True
        if tokens[i].text.upper() in _CLAUSE_FOLLOWERS:
            return False
    return False


def _at_a_statement_start(tokens: list, caret_offset: int) -> bool:
    """Whether the caret stands where a statement may begin: the buffer holds
    nothing yet, the last thing before it ended one, or the word it opens with
    is still being typed.

    The last of those is what a caller sees most: the first keystroke of
    SELECT must not turn the answer into every relation in the database.
    """
    if not tokens or tokens[-1].token_type == TokenType.SEMICOLON:
        return True
    written = _since_the_last_statement(tokens)
    return (len(written) == 1 and _is_identifier_token(written[0])
            and _caret_touches(written[0], caret_offset))


def _since_the_last_statement(tokens: list) -> list:
    """The tokens of the statement the caret is in."""
    for i in range(len(tokens) - 1, -1, -1):
        if tokens[i].token_type == TokenType.SEMICOLON:
            return tokens[i + 1:]
    return tokens


class Clause(NamedTuple):
    """The clause the caret stands in: what may be written, and the word that
    opened it. The word is "" where the walk found none and defaulted."""

    target: int
    word: str


def _detect_keyword_context(tokens: list) -> int:
    return _walk_to_clause(tokens).target


def _walk_to_clause(tokens: list) -> Clause:
    if not tokens:
        return Clause(TARGET_SCHEMA_AND_TABLE_ALL, "")

    paren_depth = 0
    inside_call = False
    i = len(tokens) - 1

    if i >= 0 and tokens[i].token_type == TokenType.DOT:
        i -= 1

    while i >= 0:
        tok = tokens[i]
        tt = tok.token_type
        upper = tok.text.upper()

        if tt == TokenType.R_PAREN:
            paren_depth += 1
            i -= 1
            continue

        if tt == TokenType.L_PAREN:
            if paren_depth > 0:
                paren_depth -= 1
                i -= 1
                continue
            if _opens_query(tokens, i):
                return Clause(TARGET_SCHEMA_AND_TABLE_ALL, "")
            # An expression's paren -- a call's arguments, a window spec, a
            # grouping -- writes what the clause around it writes, so the walk
            # continues rather than treating the paren as a new query. Only a
            # call's paren narrows that clause, since a relation standing in a
            # FROM is still a relation when a comma or LATERAL precedes it.
            inside_call = inside_call or _is_identifier_token(tokens[i - 1])
            i -= 1
            continue

        if tt == TokenType.SEMICOLON:
            break

        if paren_depth > 0:
            i -= 1
            continue

        if not _reads_as_a_name(tokens, i):
            contextual = _contextual_target(tokens, i, upper)
            if contextual is not None:
                return Clause(_narrowed_to_a_call(contextual, inside_call), upper)
            for kw_text, target in _KEYWORD_MATCHERS:
                if upper == kw_text:
                    return Clause(_narrowed_to_a_call(target, inside_call), upper)

        if upper in _STATEMENT_WORDS and upper != "WITH":
            break

        i -= 1

    return Clause(TARGET_SCHEMA_AND_TABLE_ALL, "")


# --- Column list detection ---

# What a relation may be written after. A name in one of these positions is a
# relation the statement reads, so a column list attached to it renames that
# relation rather than opening a query.
_RELATION_POSITIONS = frozenset({
    TokenType.FROM, TokenType.JOIN, TokenType.USING,
    TokenType.INTO, TokenType.UPDATE, TokenType.TABLE,
})

# The clauses a comma separates relations in, against the ones it separates
# definitions in: "FROM a, b" lists relations where "WITH a AS (), b AS ()"
# and a WINDOW clause list names being declared.
_RELATION_LISTS = frozenset({TokenType.FROM, TokenType.JOIN, TokenType.USING})
_DEFINITION_LISTS = frozenset({TokenType.WITH, TokenType.WINDOW})


def _renames_a_relation(tokens: list, paren_idx: int) -> bool:
    """Whether the paren at paren_idx opens a list renaming a relation's
    columns, as in "FROM t AS a (" or "FROM (SELECT 1) s (".

    Asked apart from which relation is renamed, because a derived table has no
    name to offer and is still not a query body.
    """
    return _relation_renamed_at(tokens, paren_idx) is not None


def _renamed_relation(tokens: list, paren_idx: int) -> str:
    """The relation such a list renames, or "" when it has no name to give:
    the names being written are new, so the ones they replace are what helps,
    and a derived table has none."""
    return _relation_renamed_at(tokens, paren_idx) or ""


def _relation_renamed_at(tokens: list, paren_idx: int) -> str | None:
    """The name of the relation the list renames, "" when the relation has no
    name, or None when this paren is not such a list."""
    i = paren_idx - 1
    if i >= 0 and _is_identifier_token(tokens[i]):
        i -= 1
    if i >= 0 and tokens[i].token_type == TokenType.ALIAS:
        i -= 1
    if i < 0:
        return None

    # A derived table or a call is the relation itself, and its closing paren
    # is what stands here.
    if tokens[i].token_type == TokenType.R_PAREN:
        return "" if _relation_precedes(tokens, _opening_paren(tokens, i)) else None

    if not _is_identifier_token(tokens[i]):
        return None
    start = _qualified_start(tokens, i)
    if not _relation_precedes(tokens, start):
        return None
    return _normalize_identifier(tokens[i].text)


def _qualified_start(tokens: list, name_idx: int) -> int:
    """The index where a qualified name begins, so "main.t1" is asked about at
    "main" rather than at the dot before "t1"."""
    i = name_idx
    while i >= 2 and tokens[i - 1].token_type == TokenType.DOT \
            and _is_identifier_token(tokens[i - 2]):
        i -= 2
    return i


def _opening_paren(tokens: list, close_idx: int) -> int:
    """The index of the paren the one at close_idx closes, or -1."""
    depth = 0
    for i in range(close_idx, -1, -1):
        if tokens[i].token_type == TokenType.R_PAREN:
            depth += 1
        elif tokens[i].token_type == TokenType.L_PAREN:
            depth -= 1
            if depth == 0:
                return i
    return -1


def _lists_relations(tokens: list, comma_idx: int) -> bool:
    """Whether the comma at comma_idx separates relations rather than the
    definitions of a WITH or a WINDOW clause."""
    for i in _walk_back(tokens, comma_idx):
        if tokens[i].token_type in _RELATION_LISTS:
            return True
        if tokens[i].token_type in _DEFINITION_LISTS:
            return False
    return False


def _relation_precedes(tokens: list, idx: int) -> bool:
    """Whether what starts at idx stands where a relation may be written. A
    call in a FROM is a relation too, so the name before its paren counts."""
    if idx <= 0:
        return False
    before = tokens[idx - 1]
    if before.token_type in _RELATION_POSITIONS:
        return True
    if before.token_type == TokenType.COMMA:
        return _lists_relations(tokens, idx - 1)
    # "FROM generate_series(1, 2) g (" names the call, not the clause.
    return _is_identifier_token(before) and idx >= 2 \
        and tokens[idx - 2].token_type in _RELATION_POSITIONS


def _detect_column_list(tokens: list) -> str:
    """The relation whose columns a parenthesised list of names belongs to, or
    "" when the caret is not in one or the relation has no name."""
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
                return _renamed_relation(tokens, i)
        elif tt == TokenType.SEMICOLON:
            return ""
        elif upper == "VALUES":
            return ""
        elif paren_depth == 0 and upper in ("SELECT", "UPDATE", "DELETE", "INSERT"):
            return ""

        i -= 1

    return ""


# --- Preceding column detection ---

def _caret_touches(tok, caret_offset: int) -> bool:
    """Report the caret as still on this token, so what it holds is being
    typed. end is the last character's index, so the position after the token
    is end + 1, and a caret there is still on it."""
    return tok.end + 1 >= caret_offset


def _detect_preceding_column(tokens: list, caret_offset: int) -> dict | None:
    """Return the column ref if the last token is a finished identifier, else None."""
    if not tokens:
        return None

    last = tokens[-1]
    if not _is_identifier_token(last):
        return None

    if _caret_touches(last, caret_offset):
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


class ValueSlot(NamedTuple):
    """Where a value may be written: col = '|', col IN ('|'), col = |."""

    column: dict | None
    quoted: bool


_NO_VALUE_SLOT = ValueSlot(None, False)


def _detect_value_position(tokens: list, caret_offset: int) -> ValueSlot:
    if not tokens:
        return _NO_VALUE_SLOT

    i = len(tokens) - 1
    quoted = False

    last = tokens[i]
    if last.token_type == TokenType.STRING:
        if not (last.start < caret_offset and _caret_touches(last, caret_offset)):
            return _NO_VALUE_SLOT
        quoted = True
        i -= 1
    elif _is_identifier_token(last) and _caret_touches(last, caret_offset):
        # A word the caret is still inside is being typed, not a value already
        # written, so the slot is the one before it. Without this the first
        # keystroke of a value loses the column's values.
        i -= 1

    if i < 0:
        return _NO_VALUE_SLOT

    if tokens[i].token_type in (TokenType.EQ, TokenType.NEQ):
        return ValueSlot(_column_ref_at(tokens, i - 1), quoted)

    j = i
    while j >= 0 and tokens[j].token_type in (TokenType.STRING, TokenType.COMMA):
        j -= 1
    if j >= 0 and tokens[j].token_type == TokenType.L_PAREN \
            and j >= 1 and tokens[j - 1].token_type == TokenType.IN:
        return ValueSlot(_column_ref_at(tokens, j - 2), quoted)

    return _NO_VALUE_SLOT


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
