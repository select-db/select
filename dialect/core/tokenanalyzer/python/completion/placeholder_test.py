"""The caret patch writes a name into the SQL so sqlglot can parse an
incomplete statement. That name must never reach the caller as something to
complete: a table or column that does not exist reads as one that does."""
import server

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


def _request(sql_with_caret: str, sg_dialect: str = "postgresql") -> dict:
    caret = sql_with_caret.index("|")
    return {
        "sql": sql_with_caret[:caret] + sql_with_caret[caret + 1:],
        "caret_line": 1,
        "caret_col": caret,
        "dialect": sg_dialect,
        "schema": SCHEMA,
        "default_schema": "main",
    }


# Spelled here rather than imported, so these cases fail on a leak rather than
# on a missing name when the filter is not there at all.
_NAME_FIELDS = ("table", "column", "name", "alias")
_PLACEHOLDER = "__placeholder__"


def _names(refs: dict) -> set[str]:
    found = set()
    for items in refs.values():
        for item in items:
            found.update(str(item.get(field, "")) for field in _NAME_FIELDS)
    return found


class TestPlaceholderIsNotAReference:
    def test_a_subquery_in_from(self):
        assert _PLACEHOLDER not in _names(server._collect_references(_request("SELECT * FROM (|")))

    def test_a_trailing_comma_in_the_select_list(self):
        assert _PLACEHOLDER not in _names(
            server._collect_column_refs(_request("SELECT c1, | FROM t1")))

    def test_a_trailing_dot(self):
        assert _PLACEHOLDER not in _names(
            server._collect_column_refs(_request("SELECT t1.|, c2 FROM t1")))

    def test_a_call_with_no_argument_yet(self):
        assert _PLACEHOLDER not in _names(
            server._collect_column_refs(_request("SELECT count(|) FROM t1")))

    def test_every_dialect(self):
        for sg_dialect in ("postgresql", "mysql", "sqlite"):
            request = _request("SELECT * FROM t1 WHERE c1 IN (|", sg_dialect)
            assert _PLACEHOLDER not in _names(server._collect_column_refs(request))
