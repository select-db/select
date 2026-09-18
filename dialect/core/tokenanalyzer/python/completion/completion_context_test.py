"""Tests for _detect_setting_context in completion_context.py."""
from completion.completion_context import _detect_setting_context, _tokenize_up_to


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
