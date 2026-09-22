"""The caret patch writes a name into the SQL so sqlglot can parse an
incomplete statement. That name must never reach the caller as something to
complete: a table or column that does not exist reads as one that does.

These go through the dispatcher rather than a collector, because that is where
the response is cleaned and where a handler added later is covered.
"""
import server
from completion.caret_patch import PLACEHOLDER, without_placeholders

SCHEMA = {
    "schemas": [
        {
            "name": "main",
            "tables": [
                {"name": "t1", "columns": [{"name": "c1", "type": "INTEGER"},
                                           {"name": "c2", "type": "TEXT"}]},
                {"name": "t2", "columns": [{"name": "c1", "type": "INTEGER"},
                                           {"name": "c3", "type": "TEXT"}]},
            ],
        },
    ],
}


def _response(action: str, sql_with_caret: str, sg_dialect: str = "postgresql") -> dict:
    caret = sql_with_caret.index("|")
    return server._dispatch({
        "action": action,
        "sql": sql_with_caret[:caret] + sql_with_caret[caret + 1:],
        "caret_line": 1,
        "caret_col": caret,
        "dialect": sg_dialect,
        "schema": SCHEMA,
        "default_schema": "main",
    })


def _carries_placeholder(value) -> bool:
    if isinstance(value, dict):
        return any(_carries_placeholder(item) for item in value.values())
    if isinstance(value, list):
        return any(_carries_placeholder(item) for item in value)
    return value == PLACEHOLDER


class TestPlaceholderIsNotAReference:
    def test_a_subquery_in_from(self):
        assert not _carries_placeholder(_response("collect_references", "SELECT * FROM (|"))

    def test_a_trailing_comma_in_the_select_list(self):
        assert not _carries_placeholder(
            _response("collect_column_refs", "SELECT c1, | FROM t1"))

    def test_a_trailing_dot(self):
        assert not _carries_placeholder(
            _response("collect_column_refs", "SELECT t1.|, c2 FROM t1"))

    def test_a_call_with_no_argument_yet(self):
        assert not _carries_placeholder(
            _response("collect_column_refs", "SELECT count(|) FROM t1"))

    def test_a_column_of_a_subquery(self):
        assert not _carries_placeholder(
            _response("collect_references", "SELECT * FROM (SELECT c1, |"))

    def test_every_dialect(self):
        for sg_dialect in ("postgresql", "mysql", "sqlite"):
            assert not _carries_placeholder(
                _response("collect_column_refs", "SELECT * FROM t1 WHERE c1 IN (|", sg_dialect))


class TestWithoutPlaceholders:
    def test_it_reaches_a_nested_list(self):
        response = {"virtual_tables": [{"name": "x", "columns": [
            {"name": "c1"}, {"name": PLACEHOLDER}]}]}
        assert without_placeholders(response) == {
            "virtual_tables": [{"name": "x", "columns": [{"name": "c1"}]}]}

    def test_it_keeps_what_the_caller_wrote(self):
        response = {"relations": [{"table": "t1", "alias": ""}]}
        assert without_placeholders(response) == response
