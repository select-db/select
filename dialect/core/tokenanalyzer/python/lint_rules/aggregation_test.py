"""Tests for aggregation.py, A001, A002, A003."""
from analysis import analyze

# Schema with aggregate functions listed as user-defined (Kind="a")
AGG_SCHEMA = {
    "public": {
        "t": {"id": "integer", "x": "integer", "score": "double precision"},
        "s": {"id": "integer", "y": "integer"},
    }
}

AGG_FUNCTIONS = ["count", "sum", "avg", "min", "max", "array_agg", "string_agg"]


def _r(sql, functions=None):
    return analyze(
        sql,
        dialect="postgresql",
        schema_dict=AGG_SCHEMA,
        default_schema="public",
        functions=functions or AGG_FUNCTIONS,
    )


def _r_no_meta(sql):
    return analyze(sql, dialect="postgresql", schema_dict={}, default_schema="public")


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


# ---------------------------------------------------------------------------
# A001, Aggregate in WHERE
# ---------------------------------------------------------------------------

class TestA001AggInWhere:
    def test_triggers_count_in_where(self):
        assert len(_diags(_r("SELECT * FROM t WHERE count(*) > 0"), "agg-in-where")) >= 1

    def test_triggers_sum_in_where(self):
        assert len(_diags(_r("SELECT * FROM t WHERE sum(x) > 10"), "agg-in-where")) >= 1

    def test_no_trigger_plain_predicate(self):
        assert _diags(_r("SELECT * FROM t WHERE x > 5"), "agg-in-where") == []

    def test_no_trigger_agg_in_subquery_in_where(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (SELECT count(*) FROM s)"), "agg-in-where") == []

    def test_no_trigger_agg_in_having(self):
        assert _diags(_r("SELECT x FROM t GROUP BY x HAVING count(*) > 1"), "agg-in-where") == []

    def test_no_trigger_agg_in_select(self):
        assert _diags(_r("SELECT count(*) FROM t"), "agg-in-where") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t WHERE count(*) > 0"), "agg-in-where")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# A002, HAVING without GROUP BY
# ---------------------------------------------------------------------------

class TestA002HavingWithoutGroupBy:
    def test_triggers_having_no_group(self):
        assert len(_diags(_r("SELECT x FROM t HAVING count(*) > 0"), "having-without-group-by")) == 1

    def test_no_trigger_having_with_group(self):
        assert _diags(_r("SELECT x FROM t GROUP BY x HAVING count(*) > 1"), "having-without-group-by") == []

    def test_no_trigger_plain_where(self):
        assert _diags(_r("SELECT * FROM t WHERE x > 5"), "having-without-group-by") == []

    def test_subquery_having_no_group_triggers(self):
        # A002 fires inside the subquery too
        r = _r("SELECT * FROM t WHERE x IN (SELECT y FROM s HAVING count(*) > 1)")
        assert len(_diags(r, "having-without-group-by")) >= 1

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT x FROM t HAVING count(*) > 0"), "having-without-group-by")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# A003, Aggregate in JOIN ON
# ---------------------------------------------------------------------------

class TestA003AggInJoinOn:
    def test_triggers_agg_in_join_on(self):
        assert len(_diags(_r("SELECT * FROM t JOIN s ON count(t.id) = s.id"), "agg-in-join")) >= 1

    def test_triggers_sum_in_join_on(self):
        assert len(_diags(_r("SELECT * FROM t JOIN s ON t.id = sum(s.x)"), "agg-in-join")) >= 1

    def test_no_trigger_plain_column_join(self):
        assert _diags(_r("SELECT * FROM t JOIN s ON t.id = s.id"), "agg-in-join") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t JOIN s ON count(t.id) = s.id"), "agg-in-join")
        assert diags[0]["severity"] == "error"
