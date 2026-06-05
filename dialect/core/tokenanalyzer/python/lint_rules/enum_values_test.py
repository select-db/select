"""Tests for enum_values.py, enum-value-mismatch (E001)."""
from analysis import analyze

SCHEMA = {
    "public": {"users": {"id": "integer", "status": "status"}},
    "app": {"orders": {"id": "integer", "state": "order_state"}},
}
ENUMS = {
    "public": {"users": {"status": ["active", "inactive"]}},
    "app": {"orders": {"state": ["new", "paid"]}},
}

RULE = "enum-value-mismatch"


def _r(sql, dialect="postgresql"):
    return analyze(sql, dialect=dialect, schema_dict=SCHEMA,
                   default_schema="public", enum_dict=ENUMS)


def _msgs(r):
    return [d["message"] for d in r["diagnostics"] if d["rule_id"] == RULE]


class TestFlagsBadValues:
    def test_where_eq(self):
        assert _msgs(_r("SELECT 1 FROM users WHERE status = 'activ'"))

    def test_where_in(self):
        assert _msgs(_r("SELECT 1 FROM users u WHERE u.status IN ('active','wrong')"))

    def test_update_set(self):
        assert _msgs(_r("UPDATE users SET status = 'bad' WHERE id = 1"))

    def test_delete_where(self):
        assert _msgs(_r("DELETE FROM users WHERE status = 'gone'"))

    def test_insert_multirow(self):
        msgs = _msgs(_r("INSERT INTO users (id, status) VALUES (1,'a'),(2,'inactive'),(3,'x')"))
        assert len(msgs) == 2

    def test_qualified_table_other_schema(self):
        assert _msgs(_r("SELECT 1 FROM app.orders WHERE state = 'cancelled'"))


class TestNoFalsePositives:
    def test_valid_value(self):
        assert not _msgs(_r("SELECT 1 FROM users WHERE status = 'active'"))

    def test_case_insensitive_tolerance(self):
        # avoid cross-dialect FP: 'ACTIVE' matches 'active' case-insensitively
        assert not _msgs(_r("SELECT 1 FROM users WHERE status = 'ACTIVE'"))

    def test_non_enum_column(self):
        assert not _msgs(_r("SELECT 1 FROM users WHERE id = 5"))

    def test_column_to_column(self):
        assert not _msgs(_r("UPDATE users SET status = status WHERE id = 1"))

    def test_placeholder(self):
        assert not _msgs(_r("SELECT 1 FROM users WHERE status = $1"))

    def test_no_enum_dict(self):
        r = analyze("SELECT 1 FROM users WHERE status = 'activ'",
                    dialect="postgresql", schema_dict=SCHEMA, default_schema="public")
        assert not [d for d in r["diagnostics"] if d["rule_id"] == RULE]

    def test_mysql_inline_enum_shape(self):
        # enum_dict is dialect-agnostic; inline enums resolve the same in Go,
        # here we just confirm the rule consumes the projected values.
        assert _msgs(_r("UPDATE users SET status = 'nope' WHERE id = 1", dialect="mysql"))
