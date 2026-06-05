"""Tests for functions.py, R009 unknown function."""
from analysis import analyze

SCHEMA = {
    "public": {
        "users":  {"id": "integer", "name": "character varying", "score": "double precision"},
        "orders": {"id": "integer", "user_id": "integer"},
    }
}


def _r(sql, functions=None):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA, default_schema="public",
                   functions=functions or [])


def _diags(r, rule_id="unknown-function"):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


class TestR009UnknownFunction:
    def test_builtin_function_no_trigger(self):
        assert _diags(_r("SELECT COUNT(*) FROM users")) == []

    def test_known_user_function_no_trigger(self):
        assert _diags(_r("SELECT my_fn(id) FROM users", functions=["my_fn"])) == []

    def test_unknown_function_triggers(self):
        diags = _diags(_r("SELECT totally_unknown_fn(id) FROM users"))
        assert len(diags) >= 1
        assert any("totally_unknown_fn" in d["message"].lower() for d in diags)

    def test_unknown_function_not_in_user_list_triggers(self):
        diags = _diags(_r("SELECT my_fn(id) FROM users", functions=["other_fn"]))
        assert any("my_fn" in d["message"].lower() for d in diags)

    def test_user_function_case_insensitive(self):
        assert _diags(_r("SELECT MY_FN(id) FROM users", functions=["my_fn"])) == []

    def test_no_false_positive_coalesce(self):
        assert _diags(_r("SELECT COALESCE(id, 0) FROM users")) == []

    def test_no_false_positive_now(self):
        assert _diags(_r("SELECT NOW()")) == []

    def test_position_reported(self):
        diags = _diags(_r("SELECT totally_unknown_fn(id) FROM users"))
        assert diags[0]["start_line"] >= 1
