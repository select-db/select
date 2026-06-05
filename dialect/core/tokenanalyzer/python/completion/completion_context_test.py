"""Tests for _detect_setting_context in completion_context.py."""
from sqlglot.tokens import Tokenizer

from completion.completion_context import _detect_setting_context


def _check(sql_with_caret: str) -> bool:
    caret = sql_with_caret.index("|")
    sql = sql_with_caret[:caret] + sql_with_caret[caret + 1:]
    tokens = [t for t in Tokenizer().tokenize(sql) if t.start < caret]
    return _detect_setting_context(tokens, sql, caret)


class TestMySQLSystemVariable:
    def test_double_at_alone(self):
        assert _check("SELECT @@|")

    def test_double_at_partial(self):
        assert _check("SELECT @@vers|")

    def test_single_at_user_variable_does_not_trigger(self):
        assert not _check("SELECT @my|")


class TestPostgresShow:
    def test_show_alone(self):
        assert _check("SHOW |")

    def test_show_partial(self):
        assert _check("SHOW time|")

    def test_show_lowercase(self):
        assert _check("show |")


class TestSqlitePragma:
    def test_pragma_alone(self):
        assert _check("PRAGMA |")

    def test_pragma_partial(self):
        assert _check("PRAGMA fore|")

    def test_pragma_lowercase(self):
        assert _check("pragma |")


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
