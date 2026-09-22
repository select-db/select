"""Tests for the caret classification in completion_context.py."""
from completion.completion_context import (
    TARGET_ALL,
    TARGET_COLUMN,
    TARGET_ENUM_VALUE,
    TARGET_FUNCTION,
    TARGET_KEYWORD,
    TARGET_OPERATOR,
    TARGET_SCHEMA_AND_TABLE_ALL,
    TARGET_TABLE_AND_COLUMN,
    TARGET_TYPE,
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
    # The caller passes a line and a column, so a case spanning lines has to
    # be asked about where it actually is.
    before = sql[:caret]
    line = before.count("\n") + 1
    column = caret - (before.rfind("\n") + 1)
    return detect_completion_context(sql, line, column, ["main"], sg_dialect)[key]


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


class TestStatementStart:
    def _targets(self, sql_with_caret: str) -> int:
        return _field(sql_with_caret, "targets")

    def test_an_empty_buffer(self):
        assert self._targets("|") == TARGET_KEYWORD

    def test_whitespace_alone(self):
        assert self._targets("   |") == TARGET_KEYWORD

    def test_after_a_semicolon(self):
        assert self._targets("SELECT 1; |") == TARGET_KEYWORD

    def test_after_a_comment(self):
        assert self._targets("-- a note\n|") == TARGET_KEYWORD

    def test_a_typed_opener_the_tokenizer_knows_is_still_one(self):
        assert self._targets("CREATE|") == TARGET_KEYWORD
        assert self._targets("SELECT|") == TARGET_KEYWORD

    def test_an_unclosed_quote_is_not_an_empty_buffer(self):
        assert self._targets('SELECT "S|') != TARGET_KEYWORD

    def test_an_unterminated_block_comment_does_not_raise(self):
        assert self._targets("/* a note|") == 0

    def test_a_select_list_is_not_one(self):
        assert self._targets("SELECT |") != TARGET_KEYWORD

    def test_a_from_clause_is_not_one(self):
        assert self._targets("SELECT * FROM |") != TARGET_KEYWORD

    def test_the_opening_word_being_typed_is_still_one(self):
        assert self._targets("SEL|") == TARGET_KEYWORD

    def test_one_letter_is_still_one(self):
        assert self._targets("S|") == TARGET_KEYWORD

    def test_a_typed_opener_after_a_semicolon(self):
        assert self._targets("SELECT 1; SEL|") == TARGET_KEYWORD

    def test_a_finished_opener_is_not_one(self):
        assert self._targets("SELECT |") != TARGET_KEYWORD

    def test_a_column_being_typed_is_not_one(self):
        assert self._targets("SELECT c|") != TARGET_KEYWORD


class TestExpressionPositions:
    def _takes_a_call(self, sql_with_caret: str) -> bool:
        return bool(_field(sql_with_caret, "targets") & TARGET_FUNCTION)

    def test_a_select_list(self):
        assert self._takes_a_call("SELECT |")

    def test_a_where(self):
        assert self._takes_a_call("SELECT * FROM t1 WHERE |")

    def test_an_assignment_value(self):
        assert self._takes_a_call("UPDATE t1 SET c1 = |")

    def test_a_values_row(self):
        assert self._takes_a_call("INSERT INTO t1 VALUES (|")

    def test_an_assignment_target_does_not(self):
        assert not self._takes_a_call("UPDATE t1 SET |")

    def test_an_insert_column_list_does_not(self):
        assert not self._takes_a_call("INSERT INTO t1 (|")

    def test_a_using_list_does_not(self):
        assert not self._takes_a_call("SELECT * FROM t1 JOIN t2 USING (|")

    def test_a_qualified_caret_does_not(self):
        assert not self._takes_a_call("SELECT t1.|")

    def test_a_from_clause_does_not(self):
        assert not self._takes_a_call("SELECT * FROM |")


class TestTypePositions:
    def _targets(self, sql_with_caret: str) -> int:
        return _field(sql_with_caret, "targets")

    def test_a_cast_takes_a_type(self):
        assert self._targets("SELECT CAST(c1 AS |") == TARGET_TYPE
        assert self._targets("SELECT CAST(count(c1) AS |") == TARGET_TYPE
        assert self._targets("SELECT * FROM t1 WHERE CAST(c1 AS |") == TARGET_TYPE

    def test_the_shorthand_cast_too(self):
        assert self._targets("SELECT c1::|") == TARGET_TYPE

    def test_an_alias_is_not_a_type(self):
        assert self._targets("SELECT c1 AS |") != TARGET_TYPE
        assert self._targets("SELECT * FROM t1 AS |") != TARGET_TYPE
        assert self._targets("WITH x AS |") != TARGET_TYPE
        assert self._targets("SELECT count(c1) AS |") != TARGET_TYPE


class TestClauseFollowers:
    def _group(self, sql_with_caret: str) -> str:
        return _field(sql_with_caret, "keyword_group")

    def test_a_finished_select_item(self):
        assert self._group("SELECT c1 |") == "select_item"

    def test_a_finished_relation(self):
        assert self._group("SELECT * FROM t1 |") == "relation"

    def test_a_joined_relation_allows_on(self):
        assert self._group("SELECT * FROM t1 JOIN t2 |") == "joined_relation"

    def test_an_alias_follows_the_item_it_renames_without_a_second_as(self):
        assert self._group("SELECT * FROM t1 AS a |") == "aliased_relation"
        assert self._group("SELECT c1 AS x |") == "aliased_select_item"

    def test_an_alias_written_without_as_is_still_one(self):
        assert self._group("SELECT * FROM t1 a |") == "aliased_relation"
        assert self._group("SELECT c1 x |") == "aliased_select_item"
        assert self._group("SELECT * FROM main.t1 a |") == "aliased_relation"
        assert self._group("SELECT * FROM (SELECT 1) s |") == "aliased_relation"

    def test_a_qualified_name_is_not_its_own_alias(self):
        assert self._group("SELECT * FROM main.t1 |") == "relation"

    def test_a_join_word_waits_for_join(self):
        for sql in ("SELECT * FROM t1 LEFT |", "SELECT * FROM t1 CROSS |",
                    "SELECT * FROM t1 LEFT OUTER |", "SELECT * FROM t1 NATURAL |"):
            assert self._group(sql) == "join_word", sql

    def test_is_and_not_wait_for_what_they_test(self):
        assert self._group("SELECT * FROM t1 WHERE c1 IS |") == "is_test"
        assert self._group("SELECT * FROM t1 WHERE c1 IS NOT |") == "is_not_test"
        assert self._group("SELECT * FROM t1 WHERE c1 NOT |") == "not_test"

    def test_a_not_opening_a_predicate_opens_an_item(self):
        assert self._group("SELECT * FROM t1 WHERE NOT |") == "expression_start"
        assert self._group("SELECT NOT |") == "expression_start"

    def test_a_set_operation_waits_for_a_query(self):
        assert self._group("SELECT 1 UNION |") == "set_operand"
        assert self._group("SELECT 1 EXCEPT |") == "set_operand"
        assert self._group("SELECT 1 UNION ALL |") == "query_word"

    def test_a_select_list_quantifier_is_not_a_set_operand(self):
        assert self._group("SELECT ALL |") == "expression_start"
        assert self._group("SELECT DISTINCT |") == "expression_start"

    def test_an_item_opens_with_a_word_as_well_as_a_name(self):
        assert self._group("SELECT |") == "select_start"
        assert self._group("SELECT c1, |") == "expression_start"
        assert self._group("SELECT * FROM t1 WHERE |") == "expression_start"
        assert self._group("INSERT INTO t1 VALUES (|") == "expression_start"

    def test_those_words_stand_beside_the_names(self):
        targets = _field("SELECT * FROM t1 WHERE |", "targets")
        assert targets & TARGET_KEYWORD
        assert targets & TARGET_COLUMN

    def test_a_name_being_named_takes_no_word(self):
        assert self._group("INSERT INTO t1 (|") == ""
        assert self._group("SELECT * FROM t1 JOIN t2 USING (|") == ""
        assert self._group("UPDATE t1 SET |") == ""

    def test_a_relation_is_not_an_expression(self):
        assert self._group("SELECT * FROM |") == ""
        assert self._group("SELECT * FROM t1, |") == ""

    def test_a_distinct_on_list_is_not_a_join(self):
        assert self._group("SELECT DISTINCT ON (c1) |") == "expression_start"
        assert self._group("SELECT DISTINCT ON (c1) c2 |") == "select_item"
        assert self._group("SELECT * FROM t1 JOIN t2 ON (c1 = c2) |") == "predicate"

    def test_a_clause_on_its_own_line_is_still_one(self):
        assert self._group("SELECT *\nFROM t1 |") == "relation"
        assert self._group("SELECT *\r\nFROM t1 |") == "relation"
        assert self._group("SELECT *\n\tFROM t1 |") == "relation"

    def test_a_quoted_name_is_still_a_name(self):
        assert self._group('SELECT * FROM "t1" |') == "relation"
        assert self._group('SELECT * FROM t1 "a" |') == "aliased_relation"
        assert self._group('SELECT "c1" |') == "select_item"

    def test_an_upsert_names_what_to_do(self):
        assert self._group("INSERT INTO t1 (c1) VALUES (1) ON CONFLICT |") == "conflict_action"
        assert self._group("INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) |") == "conflict_do"
        assert self._group("INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO |") == "conflict_resolution"

    def test_nulls_waits_for_where_they_go(self):
        assert self._group("SELECT * FROM t1 ORDER BY c1 DESC NULLS |") == "null_ordering"

    def test_a_window_spec_is_not_a_query_clause(self):
        assert self._group("SELECT row_number() OVER (|) FROM t1") == "window_start"
        assert self._group("SELECT * FROM t1 WINDOW w AS (|)") == "window_start"
        assert self._group("SELECT row_number() OVER (PARTITION BY c1 |)") == "partition_item"
        assert self._group("SELECT row_number() OVER (ORDER BY c1 |)") == "window_sort_item"
        assert self._group("SELECT * FROM t1 WINDOW w AS (PARTITION BY c1 |)") == "partition_item"

    def test_a_query_ordering_is_not_a_window_one(self):
        assert self._group("SELECT * FROM t1 ORDER BY c1 |") == "sort_item"

    def test_a_window_item_being_written_opens_normally(self):
        assert self._group("SELECT row_number() OVER (PARTITION BY |)") == "expression_start"
        assert self._group("SELECT row_number() OVER (ORDER BY |)") == "expression_start"

    def test_a_lock_names_its_strength(self):
        assert self._group("SELECT * FROM t1 FOR |") == "lock_strength"

    def test_a_ddl_statement_waits_for_what_it_acts_on(self):
        assert self._group("CREATE |") == "object_kind"
        assert self._group("DROP |") == "object_kind"
        assert self._group("ALTER |") == "object_kind"

    def test_an_insert_on_names_a_conflict(self):
        assert self._group("INSERT INTO t1 (c1) VALUES (1) ON |") == "conflict_target"

    def test_a_join_on_still_takes_a_predicate(self):
        assert self._group("SELECT * FROM t1 JOIN t2 ON |") == "expression_start"

    def test_a_write_statement_waits_for_its_word(self):
        assert self._group("INSERT INTO t1 |") == "insert_target"
        assert self._group("UPDATE t1 |") == "update_target"
        assert self._group("DELETE |") == "delete_target"

    def test_a_clause_is_named_for_its_statement(self):
        assert self._group("DELETE FROM t1 |") == "delete_relation"
        assert self._group("DELETE FROM t1 a |") == "delete_aliased_relation"
        assert self._group("DELETE FROM t1 WHERE c1 = 1 |") == "delete_predicate"
        assert self._group("UPDATE t1 SET c1 = 1 WHERE c1 = 2 |") == "update_predicate"

    def test_a_subquery_is_named_for_its_own_statement(self):
        sql = "DELETE FROM t1 WHERE c1 IN (SELECT c1 FROM t2 WHERE c2 = 1 |"
        assert self._group(sql) == "predicate"

    def test_a_case_names_its_arms(self):
        assert self._group("SELECT CASE WHEN c1 = 1 |") == "case_test"
        assert self._group("SELECT CASE WHEN c1 = 1 THEN 2 |") == "case_body"

    def test_a_closed_case_is_one_finished_item(self):
        assert self._group("SELECT CASE WHEN c1 = 1 THEN 2 END |") == "select_item"
        assert self._group("SELECT CASE WHEN c1 = 1 THEN 2 END AS x |") == "aliased_select_item"

    def test_a_nested_case_closes_only_its_own(self):
        sql = "SELECT CASE WHEN c1 = 1 THEN CASE WHEN c2 = 2 THEN 1 END |"
        assert self._group(sql) == "case_body"

    def test_a_rename_list_is_an_alias_too(self):
        assert self._group("SELECT * FROM t1 t(a, b) |") == "aliased_relation"
        assert self._group("SELECT * FROM (SELECT 1) s(a) |") == "aliased_relation"

    def test_a_call_is_not_a_rename_list(self):
        assert self._group("SELECT count(c1) |") == "select_item"

    def test_a_sort_direction_finishes_the_item(self):
        assert self._group("SELECT * FROM t1 ORDER BY c1 ASC |") == "sort_item"

    def test_a_comment_holds_no_sql(self):
        assert _field("SELECT 1 /* x|", "targets") == 0
        assert _field("SELECT -- x|", "targets") == 0

    def test_a_closed_comment_leaves_the_clause_as_it_was(self):
        assert self._group("SELECT c1 /* x */ |") == "select_item"

    def test_a_comment_marker_inside_a_literal_is_not_one(self):
        assert _field("SELECT * FROM t1 WHERE c1 = '--a|", "targets") != 0

    def test_an_operator_stands_beside_the_words(self):
        targets = _field("SELECT * FROM t1 ORDER BY c1 |", "targets")
        assert targets & TARGET_KEYWORD
        assert targets & TARGET_OPERATOR

    def test_a_defined_cte_takes_its_statement(self):
        assert self._group("WITH x AS (SELECT 1) |") == "after_cte"
        assert self._group("WITH x AS (SELECT 1), y AS (SELECT 2) |") == "after_cte"

    def test_a_finished_predicate(self):
        assert self._group("SELECT * FROM t1 WHERE c1 = 1 |") == "predicate"
        assert self._group("SELECT * FROM t1 JOIN t2 ON t1.c1 = t2.c1 |") == "predicate"

    def test_a_half_written_predicate_is_not_one(self):
        assert self._group("SELECT * FROM t1 WHERE c1 |") == ""

    def test_an_empty_clause_opens_an_item_instead(self):
        assert self._group("SELECT |") == "select_start"
        assert self._group("SELECT * FROM |") == ""
        assert self._group("SELECT * FROM t1, |") == ""

    def test_a_follower_being_typed_is_still_one(self):
        assert self._group("SELECT * FROM t1 WHERE c1 = 1 AN|") == "predicate"
        assert self._group("SELECT * FROM t1 W|") == "relation"
        assert self._group("SELECT c1 F|") == "select_item"

    def test_a_name_being_typed_in_a_clause_is_not_a_finished_item(self):
        assert self._group("SELECT c|") == "select_start"
        assert self._group("SELECT c1, c|") == "expression_start"
        assert self._group("SELECT * FROM t|") == ""


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
        assert targets == (TARGET_ENUM_VALUE | TARGET_TABLE_AND_COLUMN
                           | TARGET_FUNCTION | TARGET_KEYWORD)

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
