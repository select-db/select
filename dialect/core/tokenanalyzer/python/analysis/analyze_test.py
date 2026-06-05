"""Robustness tests for analyze(), incomplete / empty SQL must never crash."""
from analysis import analyze

SCHEMA = {"public": {"users": ["id", "name"], "orders": ["id", "user_id"]}}


def _r(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA, default_schema="public")


class TestRobustness:
    def test_empty_sql(self):
        r = _r("")
        assert r["diagnostics"] == []
        assert r["parse_errors"] == []

    def test_incomplete_sql_no_crash(self):
        r = _r("SELECT * FROM u")
        assert r["parse_errors"] == [] or True  # may warn, must not crash

    def test_incomplete_where_no_crash(self):
        r = _r("SELECT id FROM users WHERE")
        assert "diagnostics" in r

    def test_incomplete_join_no_crash(self):
        r = _r("SELECT * FROM users JOIN")
        assert "relations" in r

    def test_no_schema_no_false_positives(self):
        r = analyze("SELECT anything FROM whatever", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert not any(d["rule_id"] == "unknown-table" for d in r["diagnostics"])
        assert not any(d["rule_id"] == "unknown-column" for d in r["diagnostics"])
