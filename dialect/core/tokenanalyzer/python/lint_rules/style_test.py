"""Tests for style.py, S001–S011."""
from analysis import analyze


def _r(sql):
    return analyze(sql, dialect="postgresql", schema_dict={}, default_schema="public")


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


# ---------------------------------------------------------------------------
# S001, LIKE with no wildcard
# ---------------------------------------------------------------------------

class TestS001LikeNoWildcard:
    def test_triggers_no_wildcard(self):
        assert len(_diags(_r("SELECT * FROM t WHERE name LIKE 'foo'"), "like-no-wildcard")) == 1

    def test_no_trigger_percent(self):
        assert _diags(_r("SELECT * FROM t WHERE name LIKE 'foo%'"), "like-no-wildcard") == []

    def test_no_trigger_underscore(self):
        assert _diags(_r("SELECT * FROM t WHERE name LIKE 'f_o'"), "like-no-wildcard") == []

    def test_triggers_ilike_no_wildcard(self):
        assert len(_diags(_r("SELECT * FROM t WHERE name ILIKE 'bar'"), "like-no-wildcard")) == 1

    def test_no_trigger_ilike_with_wildcard(self):
        assert _diags(_r("SELECT * FROM t WHERE name ILIKE '%foo'"), "like-no-wildcard") == []

    def test_severity_is_warning(self):
        diags = _diags(_r("SELECT * FROM t WHERE name LIKE 'foo'"), "like-no-wildcard")
        assert diags[0]["severity"] == "warning"


# ---------------------------------------------------------------------------
# S002, LIKE with leading wildcard
# ---------------------------------------------------------------------------

class TestS002LikeLeadingWildcard:
    def test_triggers_leading_percent(self):
        assert len(_diags(_r("SELECT * FROM t WHERE name LIKE '%foo'"), "like-leading-wildcard")) == 1

    def test_no_trigger_trailing_wildcard(self):
        assert _diags(_r("SELECT * FROM t WHERE name LIKE 'foo%'"), "like-leading-wildcard") == []

    def test_no_trigger_middle_wildcard(self):
        assert _diags(_r("SELECT * FROM t WHERE name LIKE 'f%o'"), "like-leading-wildcard") == []

    def test_triggers_ilike_leading(self):
        assert len(_diags(_r("SELECT * FROM t WHERE name ILIKE '%foo'"), "like-leading-wildcard")) == 1

    def test_severity_is_hint(self):
        diags = _diags(_r("SELECT * FROM t WHERE name LIKE '%foo'"), "like-leading-wildcard")
        assert diags[0]["severity"] == "hint"


# ---------------------------------------------------------------------------
# S003, Tautological predicate
# ---------------------------------------------------------------------------

class TestS003Tautology:
    def test_triggers_1_eq_1(self):
        assert len(_diags(_r("SELECT * FROM t WHERE 1=1"), "tautological-predicate")) == 1

    def test_no_trigger_col_eq_literal(self):
        assert _diags(_r("SELECT * FROM t WHERE x = 1"), "tautological-predicate") == []

    def test_severity_is_warning(self):
        diags = _diags(_r("SELECT * FROM t WHERE 1=1"), "tautological-predicate")
        assert diags[0]["severity"] == "warning"


# ---------------------------------------------------------------------------
# S004, Empty IN list
# ---------------------------------------------------------------------------

class TestS004EmptyInList:
    def test_triggers_empty_in(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x IN ()"), "empty-in-list")) == 1

    def test_triggers_empty_not_in(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x NOT IN ()"), "empty-in-list")) == 1

    def test_no_trigger_with_values(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (1, 2)"), "empty-in-list") == []

    def test_no_trigger_subquery(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (SELECT id FROM other)"), "empty-in-list") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t WHERE x IN ()"), "empty-in-list")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# S005, BETWEEN with reversed bounds
# ---------------------------------------------------------------------------

class TestS005BetweenReversed:
    def test_triggers_reversed(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x BETWEEN 10 AND 1"), "reversed-between")) == 1

    def test_no_trigger_correct(self):
        assert _diags(_r("SELECT * FROM t WHERE x BETWEEN 1 AND 10"), "reversed-between") == []

    def test_no_trigger_equal(self):
        assert _diags(_r("SELECT * FROM t WHERE x BETWEEN 5 AND 5"), "reversed-between") == []

    def test_no_trigger_non_numeric(self):
        assert _diags(_r("SELECT * FROM t WHERE x BETWEEN 'b' AND 'a'"), "reversed-between") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t WHERE x BETWEEN 10 AND 1"), "reversed-between")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# S006, Duplicate literals in IN list
# ---------------------------------------------------------------------------

class TestS006InListDuplicates:
    def test_triggers_duplicate_number(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x IN (1, 2, 1)"), "duplicate-in-value")) == 1

    def test_triggers_duplicate_string(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x IN ('a', 'b', 'a')"), "duplicate-in-value")) == 1

    def test_no_trigger_unique_values(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (1, 2, 3)"), "duplicate-in-value") == []

    def test_no_trigger_subquery(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (SELECT id FROM other)"), "duplicate-in-value") == []

    def test_two_duplicates(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x IN (1, 1, 2, 2)"), "duplicate-in-value")) == 2

    def test_severity_is_warning(self):
        diags = _diags(_r("SELECT * FROM t WHERE x IN (1, 2, 1)"), "duplicate-in-value")
        assert diags[0]["severity"] == "warning"


# ---------------------------------------------------------------------------
# S007, NULL in plain IN list
# ---------------------------------------------------------------------------

class TestS007NullInInList:
    def test_triggers_null_in_in(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x IN (1, 2, NULL)"), "null-in-list")) == 1

    def test_no_trigger_not_in_null(self):
        # N003 handles this case
        assert _diags(_r("SELECT * FROM t WHERE x NOT IN (1, NULL)"), "null-in-list") == []

    def test_no_trigger_in_without_null(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (1, 2, 3)"), "null-in-list") == []

    def test_severity_is_warning(self):
        diags = _diags(_r("SELECT * FROM t WHERE x IN (1, NULL)"), "null-in-list")
        assert diags[0]["severity"] == "warning"


# ---------------------------------------------------------------------------
# S008, Division by zero
# ---------------------------------------------------------------------------

class TestS008DivisionByZero:
    def test_triggers_div_zero(self):
        assert len(_diags(_r("SELECT x / 0 FROM t"), "division-by-zero")) == 1

    def test_triggers_div_zero_float(self):
        assert len(_diags(_r("SELECT x / 0.0 FROM t"), "division-by-zero")) == 1

    def test_no_trigger_div_one(self):
        assert _diags(_r("SELECT x / 1 FROM t"), "division-by-zero") == []

    def test_no_trigger_div_column(self):
        assert _diags(_r("SELECT x / y FROM t"), "division-by-zero") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT x / 0 FROM t"), "division-by-zero")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# S009, Contradictory numeric predicates
# ---------------------------------------------------------------------------

class TestS009ContradictoryPredicate:
    def test_triggers_gt_lt_contradictory(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x > 5 AND x < 3"), "contradictory-predicate")) == 1

    def test_no_trigger_valid_range(self):
        assert _diags(_r("SELECT * FROM t WHERE x > 3 AND x < 5"), "contradictory-predicate") == []

    def test_no_trigger_equal_inclusive(self):
        assert _diags(_r("SELECT * FROM t WHERE x >= 5 AND x <= 5"), "contradictory-predicate") == []

    def test_triggers_strict_equal(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x > 5 AND x < 5"), "contradictory-predicate")) == 1

    def test_no_trigger_different_columns(self):
        assert _diags(_r("SELECT * FROM t WHERE a > 5 AND b < 3"), "contradictory-predicate") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT * FROM t WHERE x > 5 AND x < 3"), "contradictory-predicate")
        assert diags[0]["severity"] == "error"


# ---------------------------------------------------------------------------
# S010, Trailing comma
# ---------------------------------------------------------------------------

class TestS010TrailingComma:
    def test_triggers_trailing_comma_before_from(self):
        assert len(_diags(_r("SELECT a, b, FROM t"), "trailing-comma")) == 1

    def test_triggers_trailing_comma_before_where(self):
        assert len(_diags(_r("SELECT a FROM t WHERE x IN (1, 2,)"), "trailing-comma")) == 1

    def test_triggers_trailing_comma_in_group_by(self):
        assert len(_diags(_r("SELECT a, COUNT(*) FROM t GROUP BY a, ORDER BY a"), "trailing-comma")) >= 1

    def test_triggers_trailing_comma_before_paren(self):
        assert len(_diags(_r("SELECT * FROM t WHERE x IN (1, 2, 3,)"), "trailing-comma")) == 1

    def test_no_trigger_valid_select(self):
        assert _diags(_r("SELECT a, b, c FROM t"), "trailing-comma") == []

    def test_no_trigger_valid_in_list(self):
        assert _diags(_r("SELECT * FROM t WHERE x IN (1, 2, 3)"), "trailing-comma") == []

    def test_no_trigger_comma_in_string(self):
        assert _diags(_r("SELECT 'a, FROM b' FROM t"), "trailing-comma") == []

    def test_severity_is_error(self):
        diags = _diags(_r("SELECT a, b, FROM t"), "trailing-comma")
        assert diags[0]["severity"] == "error"

    def test_position_reported(self):
        diags = _diags(_r("SELECT a, b, FROM t"), "trailing-comma")
        assert diags[0]["start_line"] == 1
        assert diags[0]["start_col"] >= 0

    def test_triggers_multiline(self):
        sql = "SELECT\n  a,\n  b,\nFROM t"
        assert len(_diags(_r(sql), "trailing-comma")) == 1


# ---------------------------------------------------------------------------
# S011, Unquoted mixed-case identifier
# ---------------------------------------------------------------------------

class TestS011UnquotedMixedCase:
    def test_triggers_mixed_case_table(self):
        assert len(_diags(_r("SELECT * FROM customerAccount"), "unquoted-uppercase")) == 1

    def test_triggers_mixed_case_column(self):
        assert len(_diags(_r("SELECT myCol FROM t"), "unquoted-uppercase")) == 1

    def test_triggers_schema_qualified(self):
        diags = _diags(_r("SELECT * FROM mySchema.customerAccount"), "unquoted-uppercase")
        names = [d["message"] for d in diags]
        assert any("customerAccount" in m for m in names)
        assert any("mySchema" in m for m in names)

    def test_no_trigger_lowercase(self):
        assert _diags(_r("SELECT id FROM users"), "unquoted-uppercase") == []

    def test_no_trigger_quoted(self):
        assert _diags(_r('SELECT * FROM "customerAccount"'), "unquoted-uppercase") == []

    def test_no_trigger_quoted_column(self):
        assert _diags(_r('SELECT "myCol" FROM t'), "unquoted-uppercase") == []

    def test_no_trigger_mysql(self):
        r = analyze("SELECT * FROM customerAccount", dialect="mysql",
                    schema_dict={}, default_schema="public")
        assert [d for d in r["diagnostics"] if d["rule_id"] == "unquoted-uppercase"] == []

    def test_alias_unquoted_mixed_triggers(self):
        assert len(_diags(_r("SELECT id AS myAlias FROM t"), "unquoted-uppercase")) == 1

    def test_alias_quoted_no_trigger(self):
        assert _diags(_r('SELECT id AS "myAlias" FROM t'), "unquoted-uppercase") == []

    def test_severity_is_warning(self):
        diags = _diags(_r("SELECT * FROM customerAccount"), "unquoted-uppercase")
        assert diags[0]["severity"] == "warning"

    def test_position_reported(self):
        diags = _diags(_r("SELECT * FROM customerAccount"), "unquoted-uppercase")
        assert diags[0]["start_line"] >= 1
        assert "customerAccount" in diags[0]["message"]
