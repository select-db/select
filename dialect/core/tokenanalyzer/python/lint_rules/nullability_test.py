"""Tests for nullability.py, N001, N002, N003, N004."""
from analysis import analyze


def _r(sql):
    return analyze(sql, dialect="postgresql", schema_dict={}, default_schema="public")


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


# ---------------------------------------------------------------------------
# N001, = NULL
# ---------------------------------------------------------------------------

class TestN001EqualNull:
    def test_triggers_on_eq_null(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x = NULL"), "null-equality")) == 1

    def test_two_occurrences(self):
        assert len(_diags(_r("SELECT * FROM t WHERE a = NULL AND b = NULL"), "null-equality")) == 2

    def test_no_trigger_is_null(self):
        assert _diags(_r("SELECT * FROM t WHERE x IS NULL"), "null-equality") == []

    def test_no_trigger_eq_literal(self):
        assert _diags(_r("SELECT * FROM t WHERE x = 1"), "null-equality") == []

    def test_no_trigger_eq_string(self):
        assert _diags(_r("SELECT * FROM t WHERE x = 'hello'"), "null-equality") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t WHERE x = NULL"), "null-equality")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# N002, <> NULL / != NULL
# ---------------------------------------------------------------------------

class TestN002NotEqualNull:
    def test_triggers_on_neq_null(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x <> NULL"), "null-inequality")) == 1

    def test_no_trigger_is_not_null(self):
        assert _diags(_r("SELECT * FROM t WHERE x IS NOT NULL"), "null-inequality") == []

    def test_no_trigger_neq_literal(self):
        assert _diags(_r("SELECT * FROM t WHERE x <> 1"), "null-inequality") == []

    def test_two_operators(self):
        assert len(_diags(_r("SELECT * FROM t WHERE a <> NULL AND b <> NULL"), "null-inequality")) == 2

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t WHERE x <> NULL"), "null-inequality")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# N003, NULL in NOT IN list
# ---------------------------------------------------------------------------

class TestN003NullInNotIn:
    def test_triggers_on_not_in_null(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x NOT IN (1, 2, NULL)"), "null-in-not-in")) == 1

    def test_triggers_not_in_null_only(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x NOT IN (NULL)"), "null-in-not-in")) == 1

    def test_no_trigger_plain_in_null(self):
        # S007 handles plain IN NULL, not N003
        assert _diags(_r("SELECT * FROM t WHERE x IN (1, 2, NULL)"), "null-in-not-in") == []

    def test_no_trigger_not_in_no_null(self):
        assert _diags(_r("SELECT * FROM t WHERE x NOT IN (1, 2, 3)"), "null-in-not-in") == []

    def test_no_trigger_not_in_subquery(self):
        assert _diags(_r("SELECT * FROM t WHERE x NOT IN (SELECT id FROM other)"), "null-in-not-in") == []

    def test_severity_is_warning(self):
        diags = _diags(_r("SELECT * FROM t WHERE x NOT IN (1, NULL)"), "null-in-not-in")
        assert diags[0]["severity"] == "warning"


# ---------------------------------------------------------------------------
# N004, COALESCE with one argument
# ---------------------------------------------------------------------------

class TestN004CoalesceOneArg:
    def test_triggers_on_one_arg(self):
        assert len(_diags(_r("SELECT COALESCE(x) FROM t"), "coalesce-single-arg")) == 1

    def test_no_trigger_two_args(self):
        assert _diags(_r("SELECT COALESCE(x, 0) FROM t"), "coalesce-single-arg") == []

    def test_no_trigger_three_args(self):
        assert _diags(_r("SELECT COALESCE(a, b, c) FROM t"), "coalesce-single-arg") == []

    def test_severity_is_warning(self):
        diags = _diags(_r("SELECT COALESCE(x) FROM t"), "coalesce-single-arg")
        assert diags[0]["severity"] == "warning"
