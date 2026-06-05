"""
Tests for references.py, R001, R002, R004, R008.
Mirrors the Go lint rule test cases in dialect/core/lint/references_test.go.
"""
from analysis import analyze

SCHEMA = {"public": {"users": ["id", "name"], "orders": ["id", "user_id"]}}
SCHEMA_MIXED_CASE = {"public": {"users": ["id", "createdAt"], "time_series": ["interval_start"]}}
DEFAULT_SCHEMA = "public"


def _r(sql, schema=SCHEMA):
    return analyze(sql, dialect="postgresql", schema_dict=schema, default_schema=DEFAULT_SCHEMA)


def _diags(r, rule_id):
    return [d for d in r["diagnostics"] if d["rule_id"] == rule_id]


# ---------------------------------------------------------------------------
# R001, Unknown table
# ---------------------------------------------------------------------------

class TestR001UnknownTable:
    def test_known_table_no_trigger(self):
        assert _diags(_r("SELECT * FROM users"), "unknown-table") == []

    def test_unknown_table_triggers(self):
        diags = _diags(_r("SELECT * FROM typo_table"), "unknown-table")
        assert len(diags) == 1
        assert "typo_table" in diags[0]["message"]

    def test_cte_name_not_flagged(self):
        assert _diags(_r("WITH cte AS (SELECT 1) SELECT * FROM cte"), "unknown-table") == []

    def test_subquery_alias_not_flagged(self):
        assert _diags(_r("SELECT * FROM (SELECT 1 AS x) sub"), "unknown-table") == []

    def test_cross_database_not_flagged(self):
        assert _diags(_r("SELECT * FROM otherdb.public.users"), "unknown-table") == []

    def test_unknown_table_in_join(self):
        diags = _diags(_r("SELECT * FROM users JOIN no_such_table ON users.id = no_such_table.id"), "unknown-table")
        assert any("no_such_table" in d["message"] for d in diags)

    def test_unloaded_schema_not_flagged(self):
        assert _diags(_r("SELECT * FROM other_schema.something"), "unknown-table") == []

    def test_empty_metadata_no_trigger(self):
        r = analyze("SELECT * FROM anything", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert _diags(r, "unknown-table") == []


# ---------------------------------------------------------------------------
# R002, Unknown column
# ---------------------------------------------------------------------------

class TestR002UnknownColumn:
    def test_known_column_no_trigger(self):
        assert _diags(_r("SELECT id FROM users"), "unknown-column") == []

    def test_known_column_with_alias_no_trigger(self):
        assert _diags(_r("SELECT u.id FROM users u"), "unknown-column") == []

    def test_unknown_column_triggers(self):
        diags = _diags(_r("SELECT typo FROM users"), "unknown-column")
        assert any("typo" in d["message"].lower() for d in diags)

    def test_unknown_column_with_alias_triggers(self):
        diags = _diags(_r("SELECT u.typo FROM users u"), "unknown-column")
        assert any("typo" in d["message"].lower() for d in diags)

    def test_select_star_no_trigger(self):
        assert _diags(_r("SELECT * FROM users"), "unknown-column") == []

    def test_no_trigger_unknown_table_columns(self):
        assert _diags(_r("SELECT foo FROM no_such_table"), "unknown-column") == []

    def test_known_column_in_join_no_trigger(self):
        assert _diags(_r("SELECT id, user_id FROM users JOIN orders ON users.id = orders.user_id"), "unknown-column") == []

    def test_only_typo_triggers_not_valid(self):
        diags = _diags(_r("SELECT id, typo FROM users"), "unknown-column")
        assert any("typo" in d["message"].lower() for d in diags)
        assert not any(d["message"].lower() == '"id"' for d in diags)

    def test_unknown_column_in_where(self):
        diags = _diags(_r("SELECT id FROM users WHERE nocolumn = 1"), "unknown-column")
        assert any("nocolumn" in d["message"].lower() for d in diags)

    def test_known_column_in_where_no_trigger(self):
        assert _diags(_r("SELECT id FROM users WHERE name = 'x'"), "unknown-column") == []

    def test_unknown_column_in_group_by(self):
        diags = _diags(_r("SELECT id FROM users GROUP BY nocolumn"), "unknown-column")
        assert any("nocolumn" in d["message"].lower() for d in diags)

    def test_known_column_in_order_by_no_trigger(self):
        assert _diags(_r("SELECT id FROM users ORDER BY name"), "unknown-column") == []

    def test_qualified_column_ref_no_trigger(self):
        assert _diags(_r("SELECT u.id FROM users u WHERE u.name = 'x'"), "unknown-column") == []

    def test_cte_with_valid_columns_no_trigger(self):
        assert _diags(_r("WITH u AS (SELECT 1 FROM users) SELECT * FROM u"), "unknown-column") == []

    def test_cte_with_missing_column_triggers(self):
        diags = _diags(_r("WITH u AS (SELECT id FROM users) SELECT name FROM u"), "unknown-column")
        assert any("name" in d["message"].lower() for d in diags)

    def test_select_alias_in_order_by_no_trigger(self):
        diags = _diags(_r("SELECT id AS user_id FROM users ORDER BY user_id"), "unknown-column")
        assert not any('"id"' in d["message"] for d in diags)

    def test_select_alias_in_group_by_no_trigger(self):
        # PostgreSQL allows GROUP BY to reference SELECT aliases.
        sql = "SELECT id % 10 AS bucket, COUNT(*) FROM users GROUP BY bucket"
        assert _diags(_r(sql), "unknown-column") == []

    def test_computed_alias_in_group_by_and_order_by_no_trigger(self):
        # Mirrors the real-world case: date_trunc alias used in GROUP BY and ORDER BY.
        sql = (
            "SELECT date_trunc('minute', id) AS time_slot, COUNT(*) AS job_count "
            "FROM users GROUP BY time_slot ORDER BY time_slot"
        )
        assert _diags(_r(sql), "unknown-column") == []

    def test_count_star_no_trigger(self):
        assert _diags(_r("SELECT COUNT(*) AS total FROM users"), "unknown-column") == []

    def test_empty_metadata_no_trigger(self):
        r = analyze("SELECT anything FROM users", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert _diags(r, "unknown-column") == []

    def test_undefined_alias_qualifier_triggers(self):
        diags = _diags(_r("SELECT typo.id FROM users u"), "unknown-column")
        assert len(diags) >= 1
        assert any("typo" in d["message"].lower() for d in diags)

    def test_valid_alias_qualifier_no_trigger(self):
        assert _diags(_r("SELECT u.id FROM users u"), "unknown-column") == []

    def test_quoted_cte_column_in_join_no_trigger(self):
        sql = (
            'WITH accounts AS (SELECT id, "createdAt" FROM users) '
            'SELECT * FROM time_series ts '
            'LEFT JOIN accounts aa ON aa."createdAt" >= ts.interval_start'
        )
        r = analyze(sql, dialect="postgresql",
                    schema_dict=SCHEMA_MIXED_CASE, default_schema=DEFAULT_SCHEMA)
        assert not any("createdAt" in d["message"] for d in _diags(r, "unknown-column"))


# ---------------------------------------------------------------------------
# R003, Unknown column in UPDATE SET
# ---------------------------------------------------------------------------

SCHEMA_UPDATE = {
    "zygon": {
        "App":     ["id", "name", "email"],
        "Account": ["id", "ref", "label"],
    }
}

def _ru(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA_UPDATE, default_schema="zygon")


class TestR003UpdateSetColumn:
    """Simple UPDATE (no FROM), SET and WHERE column validation."""

    def test_valid_set_column_no_trigger(self):
        assert _diags(_ru('UPDATE zygon."App" SET name = \'x\' WHERE id = \'1\''), "unknown-column") == []

    def test_typo_in_set_triggers(self):
        diags = _diags(_ru('UPDATE zygon."App" SET typo = \'x\' WHERE id = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_multiple_set_columns_one_typo(self):
        diags = _diags(_ru('UPDATE zygon."App" SET name = \'x\', typo = \'y\' WHERE id = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)
        assert not any("name" in d["message"] for d in diags)

    def test_multiple_set_columns_all_valid(self):
        assert _diags(_ru('UPDATE zygon."App" SET name = \'x\', email = \'y\' WHERE id = \'1\''), "unknown-column") == []

    def test_valid_where_column_no_trigger(self):
        assert _diags(_ru('UPDATE zygon."App" SET name = \'x\' WHERE id = \'1\''), "unknown-column") == []

    def test_typo_in_where_triggers(self):
        diags = _diags(_ru('UPDATE zygon."App" SET name = \'x\' WHERE typo = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_unknown_table_skips_column_check(self):
        assert _diags(_ru("UPDATE zygon.\"NoTable\" SET typo = 'x'"), "unknown-column") == []

    def test_unloaded_schema_skips_column_check(self):
        assert _diags(_ru("UPDATE other_schema.\"App\" SET typo = 'x'"), "unknown-column") == []

    def test_empty_metadata_no_trigger(self):
        r = analyze("UPDATE t SET typo = 'x'", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert _diags(r, "unknown-column") == []

    def test_target_table_alias_set_valid(self):
        assert _diags(_ru('UPDATE zygon."App" AS a SET a.name = \'x\' WHERE a.id = \'1\''), "unknown-column") == []

    def test_target_table_alias_set_typo_triggers(self):
        diags = _diags(_ru('UPDATE zygon."App" AS a SET a.typo = \'x\' WHERE a.id = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_default_schema_resolution(self):
        assert _diags(_ru('UPDATE "App" SET name = \'x\' WHERE id = \'1\''), "unknown-column") == []

    def test_default_schema_typo_triggers(self):
        diags = _diags(_ru('UPDATE "App" SET typo = \'x\' WHERE id = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR003UpdateFromClause:
    """UPDATE … FROM, columns from joined tables must also be validated."""

    def test_from_valid_where_column_no_trigger(self):
        sql = 'UPDATE zygon."App" SET name = \'x\' FROM zygon."Account" acc WHERE "App".id = acc.ref'
        assert _diags(_ru(sql), "unknown-column") == []

    def test_from_typo_in_where_triggers(self):
        sql = 'UPDATE zygon."App" SET name = \'x\' FROM zygon."Account" acc WHERE "App".id = acc.typo'
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_from_typo_in_set_triggers(self):
        # SET LHS always belongs to target table, typo is invalid regardless of FROM
        sql = 'UPDATE zygon."App" SET typo = \'x\' FROM zygon."Account" acc WHERE "App".id = acc.ref'
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_from_valid_set_no_trigger(self):
        sql = 'UPDATE zygon."App" SET name = \'x\' FROM zygon."Account" acc WHERE "App".id = acc.ref'
        assert _diags(_ru(sql), "unknown-column") == []

    def test_from_unknown_table_where_column_skipped(self):
        # FROM table not in schema, can't validate its columns
        sql = 'UPDATE zygon."App" SET name = \'x\' FROM unknown_table acc WHERE "App".id = acc.typo'
        assert _diags(_ru(sql), "unknown-column") == []

    def test_from_join_valid_columns_no_trigger(self):
        sql = ('UPDATE zygon."App" SET name = \'x\' '
               'FROM zygon."Account" acc '
               'JOIN zygon."Account" acc2 ON acc.id = acc2.ref '
               'WHERE "App".id = acc.ref')
        assert _diags(_ru(sql), "unknown-column") == []

    def test_from_join_typo_triggers(self):
        sql = ('UPDATE zygon."App" SET name = \'x\' '
               'FROM zygon."Account" acc '
               'JOIN zygon."Account" acc2 ON acc.id = acc2.typo '
               'WHERE "App".id = acc.ref')
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_set_rhs_from_column_no_trigger(self):
        # SET name = acc.label, RHS reads from FROM table, must be valid
        sql = 'UPDATE zygon."App" SET name = acc.label FROM zygon."Account" acc WHERE "App".id = acc.ref'
        assert _diags(_ru(sql), "unknown-column") == []

    def test_set_rhs_from_column_typo_triggers(self):
        # acc.typo doesn't exist on Account, RHS read is invalid
        sql = 'UPDATE zygon."App" SET name = acc.typo FROM zygon."Account" acc WHERE "App".id = acc.ref'
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_unqualified_where_col_found_in_target_no_trigger(self):
        # Unqualified WHERE col exists on target table, valid
        sql = 'UPDATE zygon."App" SET name = \'x\' FROM zygon."Account" acc WHERE id = acc.ref'
        assert _diags(_ru(sql), "unknown-column") == []

    def test_unqualified_where_col_not_in_any_source_triggers(self):
        # Unqualified WHERE col exists on neither target nor FROM table
        sql = 'UPDATE zygon."App" SET name = \'x\' FROM zygon."Account" acc WHERE typo = acc.ref'
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR003UpdateQuotedAndCaseColumns:
    """Quoted identifiers and case-sensitivity in UPDATE."""

    def test_quoted_set_column_valid_no_trigger(self):
        schema = {"zygon": {"App": ["id", "name", "createdAt"]}}
        sql = 'UPDATE zygon."App" SET "createdAt" = NOW() WHERE id = \'1\''
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        assert _diags(r, "unknown-column") == []

    def test_quoted_set_column_wrong_case_triggers(self):
        # "createdat" (wrong case) doesn't match "createdAt"
        schema = {"zygon": {"App": ["id", "createdAt"]}}
        sql = 'UPDATE zygon."App" SET "createdat" = NOW() WHERE id = \'1\''
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        diags = _diags(r, "unknown-column")
        assert any("createdat" in d["message"] for d in diags)

    def test_unquoted_set_column_case_insensitive_no_trigger(self):
        # Unquoted identifiers are case-folded, Name matches name
        sql = 'UPDATE zygon."App" SET Name = \'x\' WHERE id = \'1\''
        assert _diags(_ru(sql), "unknown-column") == []


class TestR003UpdateReturning:
    """RETURNING clause column validation."""

    def test_returning_valid_columns_no_trigger(self):
        sql = 'UPDATE zygon."App" SET name = \'x\' WHERE id = \'1\' RETURNING id, name'
        assert _diags(_ru(sql), "unknown-column") == []

    def test_returning_typo_triggers(self):
        sql = 'UPDATE zygon."App" SET name = \'x\' WHERE id = \'1\' RETURNING id, typo'
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_returning_star_no_trigger(self):
        sql = 'UPDATE zygon."App" SET name = \'x\' WHERE id = \'1\' RETURNING *'
        assert _diags(_ru(sql), "unknown-column") == []


class TestR003UpdateSubquery:
    """Subqueries inside UPDATE SET / WHERE, inner scope validated independently."""

    def test_subquery_in_where_inner_unknown_col_triggers(self):
        # Subquery references unknown column, should trigger via normal scope analysis
        sql = 'UPDATE zygon."App" SET name = \'x\' WHERE id IN (SELECT typo FROM "Account")'
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_subquery_in_where_valid_col_no_trigger(self):
        sql = 'UPDATE zygon."App" SET name = \'x\' WHERE id IN (SELECT ref FROM zygon."Account")'
        assert _diags(_ru(sql), "unknown-column") == []

    def test_subquery_in_set_value_no_trigger(self):
        # SET name = (SELECT label FROM ...), inner SELECT validated independently
        sql = 'UPDATE zygon."App" SET name = (SELECT label FROM zygon."Account" WHERE id = \'1\') WHERE id = \'1\''
        assert _diags(_ru(sql), "unknown-column") == []


class TestR003UpdateCTE:
    """WITH … UPDATE, CTE columns visible to the UPDATE body."""

    def test_cte_valid_column_in_subquery_no_trigger(self):
        sql = ("WITH src AS (SELECT label FROM zygon.\"Account\" WHERE id = '1') "
               "UPDATE zygon.\"App\" SET name = (SELECT label FROM src) WHERE id = '1'")
        assert _diags(_ru(sql), "unknown-column") == []

    def test_cte_typo_inside_cte_body_triggers(self):
        # typo inside CTE body, should be caught by scope analysis of the inner SELECT
        sql = ("WITH src AS (SELECT typo FROM zygon.\"Account\" WHERE id = '1') "
               "UPDATE zygon.\"App\" SET name = (SELECT label FROM src) WHERE id = '1'")
        diags = _diags(_ru(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


# ---------------------------------------------------------------------------
# R004, Unknown column in INSERT
# ---------------------------------------------------------------------------

SCHEMA_INSERT = {
    "zygon": {
        "App":     ["id", "name", "email"],
        "Account": ["id", "ref", "label"],
    }
}

def _ri(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA_INSERT, default_schema="zygon")


class TestR004InsertColumns:
    """INSERT column list validation."""

    def test_valid_columns_no_trigger(self):
        assert _diags(_ri('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\')'), "unknown-column") == []

    def test_typo_in_column_list_triggers(self):
        diags = _diags(_ri('INSERT INTO zygon."App" (id, typo) VALUES (\'1\', \'x\')'), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_multiple_columns_one_typo(self):
        diags = _diags(_ri('INSERT INTO zygon."App" (id, name, typo) VALUES (\'1\', \'x\', \'y\')'), "unknown-column")
        assert any("typo" in d["message"] for d in diags)
        assert not any("name" in d["message"] for d in diags)

    def test_no_column_list_no_trigger(self):
        # No column list, can't validate, skip
        assert _diags(_ri('INSERT INTO zygon."App" VALUES (\'1\', \'x\')'), "unknown-column") == []

    def test_unknown_table_skips_check(self):
        assert _diags(_ri('INSERT INTO zygon."NoTable" (id, typo) VALUES (\'1\', \'x\')'), "unknown-column") == []

    def test_unloaded_schema_skips_check(self):
        assert _diags(_ri('INSERT INTO other_schema."App" (id, typo) VALUES (\'1\', \'x\')'), "unknown-column") == []

    def test_empty_metadata_no_trigger(self):
        r = analyze("INSERT INTO t (typo) VALUES ('x')", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert _diags(r, "unknown-column") == []

    def test_default_schema_resolution(self):
        assert _diags(_ri('INSERT INTO "App" (id, name) VALUES (\'1\', \'x\')'), "unknown-column") == []

    def test_default_schema_typo_triggers(self):
        diags = _diags(_ri('INSERT INTO "App" (id, typo) VALUES (\'1\', \'x\')'), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_quoted_column_valid_no_trigger(self):
        schema = {"zygon": {"App": ["id", "createdAt"]}}
        sql = 'INSERT INTO zygon."App" (id, "createdAt") VALUES (\'1\', NOW())'
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        assert _diags(r, "unknown-column") == []

    def test_quoted_column_wrong_case_triggers(self):
        schema = {"zygon": {"App": ["id", "createdAt"]}}
        sql = 'INSERT INTO zygon."App" (id, "createdat") VALUES (\'1\', NOW())'
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        diags = _diags(r, "unknown-column")
        assert any("createdat" in d["message"] for d in diags)

    def test_unquoted_column_case_insensitive_no_trigger(self):
        assert _diags(_ri('INSERT INTO zygon."App" (ID, Name) VALUES (\'1\', \'x\')'), "unknown-column") == []


class TestR004InsertSelect:
    """INSERT … SELECT, SELECT body validated by scope analysis, INSERT column list validated separately."""

    def test_valid_select_no_trigger(self):
        sql = 'INSERT INTO zygon."App" (id, name) SELECT id, label FROM zygon."Account"'
        assert _diags(_ri(sql), "unknown-column") == []

    def test_typo_in_insert_col_list_triggers(self):
        sql = 'INSERT INTO zygon."App" (id, typo) SELECT id, label FROM zygon."Account"'
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_typo_in_select_body_triggers(self):
        sql = 'INSERT INTO zygon."App" (id, name) SELECT id, typo FROM zygon."Account"'
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_subquery_in_select_body_not_leaked(self):
        # Inner subquery columns must not be checked against the INSERT target table
        sql = ('INSERT INTO zygon."App" (id, name) '
               'SELECT id, label FROM zygon."Account" '
               'WHERE id IN (SELECT ref FROM zygon."Account")')
        assert _diags(_ri(sql), "unknown-column") == []

    def test_subquery_in_select_body_typo_triggers(self):
        # Typo inside the subquery should still be caught by scope analysis
        sql = ('INSERT INTO zygon."App" (id, name) '
               'SELECT id, label FROM zygon."Account" '
               'WHERE id IN (SELECT typo FROM zygon."Account")')
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR004InsertReturning:
    """RETURNING clause column validation."""

    def test_returning_valid_no_trigger(self):
        sql = 'INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') RETURNING id, name'
        assert _diags(_ri(sql), "unknown-column") == []

    def test_returning_typo_triggers(self):
        sql = 'INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') RETURNING id, typo'
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_returning_star_no_trigger(self):
        sql = 'INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') RETURNING *'
        assert _diags(_ri(sql), "unknown-column") == []


class TestR004InsertOnConflict:
    """ON CONFLICT DO UPDATE SET column validation."""

    def test_on_conflict_valid_no_trigger(self):
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name')
        assert _diags(_ri(sql), "unknown-column") == []

    def test_on_conflict_do_nothing_no_trigger(self):
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT DO NOTHING')
        assert _diags(_ri(sql), "unknown-column") == []

    def test_on_conflict_typo_in_set_lhs_triggers(self):
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (id) DO UPDATE SET typo = EXCLUDED.name')
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_on_conflict_set_rhs_plain_col_valid_no_trigger(self):
        # SET name = name, RHS references target table's current value
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (id) DO UPDATE SET name = name')
        assert _diags(_ri(sql), "unknown-column") == []

    def test_on_conflict_set_rhs_plain_col_typo_triggers(self):
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (id) DO UPDATE SET name = typo')
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_on_conflict_key_typo_triggers(self):
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (typo) DO UPDATE SET name = EXCLUDED.name')
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_on_conflict_key_quoted_valid_no_trigger(self):
        schema = {"zygon": {"App": ["id", "createdAt"]}}
        sql = ('INSERT INTO zygon."App" (id, "createdAt") VALUES (\'1\', NOW()) '
               'ON CONFLICT ("createdAt") DO UPDATE SET "createdAt" = EXCLUDED."createdAt"')
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        assert _diags(r, "unknown-column") == []

    def test_on_conflict_excluded_col_typo_triggers(self):
        # EXCLUDED mirrors the target table, EXCLUDED.typo is invalid
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.typo')
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_on_conflict_where_valid_no_trigger(self):
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name WHERE "App".id IS NOT NULL')
        assert _diags(_ri(sql), "unknown-column") == []

    def test_on_conflict_where_typo_triggers(self):
        sql = ('INSERT INTO zygon."App" (id, name) VALUES (\'1\', \'x\') '
               'ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name WHERE "App".typo IS NOT NULL')
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR004InsertValuesSubquery:
    """Subqueries inside VALUES, inner scope not reachable via traverse_scope on INSERT."""

    def test_subquery_in_values_valid_no_trigger(self):
        sql = ('INSERT INTO zygon."App" (id, name) '
               'VALUES (\'1\', (SELECT label FROM zygon."Account" WHERE ref = \'x\'))')
        assert _diags(_ri(sql), "unknown-column") == []

    def test_subquery_in_values_typo_not_caught(self):
        # sqlglot traverse_scope returns no scopes for INSERT VALUES,
        # so typos inside subqueries there are not caught, document the limitation
        sql = ('INSERT INTO zygon."App" (id, name) '
               'VALUES (\'1\', (SELECT typo FROM zygon."Account" WHERE ref = \'x\'))')
        diags = _diags(_ri(sql), "unknown-column")
        assert not any("typo" in d["message"] for d in diags)


class TestR004InsertCTE:
    """WITH … INSERT, CTE columns visible to the INSERT SELECT body."""

    def test_cte_valid_no_trigger(self):
        sql = ("WITH src AS (SELECT id, label FROM zygon.\"Account\") "
               "INSERT INTO zygon.\"App\" (id, name) SELECT id, label FROM src")
        assert _diags(_ri(sql), "unknown-column") == []

    def test_cte_typo_in_insert_col_list_triggers(self):
        sql = ("WITH src AS (SELECT id, label FROM zygon.\"Account\") "
               "INSERT INTO zygon.\"App\" (id, typo) SELECT id, label FROM src")
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_cte_typo_inside_cte_body_triggers(self):
        sql = ("WITH src AS (SELECT typo FROM zygon.\"Account\") "
               "INSERT INTO zygon.\"App\" (name) SELECT typo FROM src")
        diags = _diags(_ri(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


# ---------------------------------------------------------------------------
# R005, Unknown column in DELETE
# ---------------------------------------------------------------------------

SCHEMA_DELETE = {
    "zygon": {
        "App":     ["id", "name", "email"],
        "Account": ["id", "ref", "label"],
    }
}

def _rd(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA_DELETE, default_schema="zygon")


class TestR005DeleteWhere:
    """DELETE WHERE column validation."""

    def test_valid_where_no_trigger(self):
        assert _rd('DELETE FROM zygon."App" WHERE id = \'1\''), "unknown-column" == []

    def test_typo_in_where_triggers(self):
        diags = _diags(_rd('DELETE FROM zygon."App" WHERE typo = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_multiple_where_conditions_one_typo(self):
        diags = _diags(_rd('DELETE FROM zygon."App" WHERE id = \'1\' AND typo = \'x\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)
        assert not any("id" in d["message"] for d in diags)

    def test_unknown_table_skips_check(self):
        assert _diags(_rd('DELETE FROM zygon."NoTable" WHERE typo = \'1\''), "unknown-column") == []

    def test_unloaded_schema_skips_check(self):
        assert _diags(_rd('DELETE FROM other_schema."App" WHERE typo = \'1\''), "unknown-column") == []

    def test_empty_metadata_no_trigger(self):
        r = analyze("DELETE FROM t WHERE typo = 'x'", dialect="postgresql",
                    schema_dict={}, default_schema="public")
        assert _diags(r, "unknown-column") == []

    def test_default_schema_resolution(self):
        assert _diags(_rd('DELETE FROM "App" WHERE id = \'1\''), "unknown-column") == []

    def test_default_schema_typo_triggers(self):
        diags = _diags(_rd('DELETE FROM "App" WHERE typo = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_alias_qualified_where_valid_no_trigger(self):
        assert _diags(_rd('DELETE FROM zygon."App" a WHERE a.id = \'1\''), "unknown-column") == []

    def test_alias_qualified_where_typo_triggers(self):
        diags = _diags(_rd('DELETE FROM zygon."App" a WHERE a.typo = \'1\''), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_quoted_column_valid_no_trigger(self):
        schema = {"zygon": {"App": ["id", "createdAt"]}}
        sql = 'DELETE FROM zygon."App" WHERE "createdAt" < NOW()'
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        assert _diags(r, "unknown-column") == []

    def test_quoted_column_wrong_case_triggers(self):
        schema = {"zygon": {"App": ["id", "createdAt"]}}
        sql = 'DELETE FROM zygon."App" WHERE "createdat" < NOW()'
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        diags = _diags(r, "unknown-column")
        assert any("createdat" in d["message"] for d in diags)

    def test_unquoted_where_case_insensitive_no_trigger(self):
        assert _diags(_rd('DELETE FROM zygon."App" WHERE ID = \'1\''), "unknown-column") == []

    def test_subquery_in_where_not_leaked(self):
        # Subquery columns must not be checked against the DELETE target table
        sql = 'DELETE FROM zygon."App" WHERE id IN (SELECT ref FROM zygon."Account")'
        assert _diags(_rd(sql), "unknown-column") == []

    def test_subquery_in_where_typo_triggers(self):
        # Typo inside the subquery is caught by scope analysis
        sql = 'DELETE FROM zygon."App" WHERE id IN (SELECT typo FROM zygon."Account")'
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR005DeleteReturning:
    """RETURNING clause column validation."""

    def test_returning_valid_no_trigger(self):
        sql = 'DELETE FROM zygon."App" WHERE id = \'1\' RETURNING id, name'
        assert _diags(_rd(sql), "unknown-column") == []

    def test_returning_typo_triggers(self):
        sql = 'DELETE FROM zygon."App" WHERE id = \'1\' RETURNING id, typo'
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_returning_star_no_trigger(self):
        sql = 'DELETE FROM zygon."App" WHERE id = \'1\' RETURNING *'
        assert _diags(_rd(sql), "unknown-column") == []

    def test_returning_alias_qualified_valid_no_trigger(self):
        sql = 'DELETE FROM zygon."App" a WHERE a.id = \'1\' RETURNING a.id, a.name'
        assert _diags(_rd(sql), "unknown-column") == []

    def test_returning_alias_qualified_typo_triggers(self):
        sql = 'DELETE FROM zygon."App" a WHERE a.id = \'1\' RETURNING a.id, a.typo'
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_returning_col_from_using_table_triggers(self):
        # RETURNING only sees the deleted row; USING table columns are not in scope.
        sql = ('DELETE FROM zygon."App" USING zygon."Account" acc '
               'WHERE "App".id = acc.ref RETURNING acc.label')
        diags = _diags(_rd(sql), "unknown-column")
        assert any("label" in d["message"] for d in diags)


class TestR005DeleteUsing:
    """DELETE … USING, PostgreSQL extension, columns from joined tables validated."""

    def test_using_valid_where_no_trigger(self):
        sql = 'DELETE FROM zygon."App" USING zygon."Account" acc WHERE "App".id = acc.ref'
        assert _diags(_rd(sql), "unknown-column") == []

    def test_using_typo_in_where_triggers(self):
        sql = 'DELETE FROM zygon."App" USING zygon."Account" acc WHERE "App".id = acc.typo'
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_using_typo_on_target_triggers(self):
        sql = 'DELETE FROM zygon."App" USING zygon."Account" acc WHERE "App".typo = acc.ref'
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_using_unknown_table_where_column_skipped(self):
        sql = 'DELETE FROM zygon."App" USING unknown_table acc WHERE "App".id = acc.typo'
        assert _diags(_rd(sql), "unknown-column") == []

    def test_using_multiple_tables_valid_no_trigger(self):
        sql = ('DELETE FROM zygon."App" '
               'USING zygon."Account" acc, zygon."Account" acc2 '
               'WHERE "App".id = acc.ref AND acc.id = acc2.ref')
        assert _diags(_rd(sql), "unknown-column") == []

    def test_using_multiple_tables_one_typo_triggers(self):
        sql = ('DELETE FROM zygon."App" '
               'USING zygon."Account" acc, zygon."Account" acc2 '
               'WHERE "App".id = acc.ref AND acc.id = acc2.typo')
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_unqualified_where_col_found_in_target_no_trigger(self):
        sql = 'DELETE FROM zygon."App" USING zygon."Account" acc WHERE id = acc.ref'
        assert _diags(_rd(sql), "unknown-column") == []

    def test_unqualified_where_col_found_in_using_only_no_trigger(self):
        # label exists in Account (USING) but not in App (target), still valid unqualified
        sql = 'DELETE FROM zygon."App" USING zygon."Account" acc WHERE label = \'x\''
        assert _diags(_rd(sql), "unknown-column") == []

    def test_unqualified_where_col_not_in_any_source_triggers(self):
        sql = 'DELETE FROM zygon."App" USING zygon."Account" acc WHERE typo = acc.ref'
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR005DeleteCTE:
    """WITH … DELETE, CTE columns visible to the DELETE body."""

    def test_cte_valid_no_trigger(self):
        sql = ("WITH src AS (SELECT id FROM zygon.\"Account\") "
               "DELETE FROM zygon.\"App\" WHERE id IN (SELECT id FROM src)")
        assert _diags(_rd(sql), "unknown-column") == []

    def test_cte_typo_inside_cte_body_triggers(self):
        sql = ("WITH src AS (SELECT typo FROM zygon.\"Account\") "
               "DELETE FROM zygon.\"App\" WHERE id IN (SELECT typo FROM src)")
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_cte_typo_in_delete_where_triggers(self):
        sql = ("WITH src AS (SELECT id FROM zygon.\"Account\") "
               "DELETE FROM zygon.\"App\" WHERE typo IN (SELECT id FROM src)")
        diags = _diags(_rd(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


# ---------------------------------------------------------------------------
# R005, Dead CTE
# ---------------------------------------------------------------------------
# R006, Unknown column in MERGE
# ---------------------------------------------------------------------------

SCHEMA_MERGE = {
    "zygon": {
        "App":     ["id", "name", "email"],
        "Account": ["id", "ref", "label"],
    }
}


def _rm(sql):
    return analyze(sql, dialect="postgresql", schema_dict=SCHEMA_MERGE, default_schema="zygon")


class TestR006MergeOnCondition:
    """ON join condition, columns from both target and source validated."""

    def test_valid_on_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_typo_on_target_side_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.typo = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_typo_on_source_side_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.typo '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_unknown_target_table_skips(self):
        sql = ('MERGE INTO zygon."NoTable" t USING zygon."Account" s ON t.typo = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_unknown_source_table_on_still_validates_target(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."NoSource" s ON t.typo = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_unloaded_schema_skips(self):
        sql = ('MERGE INTO other_schema."App" t USING other_schema."Account" s ON t.typo = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_empty_metadata_no_trigger(self):
        r = analyze(
            'MERGE INTO t USING s ON t.typo = s.ref WHEN MATCHED THEN UPDATE SET t.x = s.y',
            dialect="postgresql", schema_dict={}, default_schema="public")
        assert _diags(r, "unknown-column") == []


class TestR006MergeMatchedUpdate:
    """WHEN MATCHED THEN UPDATE SET, target cols on LHS, both sources on RHS."""

    def test_valid_set_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_typo_in_set_lhs_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.typo = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_typo_in_set_rhs_source_col_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.typo')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_set_rhs_from_target_valid_no_trigger(self):
        # RHS can read current target value
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = t.email')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_set_rhs_target_col_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = t.typo')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_unqualified_set_lhs_valid_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_unqualified_set_lhs_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET typo = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_when_condition_valid_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED AND t.name IS NULL THEN UPDATE SET t.name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_when_condition_target_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED AND t.typo IS NULL THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_when_condition_source_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED AND s.typo IS NULL THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_multiple_set_assignments_one_typo(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label, t.typo = s.ref')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)
        assert not any("name" in d["message"] for d in diags)


class TestR006MergeNotMatchedInsert:
    """WHEN NOT MATCHED THEN INSERT, target cols in list, source cols in VALUES."""

    def test_valid_insert_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED THEN INSERT (id, name) VALUES (s.ref, s.label)')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_typo_in_insert_col_list_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED THEN INSERT (id, typo) VALUES (s.ref, s.label)')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_typo_in_values_source_col_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED THEN INSERT (id, name) VALUES (s.ref, s.typo)')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_valid_no_trigger_multiple_cols(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED THEN INSERT (id, name, email) VALUES (s.ref, s.label, s.label)')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_when_not_matched_condition_source_only_valid(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED AND s.label IS NOT NULL '
               'THEN INSERT (id, name) VALUES (s.ref, s.label)')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_when_not_matched_condition_source_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED AND s.typo IS NOT NULL '
               'THEN INSERT (id, name) VALUES (s.ref, s.label)')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR006MergeNotMatchedBySource:
    """WHEN NOT MATCHED BY SOURCE, only target visible (source row absent)."""

    def test_delete_no_column_refs_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED BY SOURCE THEN DELETE')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_update_target_only_valid_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED BY SOURCE THEN UPDATE SET t.name = t.email')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_update_set_lhs_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED BY SOURCE THEN UPDATE SET t.typo = t.email')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_update_set_rhs_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED BY SOURCE THEN UPDATE SET t.name = t.typo')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_condition_target_col_valid_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED BY SOURCE AND t.name IS NOT NULL THEN DELETE')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_condition_target_col_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN NOT MATCHED BY SOURCE AND t.typo IS NOT NULL THEN DELETE')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


class TestR006MergeReturning:
    """RETURNING clause, only target columns visible."""

    def test_returning_valid_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label RETURNING t.id, t.name')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_returning_typo_triggers(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label RETURNING t.id, t.typo')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_returning_source_col_triggers(self):
        # RETURNING only sees the affected target row, source cols are not in scope
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label RETURNING s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("label" in d["message"] for d in diags)

    def test_returning_star_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label RETURNING *')
        assert _diags(_rm(sql), "unknown-column") == []


class TestR006MergeQuotedColumns:
    """Quoted identifiers, case-sensitive matching."""

    def test_quoted_target_col_valid_no_trigger(self):
        schema = {"zygon": {"App": ["id", "createdAt"], "Account": ["id", "ref"]}}
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.id '
               'WHEN MATCHED THEN UPDATE SET t."createdAt" = NOW()')
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        assert _diags(r, "unknown-column") == []

    def test_quoted_target_col_wrong_case_triggers(self):
        schema = {"zygon": {"App": ["id", "createdAt"], "Account": ["id", "ref"]}}
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.id = s.id '
               'WHEN MATCHED THEN UPDATE SET t."createdat" = NOW()')
        r = analyze(sql, dialect="postgresql", schema_dict=schema, default_schema="zygon")
        diags = _diags(r, "unknown-column")
        assert any("createdat" in d["message"] for d in diags)

    def test_unquoted_col_case_insensitive_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t USING zygon."Account" s ON t.ID = s.REF '
               'WHEN MATCHED THEN UPDATE SET t.NAME = s.LABEL')
        assert _diags(_rm(sql), "unknown-column") == []


class TestR006MergeUsingSubquery:
    """USING (SELECT …), source schema unknown, source-qualified refs not validated."""

    def test_using_subquery_valid_no_trigger(self):
        sql = ('MERGE INTO zygon."App" t '
               'USING (SELECT ref, label FROM zygon."Account") s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_using_subquery_target_typo_still_triggers(self):
        sql = ('MERGE INTO zygon."App" t '
               'USING (SELECT ref, label FROM zygon."Account") s ON t.typo = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_using_subquery_source_col_not_validated(self):
        # s.anything, can't validate against subquery projection, skip
        sql = ('MERGE INTO zygon."App" t '
               'USING (SELECT ref, label FROM zygon."Account") s ON t.id = s.typo '
               'WHEN MATCHED THEN UPDATE SET t.name = s.typo')
        assert _diags(_rm(sql), "unknown-column") == []


class TestR006MergeCTE:
    """WITH … MERGE, CTE body and MERGE body validated."""

    def test_cte_valid_no_trigger(self):
        sql = ('WITH src AS (SELECT ref, label FROM zygon."Account") '
               'MERGE INTO zygon."App" t USING src s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_cte_typo_inside_cte_body_triggers(self):
        sql = ('WITH src AS (SELECT typo FROM zygon."Account") '
               'MERGE INTO zygon."App" t USING src s ON t.id = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)

    def test_cte_as_using_source_unknown_skips_source_cols(self):
        # CTE as USING source, columns are the CTE projection, not a schema table
        sql = ('WITH src AS (SELECT ref, label FROM zygon."Account") '
               'MERGE INTO zygon."App" t USING src s ON t.id = s.typo '
               'WHEN MATCHED THEN UPDATE SET t.name = s.typo')
        assert _diags(_rm(sql), "unknown-column") == []

    def test_cte_as_using_target_typo_still_triggers(self):
        sql = ('WITH src AS (SELECT ref, label FROM zygon."Account") '
               'MERGE INTO zygon."App" t USING src s ON t.typo = s.ref '
               'WHEN MATCHED THEN UPDATE SET t.name = s.label')
        diags = _diags(_rm(sql), "unknown-column")
        assert any("typo" in d["message"] for d in diags)


# ---------------------------------------------------------------------------

class TestR004DeadCTE:
    def test_unused_cte_triggers(self):
        diags = _diags(_r("WITH unused AS (SELECT 1) SELECT 1"), "unused-cte")
        assert any("unused" in d["message"].lower() for d in diags)

    def test_used_cte_no_trigger(self):
        assert _diags(_r("WITH cte AS (SELECT 1 AS x) SELECT * FROM cte"), "unused-cte") == []

    def test_no_with_clause_no_trigger(self):
        assert _diags(_r("SELECT * FROM users"), "unused-cte") == []

    def test_cte_used_inside_another_cte_no_trigger(self):
        assert _diags(_r("WITH a AS (SELECT 1), b AS (SELECT * FROM a) SELECT * FROM b"), "unused-cte") == []

    def test_only_unused_cte_triggers_when_one_used(self):
        diags = _diags(_r("WITH used AS (SELECT 1), dead AS (SELECT 2) SELECT * FROM used"), "unused-cte")
        assert any("dead" in d["message"].lower() for d in diags)
        assert not any('"used"' in d["message"] for d in diags)


# ---------------------------------------------------------------------------
# R008, Ambiguous column
# ---------------------------------------------------------------------------

class TestR008AmbiguousColumn:
    def test_unqualified_ambiguous_column_triggers(self):
        diags = _diags(_r("SELECT id FROM users u JOIN orders o ON u.id = o.user_id"), "ambiguous-column")
        assert any("id" in d["message"].lower() for d in diags)

    def test_qualified_ambiguous_column_no_trigger(self):
        assert _diags(_r("SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id"), "ambiguous-column") == []

    def test_unambiguous_column_no_trigger(self):
        diags = _diags(_r("SELECT user_id FROM users u JOIN orders o ON u.id = o.user_id"), "ambiguous-column")
        assert not any("user_id" in d["message"].lower() for d in diags)

    def test_no_join_no_ambiguity(self):
        assert _diags(_r("SELECT id FROM users"), "ambiguous-column") == []
