"""Tests for structure.py, G001/G002, W001, S010."""
from analysis import analyze

SCHEMA = {
    "public": {
        "users":  {"id": "integer", "name": "character varying", "score": "double precision"},
        "orders": {"id": "integer", "user_id": "integer"},
    }
}


def _r(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA, default_schema="public")


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


# ---------------------------------------------------------------------------
# G001 / G002, Ordinal out of range
# ---------------------------------------------------------------------------

class TestOrdinalViolations:
    def test_group_by_valid_ordinal_no_trigger(self):
        assert _diags(_r("SELECT id, name FROM users GROUP BY 1"), "ordinal-out-of-range") == []

    def test_order_by_valid_ordinal_no_trigger(self):
        assert _diags(_r("SELECT id, name FROM users ORDER BY 2"), "ordinal-out-of-range") == []

    def test_group_by_ordinal_too_large_triggers(self):
        diags = _diags(_r("SELECT id FROM users GROUP BY 5"), "ordinal-out-of-range")
        assert len(diags) >= 1
        assert "GROUP BY" in diags[0]["message"]
        assert "5" in diags[0]["message"]

    def test_order_by_ordinal_too_large_triggers(self):
        diags = _diags(_r("SELECT id FROM users ORDER BY 5"), "ordinal-out-of-range")
        assert len(diags) >= 1
        assert "ORDER BY" in diags[0]["message"]

    def test_both_clauses_can_trigger(self):
        r = _r("SELECT id FROM users GROUP BY 9 ORDER BY 9")
        assert len(_diags(r, "ordinal-out-of-range")) >= 1
        assert len(_diags(r, "ordinal-out-of-range")) >= 1

    def test_column_name_not_flagged(self):
        assert _diags(_r("SELECT id FROM users GROUP BY id"), "ordinal-out-of-range") == []

    def test_ordinal_zero_not_flagged(self):
        # 0 is invalid SQL but not > col_count
        assert _diags(_r("SELECT id FROM users ORDER BY 0"), "ordinal-out-of-range") == []

    def test_position_reported(self):
        diags = _diags(_r("SELECT id FROM users ORDER BY 9"), "ordinal-out-of-range")
        assert diags[0]["start_line"] >= 1


# ---------------------------------------------------------------------------
# W001, Window function in WHERE
# ---------------------------------------------------------------------------

class TestWindowInWhere:
    def test_window_in_select_no_trigger(self):
        assert _diags(_r("SELECT ROW_NUMBER() OVER (ORDER BY id) FROM users"), "window-in-where") == []

    def test_window_in_where_triggers(self):
        diags = _diags(_r("SELECT id FROM users WHERE ROW_NUMBER() OVER (ORDER BY id) = 1"), "window-in-where")
        assert len(diags) == 1

    def test_window_in_where_position_reported(self):
        diags = _diags(_r("SELECT id FROM users WHERE ROW_NUMBER() OVER (ORDER BY id) > 0"), "window-in-where")
        assert diags[0]["start_line"] >= 1

    def test_no_window_no_trigger(self):
        assert _diags(_r("SELECT id FROM users WHERE id > 0"), "window-in-where") == []

    def test_window_in_having_not_flagged(self):
        assert _diags(_r("SELECT id, COUNT(*) FROM users GROUP BY id HAVING COUNT(*) > 1"), "window-in-where") == []


# ---------------------------------------------------------------------------
# S010, Duplicate SELECT output column names
# ---------------------------------------------------------------------------

class TestS010DuplicateColumnName:
    def test_distinct_columns_no_trigger(self):
        assert _diags(_r("SELECT id, name FROM users"), "duplicate-column") == []

    def test_duplicate_bare_column_triggers(self):
        diags = _diags(_r("SELECT id, id FROM users"), "duplicate-column")
        assert len(diags) == 1
        assert "id" in diags[0]["message"].lower()

    def test_duplicate_alias_triggers(self):
        diags = _diags(_r("SELECT id AS x, name AS x FROM users"), "duplicate-column")
        assert len(diags) == 1
        assert "x" in diags[0]["message"].lower()

    def test_alias_collides_with_column_triggers(self):
        assert len(_diags(_r("SELECT id, score AS id FROM users"), "duplicate-column")) == 1

    def test_case_insensitive(self):
        assert len(_diags(_r('SELECT id, "ID" FROM users'), "duplicate-column")) == 1

    def test_three_duplicates_reports_two(self):
        assert len(_diags(_r("SELECT id, id, id FROM users"), "duplicate-column")) == 2

    def test_star_not_flagged(self):
        assert _diags(_r("SELECT *, id FROM users"), "duplicate-column") == []

    def test_position_reported(self):
        diags = _diags(_r("SELECT id, id FROM users"), "duplicate-column")
        assert diags[0]["start_line"] >= 1
