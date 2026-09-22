"""Tests for the caret classification in completion_context.py."""
from completion.completion_context import (
    TARGET_ALL,
    TARGET_COLUMN,
    TARGET_ENUM_VALUE,
    TARGET_SCHEMA_AND_TABLE_ALL,
    TARGET_TABLE_AND_COLUMN,
    detect_completion_context,
    _detect_keyword_context,
    _detect_setting_context,
    _detect_value_position,
    _tokenize_up_to,
)


def _at_caret(sql_with_caret: str, sg_dialect: str = "postgres"):
    """Tokenize the way production does, so these cases exercise the tokenizer
    the caller actually gets rather than sqlglot's generic one.
    """
    caret = sql_with_caret.index("|")
    sql = sql_with_caret[:caret] + sql_with_caret[caret + 1:]
    return _tokenize_up_to(sql, caret, sg_dialect), sql, caret


def _field(sql_with_caret: str, key: str, sg_dialect: str = "postgres"):
    """One field of the whole context, for the cases a token walk alone cannot
    answer."""
    _, sql, caret = _at_caret(sql_with_caret, sg_dialect)
    return detect_completion_context(sql, 1, caret, ["main"], sg_dialect)[key]


def _shared(sql_with_caret: str, sg_dialect: str = "postgres") -> bool:
    return _field(sql_with_caret, "shared_columns", sg_dialect)


def _check(sql_with_caret: str, sg_dialect: str = "postgres") -> bool:
    tokens, sql, caret = _at_caret(sql_with_caret, sg_dialect)
    return _detect_setting_context(tokens, sql, caret)


class TestMySQLSystemVariable:
    def test_double_at_alone(self):
        assert _check("SELECT @@|", "mysql")

    def test_double_at_partial(self):
        assert _check("SELECT @@vers|", "mysql")

    def test_single_at_user_variable_does_not_trigger(self):
        assert not _check("SELECT @my|", "mysql")


class TestPostgresShow:
    def test_show_alone(self):
        assert _check("SHOW |")

    def test_show_partial(self):
        assert _check("SHOW time|")

    def test_show_lowercase(self):
        assert _check("show |")


class TestSqlitePragma:
    def test_pragma_alone(self):
        assert _check("PRAGMA |", "sqlite")

    def test_pragma_partial(self):
        assert _check("PRAGMA fore|")

    def test_pragma_lowercase(self):
        assert _check("pragma |")


class TestCallParen:
    def test_inside_a_call_keeps_the_select_clause(self):
        tokens, _, _ = _at_caret("SELECT count(|) FROM t1")
        assert _detect_keyword_context(tokens) == TARGET_ALL

    def test_inside_a_call_in_where_keeps_the_where_clause(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE lower(|)")
        assert _detect_keyword_context(tokens) & TARGET_COLUMN

    def test_an_in_list_keeps_its_clause(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE c1 IN (|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_comparison_opens_a_query(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE c1 = (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_quantified_comparison_opens_a_query(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE c1 = ANY (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_word_of_the_syntax_before_a_paren_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS MATERIALIZED (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_not_materialized_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS NOT MATERIALIZED (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_materialization_hint_touching_its_paren_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS MATERIALIZED(|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_not_materialized_touching_its_paren_is_not_a_call(self):
        tokens, _, _ = _at_caret("WITH x AS NOT MATERIALIZED(|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_call_with_a_space_before_its_paren_is_still_a_call(self):
        tokens, _, _ = _at_caret("SELECT count (|) FROM t1")
        assert _detect_keyword_context(tokens) == TARGET_ALL

    def test_a_grouping_paren_keeps_its_clause(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE (|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_window_spec_keeps_its_clause(self):
        tokens, _, _ = _at_caret("SELECT row_number() OVER (|) FROM t1")
        assert _detect_keyword_context(tokens) == TARGET_ALL

    def test_a_grouping_set_keeps_its_clause(self):
        tokens, _, _ = _at_caret("SELECT c1 FROM t1 GROUP BY GROUPING SETS ((|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_subquery_in_from_opens_a_query(self):
        tokens, _, _ = _at_caret("SELECT * FROM (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_exists_opens_a_query(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 WHERE EXISTS (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_paren_at_the_start_opens_a_query(self):
        tokens, _, _ = _at_caret("(|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_plain_cte_body_is_a_nested_query(self):
        tokens, _, _ = _at_caret("WITH x AS (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL


class TestRowCount:
    def test_limit_offers_no_relation(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 LIMIT |")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_offset_offers_no_relation(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 LIMIT 10 OFFSET |")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN


class TestContextualWords:
    def test_a_join_using_names_columns(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 JOIN t2 USING (|")
        assert _detect_keyword_context(tokens) == TARGET_COLUMN

    def test_a_delete_using_names_a_relation(self):
        tokens, _, _ = _at_caret("DELETE FROM t1 USING |")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_merge_using_names_a_relation(self):
        tokens, _, _ = _at_caret("MERGE INTO t1 USING |")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_join_over_a_subquery_still_names_columns(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 JOIN (SELECT 1 AS i) s ON |")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_join_does_not_reach_across_a_statement(self):
        tokens, _, _ = _at_caret("SELECT * FROM a JOIN b ON a.i=b.i; CREATE INDEX i ON |")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_later_window_definition_is_not_a_cte_body(self):
        tokens, _, _ = _at_caret("SELECT c1 FROM t1 WINDOW w AS (ORDER BY c1), v AS (|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_derived_table_rename_is_not_a_query_body(self):
        tokens, _, _ = _at_caret("SELECT * FROM (SELECT 1) s (|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_table_function_rename_is_not_a_query_body(self):
        tokens, _, _ = _at_caret("SELECT * FROM generate_series(1,2) g (|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_later_cte_body_is_still_one(self):
        tokens, _, _ = _at_caret("WITH w AS (SELECT 1), v AS (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_mysql_upsert_names_columns(self):
        tokens, _, _ = _at_caret(
            "INSERT INTO t1 (c1) VALUES (1) ON DUPLICATE KEY UPDATE |", "mysql")
        assert _detect_keyword_context(tokens) == TARGET_COLUMN

    def test_a_merge_on_names_columns(self):
        tokens, _, _ = _at_caret("MERGE INTO t1 USING t2 ON |")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_grant_on_names_a_relation(self):
        tokens, _, _ = _at_caret("GRANT SELECT ON |")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_conflict_target_names_columns(self):
        tokens, _, _ = _at_caret("INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (|")
        assert _detect_keyword_context(tokens) == TARGET_COLUMN

    def test_fetch_first_is_a_row_count(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 FETCH FIRST |")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_first_alone_is_not_a_row_count(self):
        sql = "INSERT INTO t1 (first, "
        targets = detect_completion_context(sql, 1, len(sql), ["main"], "postgres")["targets"]
        assert targets == TARGET_COLUMN

    def test_a_window_definition_is_not_a_cte_body(self):
        tokens, _, _ = _at_caret("SELECT c1 FROM t1 WINDOW w AS (|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_cte_body_is_still_one(self):
        tokens, _, _ = _at_caret("WITH w AS (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_returning_names_columns(self):
        tokens, _, _ = _at_caret("DELETE FROM t1 RETURNING |")
        assert _detect_keyword_context(tokens) == TARGET_COLUMN


class TestInsideACall:
    def test_a_table_function_takes_values(self):
        tokens, _, _ = _at_caret("SELECT * FROM generate_series(|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_joined_table_function_takes_values(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 JOIN generate_series(|")
        assert _detect_keyword_context(tokens) == TARGET_TABLE_AND_COLUMN

    def test_a_relation_after_a_comma_is_still_a_relation(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1, (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_lateral_is_still_a_relation(self):
        tokens, _, _ = _at_caret("SELECT * FROM t1 JOIN LATERAL (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_merge_source_is_still_a_relation(self):
        tokens, _, _ = _at_caret("MERGE INTO t1 USING (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_delete_source_is_still_a_relation(self):
        tokens, _, _ = _at_caret("DELETE FROM t1 USING (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_from_clause_itself_still_takes_relations(self):
        tokens, _, _ = _at_caret("SELECT * FROM |")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL

    def test_a_subquery_in_from_still_takes_relations(self):
        tokens, _, _ = _at_caret("SELECT * FROM (|")
        assert _detect_keyword_context(tokens) == TARGET_SCHEMA_AND_TABLE_ALL


class TestColumnListRelation:
    def _relation(self, sql_with_caret: str) -> str:
        return _field(sql_with_caret, "column_list_relation")

    def test_a_rename_with_as(self):
        assert self._relation("SELECT * FROM t1 AS a (|") == "t1"

    def test_a_rename_without_as(self):
        assert self._relation("SELECT * FROM t1 a (|") == "t1"

    def test_a_rename_of_a_joined_relation(self):
        assert self._relation("SELECT * FROM t1 JOIN t2 b (|") == "t2"

    def test_a_qualified_relation(self):
        assert self._relation("SELECT * FROM main.t1 AS a (|") == "t1"

    def test_a_relation_after_a_comma(self):
        assert self._relation("SELECT * FROM t1, t2 b (|") == "t2"

    def test_a_relation_a_delete_uses(self):
        assert self._relation("DELETE FROM t1 USING t2 AS b (|") == "t2"

    def test_a_relation_an_update_reads(self):
        assert self._relation("UPDATE t1 SET c1 = 1 FROM t2 b (|") == "t2"

    def test_an_upsert_alias(self):
        assert self._relation("INSERT INTO t1 AS a (|") == "t1"

    def test_a_derived_table_has_no_name_to_give(self):
        assert self._relation("SELECT * FROM (SELECT 1) s (|") == ""

    def test_a_cte_body_is_not_a_rename(self):
        assert self._relation("WITH x AS (|") == ""

    def test_a_materialization_hint_is_not_a_rename(self):
        assert self._relation("WITH x AS MATERIALIZED (|") == ""

    def test_an_insert_column_list_still_reads(self):
        assert self._relation("INSERT INTO t1 (|") == "t1"


class TestSharedColumns:
    def test_a_join_using_list(self):
        assert _shared("SELECT * FROM t1 JOIN t2 USING (|")

    def test_a_later_name_in_the_list(self):
        assert _shared("SELECT * FROM t1 JOIN t2 USING (c1, |")

    def test_a_join_over_a_subquery(self):
        assert _shared("SELECT * FROM t1 JOIN (SELECT 1) s USING (|")

    def test_a_delete_using_is_not_one(self):
        assert not _shared("DELETE FROM t1 USING |")

    def test_a_merge_using_is_not_one(self):
        assert not _shared("MERGE INTO t1 USING |")

    def test_an_insert_column_list_is_not_one(self):
        assert not _shared("INSERT INTO t1 (|")

    def test_a_join_on_is_not_one(self):
        assert not _shared("SELECT * FROM t1 JOIN t2 ON |")

    def test_a_qualified_caret_is_not_one(self):
        assert not _shared("SELECT * FROM t1 JOIN t2 USING (t1.|")


class TestValuePosition:
    def test_inside_a_literal_is_quoted(self):
        tokens, _, caret = _at_caret("UPDATE t SET c1 = '|'")
        assert _detect_value_position(tokens, caret).quoted

    def test_after_an_equals_is_not_quoted(self):
        tokens, _, caret = _at_caret("UPDATE t SET c1 = |")
        assert not _detect_value_position(tokens, caret).quoted

    def test_inside_an_in_list_is_quoted(self):
        tokens, _, caret = _at_caret("SELECT * FROM t WHERE c1 IN ('|')")
        assert _detect_value_position(tokens, caret).quoted

    def test_an_in_list_is_a_value_slot(self):
        tokens, _, caret = _at_caret("SELECT * FROM t WHERE c1 IN (|")
        assert _detect_value_position(tokens, caret).column == {"name": "c1"}

    def test_a_typed_prefix_is_still_a_value_slot(self):
        tokens, _, caret = _at_caret("SELECT * FROM t WHERE c1 = ac|")
        assert _detect_value_position(tokens, caret).column == {"name": "c1"}

    def test_a_typed_prefix_in_an_in_list_is_still_a_value_slot(self):
        tokens, _, caret = _at_caret("SELECT * FROM t WHERE c1 IN (ac|")
        assert _detect_value_position(tokens, caret).column == {"name": "c1"}

    def test_a_finished_word_is_not_a_typed_prefix(self):
        tokens, _, caret = _at_caret("SELECT * FROM t WHERE c1 = ac |")
        assert _detect_value_position(tokens, caret).column is None

    def test_an_in_list_offers_its_clause_as_well_as_values(self):
        sql = "SELECT * FROM t1 WHERE c1 IN ("
        targets = detect_completion_context(sql, 1, len(sql), ["main"], "postgres")["targets"]
        assert targets == TARGET_ENUM_VALUE | TARGET_TABLE_AND_COLUMN

    def test_inside_a_literal_offers_values_alone(self):
        sql = "SELECT * FROM t1 WHERE c1 IN ('"
        targets = detect_completion_context(sql, 1, len(sql), ["main"], "postgres")["targets"]
        assert targets == TARGET_ENUM_VALUE


class TestNonTriggers:
    def test_select_keyword(self):
        assert not _check("SELECT |")

    def test_from_clause(self):
        assert not _check("SELECT * FROM |")

    def test_where_clause(self):
        assert not _check("SELECT * FROM t WHERE |")

    def test_update_set_does_not_trigger(self):
        assert not _check("UPDATE t SET |")

    def test_empty_sql(self):
        assert not _check("|")
