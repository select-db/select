"""The caret patch writes a name into the SQL so sqlglot can parse an
incomplete statement. That name must never reach the caller as something to
complete: a table or column that does not exist reads as one that does.

The cases that need a collector go through the dispatcher, because that is
where a request is patched and its response cleaned.
"""
import server
from completion.caret_patch import (
    PLACEHOLDER,
    reduce_to_clause,
    unwrap_explain,
    without_placeholders,
)

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


class TestUnwrapExplain:
    def test_it_blanks_the_word(self):
        assert unwrap_explain("EXPLAIN SELECT 1") == "        SELECT 1"

    def test_it_blanks_the_options(self):
        for sql in ("EXPLAIN ANALYZE SELECT 1",
                    "EXPLAIN (ANALYZE, VERBOSE) SELECT 1",
                    "EXPLAIN QUERY PLAN SELECT 1"):
            assert unwrap_explain(sql).strip() == "SELECT 1"

    def test_it_keeps_every_offset(self):
        sql = "EXPLAIN ANALYZE\nSELECT 1"
        patched = unwrap_explain(sql)
        assert len(patched) == len(sql)
        assert patched.count("\n") == sql.count("\n")

    def test_it_leaves_a_column_named_explain_alone(self):
        assert unwrap_explain("SELECT explain FROM t") == "SELECT explain FROM t"

    def test_it_leaves_a_commented_explain_alone(self):
        sql = "SELECT 1 -- EXPLAIN SELECT 2"
        assert unwrap_explain(sql) == sql

    def test_it_reaches_a_later_statement(self):
        assert unwrap_explain("SELECT 1; EXPLAIN SELECT 2").endswith("SELECT 2")
        assert "EXPLAIN" not in unwrap_explain("SELECT 1; EXPLAIN SELECT 2")

    def test_it_reaches_past_a_leading_comment(self):
        assert "EXPLAIN" not in unwrap_explain("-- c\nEXPLAIN SELECT 1")

    def test_it_blanks_the_dialects_own_spellings(self):
        for sql in ("EXPLAIN FORMAT=JSON SELECT 1", "EXPLAIN EXTENDED SELECT 1"):
            assert unwrap_explain(sql).strip() == "SELECT 1"


class TestReduceToClause:
    def test_the_unfinished_item_goes_and_the_clauses_stay(self):
        sql = "SELECT\n  c.\n  c.c2\nFROM\n  t1 c"
        reduced = reduce_to_clause(sql, 2, 4)
        assert "c.c2" not in reduced
        assert "FROM" in reduced and "t1 c" in reduced

    def test_offsets_do_not_move(self):
        sql = "SELECT\n  c.\n  c.c2\nFROM\n  t1 c"
        assert len(reduce_to_clause(sql, 2, 4)) == len(sql)
        assert reduce_to_clause(sql, 2, 4).count("\n") == sql.count("\n")

    def test_a_closing_paren_stops_it(self):
        sql = "SELECT * FROM t1 WHERE c1 IN (SELECT a. b FROM t2 a) AND c2 = 1"
        reduced = reduce_to_clause(sql, 1, 39)
        assert reduced.endswith("AND c2 = 1")
        assert ")" in reduced
