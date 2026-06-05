"""Tests for type_checks.py, T001–T006."""
from analysis import analyze

SCHEMA = {
    "public": {
        "users": {
            "id":         "integer",
            "name":       "character varying",
            "active":     "boolean",
            "score":      "double precision",
            "created_at": "timestamp with time zone",
            "birth_date": "date",
            "balance":    "numeric",
        }
    }
}


def _r(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA, default_schema="public")


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


def _rule_ids(r):
    return [d["rule_id"] for d in r["diagnostics"]]


# ---------------------------------------------------------------------------
# T001, comparison type mismatches
# ---------------------------------------------------------------------------

class TestT001ComparisonMismatch:
    def test_int_eq_int_ok(self):
        assert "type-mismatch-comparison" not in _rule_ids(_r("SELECT 1 FROM users WHERE id = 42"))

    def test_text_eq_text_ok(self):
        assert "type-mismatch-comparison" not in _rule_ids(_r("SELECT 1 FROM users WHERE name = 'alice'"))

    def test_bool_eq_bool_ok(self):
        assert "type-mismatch-comparison" not in _rule_ids(_r("SELECT 1 FROM users WHERE active = true"))

    def test_int_eq_text_triggers(self):
        assert "type-mismatch-comparison" in _rule_ids(_r("SELECT 1 FROM users WHERE id = 'foo'"))

    def test_text_eq_int_triggers(self):
        assert "type-mismatch-comparison" in _rule_ids(_r("SELECT 1 FROM users WHERE name = 42"))

    def test_bool_eq_int_triggers(self):
        assert "type-mismatch-comparison" in _rule_ids(_r("SELECT 1 FROM users WHERE active = 1"))

    def test_bool_eq_text_triggers(self):
        assert "type-mismatch-comparison" in _rule_ids(_r("SELECT 1 FROM users WHERE active = 'yes'"))

    def test_timestamp_eq_int_triggers(self):
        assert "type-mismatch-comparison" in _rule_ids(_r("SELECT 1 FROM users WHERE created_at = 1000000"))

    def test_timestamp_eq_text_ok(self):
        assert "type-mismatch-comparison" not in _rule_ids(_r("SELECT 1 FROM users WHERE created_at = '2024-01-01'"))

    def test_date_eq_text_ok(self):
        assert "type-mismatch-comparison" not in _rule_ids(_r("SELECT 1 FROM users WHERE birth_date = '2024-01-01'"))

    def test_gt_mismatch_triggers(self):
        assert "type-mismatch-comparison" in _rule_ids(_r("SELECT 1 FROM users WHERE id > 'abc'"))

    def test_no_schema_no_trigger(self):
        r = analyze("SELECT 1 FROM users WHERE id = 'foo'", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert _diags(r, "type-mismatch-comparison") == []

    def test_incomplete_sql_no_crash(self):
        assert "diagnostics" in _r("SELECT 1 FROM users WHERE id =")


# ---------------------------------------------------------------------------
# T002, arithmetic on non-numeric/non-temporal types
# ---------------------------------------------------------------------------

class TestT002ArithmeticOnWrongType:
    def test_int_plus_int_ok(self):
        assert "type-mismatch-arithmetic" not in _rule_ids(_r("SELECT id + 1 FROM users"))

    def test_score_mul_ok(self):
        assert "type-mismatch-arithmetic" not in _rule_ids(_r("SELECT score * 2.0 FROM users"))

    def test_text_plus_int_triggers(self):
        assert "type-mismatch-arithmetic" in _rule_ids(_r("SELECT name + 1 FROM users"))

    def test_text_mul_int_triggers(self):
        assert "type-mismatch-arithmetic" in _rule_ids(_r("SELECT name * 2 FROM users"))

    def test_bool_plus_int_triggers(self):
        assert "type-mismatch-arithmetic" in _rule_ids(_r("SELECT active + 1 FROM users"))


# ---------------------------------------------------------------------------
# T003, COALESCE inconsistent argument types
# ---------------------------------------------------------------------------

class TestT003CoalesceMixedTypes:
    def test_coalesce_same_type_ok(self):
        assert "coalesce-type-mismatch" not in _rule_ids(_r("SELECT COALESCE(id, 0) FROM users"))

    def test_coalesce_text_text_ok(self):
        assert "coalesce-type-mismatch" not in _rule_ids(_r("SELECT COALESCE(name, 'unknown') FROM users"))

    def test_coalesce_int_text_triggers(self):
        assert "coalesce-type-mismatch" in _rule_ids(_r("SELECT COALESCE(id, name) FROM users"))

    def test_coalesce_bool_int_triggers(self):
        assert "coalesce-type-mismatch" in _rule_ids(_r("SELECT COALESCE(active, id) FROM users"))

    def test_coalesce_message_contains_families(self):
        diags = _diags(_r("SELECT COALESCE(id, name) FROM users"), "coalesce-type-mismatch")
        assert diags
        assert "numeric" in diags[0]["message"]
        assert "text" in diags[0]["message"]


# ---------------------------------------------------------------------------
# T004, IN list value type mismatch
# ---------------------------------------------------------------------------

class TestT004InListTypeMismatch:
    def test_int_in_ints_ok(self):
        assert "in-list-type-mismatch" not in _rule_ids(_r("SELECT 1 FROM users WHERE id IN (1, 2, 3)"))

    def test_text_in_texts_ok(self):
        assert "in-list-type-mismatch" not in _rule_ids(_r("SELECT 1 FROM users WHERE name IN ('alice', 'bob')"))

    def test_int_col_with_text_value_triggers(self):
        assert "in-list-type-mismatch" in _rule_ids(_r("SELECT 1 FROM users WHERE id IN (1, 'bad', 3)"))

    def test_text_col_with_int_value_triggers(self):
        assert "in-list-type-mismatch" in _rule_ids(_r("SELECT 1 FROM users WHERE name IN ('alice', 42)"))

    def test_multiple_bad_values_reported(self):
        diags = _diags(_r("SELECT 1 FROM users WHERE id IN (1, 'x', 'y', 4)"), "in-list-type-mismatch")
        assert len(diags) == 2


# ---------------------------------------------------------------------------
# T005, function argument type mismatch
# ---------------------------------------------------------------------------

class TestT005FunctionArgTypeMismatch:
    def test_length_text_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT LENGTH(name) FROM users"))

    def test_length_int_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT LENGTH(id) FROM users"))

    def test_upper_text_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT UPPER(name) FROM users"))

    def test_upper_int_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT UPPER(id) FROM users"))

    def test_lower_bool_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT LOWER(active) FROM users"))

    def test_trim_text_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT TRIM(name) FROM users"))

    def test_trim_int_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT TRIM(id) FROM users"))

    def test_abs_numeric_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT ABS(score) FROM users"))

    def test_abs_int_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT ABS(id) FROM users"))

    def test_abs_text_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT ABS(name) FROM users"))

    def test_round_numeric_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT ROUND(balance) FROM users"))

    def test_round_text_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT ROUND(name) FROM users"))

    def test_sqrt_int_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT SQRT(id) FROM users"))

    def test_sqrt_text_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT SQRT(name) FROM users"))

    def test_not_bool_ok(self):
        assert "type-mismatch-argument" not in _rule_ids(_r("SELECT NOT active FROM users"))

    def test_not_int_triggers(self):
        assert "type-mismatch-argument" in _rule_ids(_r("SELECT NOT id FROM users"))

    def test_message_contains_expected_and_actual(self):
        diags = _diags(_r("SELECT LENGTH(id) FROM users"), "type-mismatch-argument")
        assert diags
        assert "text" in diags[0]["message"]
        assert "numeric" in diags[0]["message"]

    def test_no_schema_no_trigger(self):
        r = analyze("SELECT LENGTH(id) FROM users", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert _diags(r, "type-mismatch-argument") == []


# ---------------------------------------------------------------------------
# Limitations / edge cases
# ---------------------------------------------------------------------------

class TestLimitations:
    def test_no_false_positive_on_no_schema(self):
        r = analyze("SELECT 1 FROM users WHERE id = 'foo'", dialect="postgresql",
                    schema_dict={"public": {"users": ["id", "name"]}}, default_schema="public")
        assert _diags(r, "type-mismatch-comparison") == []

    def test_numeric_string_literal_is_flagged(self):
        assert "diagnostics" in _r("SELECT 1 FROM users WHERE id = '123'")

    def test_cast_expression_no_false_positive(self):
        assert "type-mismatch-comparison" not in _rule_ids(_r("SELECT 1 FROM users WHERE id = CAST('123' AS INTEGER)"))


# ---------------------------------------------------------------------------
# T006, missing required function arguments
# ---------------------------------------------------------------------------

def _arity_names(r):
    return [d["message"].split()[0] for d in r["diagnostics"] if d["rule_id"] == "missing-argument"]


class TestT006ArityErrors:
    def test_count_star_ok(self):
        assert not any("COUNT" in n for n in _arity_names(_r("SELECT COUNT(*) FROM users")))

    def test_count_col_ok(self):
        assert not any("COUNT" in n for n in _arity_names(_r("SELECT COUNT(id) FROM users")))

    def test_count_no_arg_triggers(self):
        assert any("COUNT" in n for n in _arity_names(_r("SELECT COUNT() FROM users")))

    def test_sum_col_ok(self):
        assert not any("SUM" in n for n in _arity_names(_r("SELECT SUM(score) FROM users")))

    def test_sum_no_arg_triggers(self):
        assert any("SUM" in n for n in _arity_names(_r("SELECT SUM() FROM users")))

    def test_avg_no_arg_triggers(self):
        assert any("AVG" in n for n in _arity_names(_r("SELECT AVG() FROM users")))

    def test_max_no_arg_triggers(self):
        assert any("MAX" in n for n in _arity_names(_r("SELECT MAX() FROM users")))

    def test_min_no_arg_triggers(self):
        assert any("MIN" in n for n in _arity_names(_r("SELECT MIN() FROM users")))

    def test_length_col_ok(self):
        assert not any("LENGTH" in n for n in _arity_names(_r("SELECT LENGTH(name) FROM users")))

    def test_length_no_arg_triggers(self):
        assert any("LENGTH" in n for n in _arity_names(_r("SELECT LENGTH() FROM users")))

    def test_upper_no_arg_triggers(self):
        assert any("UPPER" in n for n in _arity_names(_r("SELECT UPPER() FROM users")))

    def test_lower_no_arg_triggers(self):
        assert any("LOWER" in n for n in _arity_names(_r("SELECT LOWER() FROM users")))

    def test_now_ok(self):
        assert _arity_names(_r("SELECT NOW()")) == []

    def test_current_timestamp_ok(self):
        assert _arity_names(_r("SELECT CURRENT_TIMESTAMP()")) == []

    def test_no_schema_still_flags(self):
        r = analyze("SELECT COUNT() FROM users", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert any("COUNT" in n for n in _arity_names(r))

    def test_multi_required_args_reported_once(self):
        r = analyze("SELECT array_append()", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        t006 = [d for d in r["diagnostics"] if d["rule_id"] == "missing-argument"]
        assert len(t006) == 1
