"""Tests for the caret classification in completion_context.py."""
from completion.completion_context import (
    TARGET_ALL,
    TARGET_COLUMN,
    TARGET_SCHEMA_AND_TABLE_ALL,
    _detect_keyword_context,
    _detect_setting_context,
    _detect_value_position,
    _tokenize_up_to,
)


def _check(sql_with_caret: str, sg_dialect: str = "postgres") -> bool:
    """Tokenize the way production does, so these cases exercise the tokenizer
    the caller actually gets rather than sqlglot's generic one.
    """
    caret = sql_with_caret.index("|")
    sql = sql_with_caret[:caret] + sql_with_caret[caret + 1:]
    return _detect_setting_context(_tokenize_up_to(sql, caret, sg_dialect), sql, caret)


class TestMySQLSystemVariable:
    def test_double_at_alone(self):
        assert _check("SELECT @@|", "mysql")

    def test_double_at_partial(self):
        assert _check("SELECT @@vers|", "mysql")

    def test_single_at_user_variable_does_not_trigger(self):
        assert not _check("SELECT @my|", "mysql")


class TestPostgresShow:
    def test_show_alone(self):
        assert _check("SHOW |")

    def test_show_partial(self):
        assert _check("SHOW time|")

    def test_show_lowercase(self):
        assert _check("show |")


class TestSqlitePragma:
    def test_pragma_alone(self):
        assert _check("PRAGMA |", "sqlite")

    def test_pragma_partial(self):
        assert _check("PRAGMA fore|")

    def test_pragma_lowercase(self):
        assert _check("pragma |")


def _tokens(sql_with_caret: str, sg_dialect: str = "postgres"):
    caret = sql_with_caret.index("|")
    sql = sql_with_caret[:caret] + sql_with_caret[caret + 1:]
    return _tokenize_up_to(sql, caret, sg_dialect), caret


class TestCallParen:
    """A paren an identifier opens is a call, and a call is part of its
    clause: what may be written inside it is what may be written around it.
    """

    def test_inside_a_call_keeps_the_select_clause(self):
        tokens, _ = _tokens("SELECT count(|) FROM t1")
        assert _detect_keyword_context(tokens) == TARGET_ALL

    def test_inside_a_call_in_where_keeps_the_where_clause(self):
        tokens, _ = _tokens("SELECT * FROM t1 WHERE lower(|)")
        assert _detect_keyword_context(tokens) & TARGET_COLUMN

    def test_a_paren_a_keyword_opens_is_still_a_nested_query(self):
        tokens, _ = _tokens("SELECT * FROM t1 WHERE c1 IN (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL


class TestValuePosition:
    """Inside a literal only a value fits. Outside one an expression does too,
    so the caret keeps what its clause offers.
    """

    def test_inside_a_literal_is_quoted(self):
        tokens, caret = _tokens("UPDATE t SET c1 = '|'")
        assert _detect_value_position(tokens, caret)["quoted"]

    def test_after_an_equals_is_not_quoted(self):
        tokens, caret = _tokens("UPDATE t SET c1 = |")
        assert not _detect_value_position(tokens, caret)["quoted"]

    def test_inside_an_in_list_is_quoted(self):
        tokens, caret = _tokens("SELECT * FROM t WHERE c1 IN ('|')")
        assert _detect_value_position(tokens, caret)["quoted"]


class TestNonTriggers:
    def test_select_keyword(self):
        assert not _check("SELECT |")

    def test_from_clause(self):
        assert not _check("SELECT * FROM |")

    def test_where_clause(self):
        assert not _check("SELECT * FROM t WHERE |")

    def test_update_set_does_not_trigger(self):
        assert not _check("UPDATE t SET |")

    def test_empty_sql(self):
        assert not _check("|")
