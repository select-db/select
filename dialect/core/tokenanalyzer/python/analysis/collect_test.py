"""Tests for collect.py, relations output shape."""
from analysis import analyze

SCHEMA = {"public": {"users": ["id", "name"], "orders": ["id", "user_id"]}}


def _r(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA, default_schema="public")


class TestRelations:
    def test_physical_table_in_relations(self):
        r = _r("SELECT * FROM users")
        tables = [rel["table"].lower() for rel in r["relations"]]
        assert "users" in tables

    def test_cte_marked_virtual(self):
        r = _r("WITH cte AS (SELECT 1) SELECT * FROM cte")
        virtual = [rel for rel in r["relations"] if rel["is_virtual"]]
        assert "cte" in [v["table"].lower() for v in virtual]

    def test_subquery_alias_marked_virtual(self):
        r = _r("SELECT * FROM (SELECT id FROM users) sub")
        virtual = [rel for rel in r["relations"] if rel["is_virtual"]]
        assert "sub" in [v["table"].lower() for v in virtual]

    def test_join_both_tables_present(self):
        r = _r("SELECT * FROM users JOIN orders ON users.id = orders.user_id")
        tables = {rel["table"].lower() for rel in r["relations"] if not rel["is_virtual"]}
        assert "users" in tables
        assert "orders" in tables
