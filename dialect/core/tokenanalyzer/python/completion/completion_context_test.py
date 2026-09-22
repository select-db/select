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


def _at_caret(sql_with_caret: str, sg_dialect: str = "postgres"):
    """Tokenize the way production does, so these cases exercise the tokenizer
    the caller actually gets rather than sqlglot's generic one.
    """
    caret = sql_with_caret.index("|")
    sql = sql_with_caret[:caret] + sql_with_caret[caret + 1:]
    return _tokenize_up_to(sql, caret, sg_dialect), sql, caret


def _check(sql_with_caret: str, sg_dialect: str = "postgres") -> bool:
    tokens, sql, caret = _at_caret(sql_with_caret, sg_dialect)
    return _detect_setting_context(tokens, sql, caret)


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


class TestCallParen:
    def test_inside_a_call_keeps_the_select_clause(self):
        tokens, _, _ = _at_caret("SELECT count(|) FROM t1")
        assert _detect_keyword_context(tokens) == TARGET_ALL

    def test_inside_a_call_in_where_keeps_the_where_clause(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE lower(|)")
        assert _detect_keyword_context(tokens) & TARGET_COLUMN

    def test_a_paren_a_keyword_opens_is_still_a_nested_query(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE c1 IN (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_word_of_the_syntax_before_a_paren_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS MATERIALIZED (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_not_materialized_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS NOT MATERIALIZED (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_materialization_hint_touching_its_paren_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS MATERIALIZED(|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_not_materialized_touching_its_paren_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS NOT MATERIALIZED(|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_call_with_a_space_before_its_paren_is_still_a_call(self):
        tokens, _, _ = _at_caret("SELECT count (|) FROM t1")
        assert _detect_keyword_context(tokens) == TARGET_ALL

    def test_a_plain_cte_body_is_a_nested_query(self):
        tokens, _, _ = _at_caret("WITH x AS (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL


class TestValuePosition:
    def test_inside_a_literal_is_quoted(self):
        tokens, _, caret = _at_caret("UPDATE t SET c1 = '|'")
        assert _detect_value_position(tokens, caret).quoted

    def test_after_an_equals_is_not_quoted(self):
        tokens, _, caret = _at_caret("UPDATE t SET c1 = |")
        assert not _detect_value_position(tokens, caret).quoted

    def test_inside_an_in_list_is_quoted(self):
        tokens, _, caret = _at_caret("SELECT * FROM t WHERE c1 IN ('|')")
        assert _detect_value_position(tokens, caret).quoted

    def test_an_in_list_says_so(self):
        tokens, _, caret = _at_caret("SELECT * FROM t WHERE c1 IN (|")
        assert _detect_value_position(tokens, caret).in_list

    def test_a_plain_slot_does_not(self):
        tokens, _, caret = _at_caret("UPDATE t SET c1 = |")
        assert not _detect_value_position(tokens, caret).in_list


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
