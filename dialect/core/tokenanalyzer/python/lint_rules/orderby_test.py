"""Tests for orderby.py, O001, O002, O003."""
from analysis import analyze


def _r(sql):
    return analyze(sql, dialect="postgresql", schema_dict={}, default_schema="public")


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


# ---------------------------------------------------------------------------
# O001, OFFSET without LIMIT
# ---------------------------------------------------------------------------

class TestO001OffsetWithoutLimit:
    def test_triggers_offset_no_limit(self):
        assert len(_diags(_r("SELECT * FROM t ORDER BY id OFFSET 10"), "offset-without-limit")) == 1

    def test_no_trigger_limit_with_offset(self):
        assert _diags(_r("SELECT * FROM t ORDER BY id LIMIT 20 OFFSET 10"), "offset-without-limit") == []

    def test_no_trigger_limit_only(self):
        assert _diags(_r("SELECT * FROM t ORDER BY id LIMIT 10"), "offset-without-limit") == []

    def test_no_trigger_neither(self):
        assert _diags(_r("SELECT * FROM t WHERE x > 5"), "offset-without-limit") == []

    def test_subquery_offset_no_limit_triggers(self):
        assert len(_diags(_r("SELECT * FROM (SELECT id FROM t OFFSET 5) sub"), "offset-without-limit")) == 1

    def test_subquery_offset_with_limit_no_trigger(self):
        assert _diags(_r("SELECT * FROM (SELECT id FROM t LIMIT 10 OFFSET 5) sub"), "offset-without-limit") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t ORDER BY id OFFSET 10"), "offset-without-limit")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# O002, LIMIT without ORDER BY
# ---------------------------------------------------------------------------

class TestO002LimitWithoutOrderBy:
    def test_triggers_limit_no_order(self):
        assert len(_diags(_r("SELECT * FROM t LIMIT 10"), "limit-without-order-by")) == 1

    def test_no_trigger_order_with_limit(self):
        assert _diags(_r("SELECT * FROM t ORDER BY id LIMIT 10"), "limit-without-order-by") == []

    def test_no_trigger_no_limit(self):
        assert _diags(_r("SELECT * FROM t WHERE x > 5"), "limit-without-order-by") == []

    def test_subquery_limit_no_order_triggers(self):
        assert len(_diags(_r("SELECT * FROM (SELECT id FROM t LIMIT 5) sub"), "limit-without-order-by")) == 1

    def test_subquery_limit_with_order_no_trigger(self):
        assert _diags(_r("SELECT * FROM (SELECT id FROM t ORDER BY id LIMIT 5) sub"), "limit-without-order-by") == []

    def test_severity_is_hint(self):
        diags = _diags(_r("SELECT * FROM t LIMIT 10"), "limit-without-order-by")
        assert diags[0]["severity"] == "hint"


# ---------------------------------------------------------------------------
# O003, ORDER BY in subquery without LIMIT
# ---------------------------------------------------------------------------

class TestO003OrderByInSubquery:
    def test_triggers_order_in_subquery_no_limit(self):
        assert len(_diags(_r("SELECT * FROM (SELECT a FROM t ORDER BY a) sub"), "subquery-order-by")) == 1

    def test_no_trigger_limit_in_subquery(self):
        assert _diags(_r("SELECT * FROM (SELECT a FROM t ORDER BY a LIMIT 10) sub"), "subquery-order-by") == []

    def test_no_trigger_top_level_order(self):
        assert _diags(_r("SELECT a FROM t ORDER BY a"), "subquery-order-by") == []

    def test_triggers_in_subquery_in_where(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x IN (SELECT id FROM s ORDER BY id)"), "subquery-order-by")) == 1

    def test_no_trigger_subquery_with_limit(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (SELECT id FROM s ORDER BY id LIMIT 5)"), "subquery-order-by") == []

    def test_severity_is_hint(self):
        diags = _diags(_r("SELECT * FROM (SELECT a FROM t ORDER BY a) sub"), "subquery-order-by")
        assert diags[0]["severity"] == "hint"
