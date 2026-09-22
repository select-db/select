package core

import "strings"

// CompletionTestCase represents a single completion test case
type CompletionTestCase struct {
	Name     string
	SQL      string
	Expected []CompletionTestExpectation
}

// CompletionTestExpectation represents an expected candidate in a test case
type CompletionTestExpectation struct {
	Type CandidateType
	Text string
}

// quoteIdentifiersInText replaces the ~ placeholder with the dialect's
// identifier quote, so one case covers quoting in every dialect rather than
// naming one dialect's spelling and skipping the rest.
func quoteIdentifiersInText(text, identifierQuote string) string {
	return strings.ReplaceAll(text, "~", identifierQuote)
}

// replaceSchemaInText replaces "main" with the target schema in text
func replaceSchemaInText(text, defaultSchema string) string {
	if defaultSchema == "main" {
		return text // No replacement needed
	}
	return strings.ReplaceAll(text, "main", defaultSchema)
}

// GetCompletionTestMetadata returns the standard metadata used for completion tests
func GetCompletionTestMetadata() Metadata {
	return Metadata{
		DefaultSchema: "main",
		CurrentSchema: "", // Empty means it will fall back to DefaultSchema
		Schemas: []Schema{
			{
				Name: "main",
				Tables: []Table{
					{
						Name: "t1",
						Columns: []Column{
							{Name: "c1", Type: "INTEGER"},
							{Name: "c2", Type: "TEXT"},
						},
					},
					{
						Name: "t2",
						Columns: []Column{
							{Name: "c1", Type: "INTEGER"},
							{Name: "c3", Type: "TEXT"},
						},
					},
				},
			},
		},
	}
}

// GetCompletionTestCases returns the standard completion test cases.
// defaultSchema replaces "main" schema references in SQL and expected outputs,
// and identifierQuote replaces the ~ placeholder around quoted identifiers.
func GetCompletionTestCases(defaultSchema, identifierQuote string) []CompletionTestCase {
	return []CompletionTestCase{
		{
			Name: "SELECT without FROM",
			SQL:  "SELECT | ",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "unqualified column completion",
			SQL:  "SELECT | FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "qualified table columns",
			SQL:  "SELECT t1.| FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "qualified table columns without FROM",
			SQL:  "SELECT t1.|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "quoted table columns",
			SQL:  quoteIdentifiersInText("SELECT ~t1~.| FROM ~t1~", identifierQuote),
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "quoted table columns without FROM",
			SQL:  quoteIdentifiersInText("SELECT ~t1~.|", identifierQuote),
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "unqualified column completion with alias",
			SQL:  "SELECT | FROM t1 tt1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "tt1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "table alias columns",
			SQL:  "SELECT tt1.| FROM t1 tt1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "FROM clause completion",
			SQL:  "SELECT * FROM |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "WHERE clause completion",
			SQL:  "SELECT * FROM t1 WHERE |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "ORDER BY completion",
			SQL:  "SELECT * FROM t1 ORDER BY |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "JOIN clause completion",
			SQL:  "SELECT * FROM t1 JOIN |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "ON clause completion",
			SQL:  "SELECT * FROM t1 JOIN t2 ON |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
				{Type: CandidateTypeColumn, Text: "t1.c1"},
				{Type: CandidateTypeColumn, Text: "t2.c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
				{Type: CandidateTypeColumn, Text: "c3"},
			},
		},
		{
			Name: "schema-qualified table completion",
			SQL:  replaceSchemaInText("SELECT main.| FROM main.t1", defaultSchema),
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
			},
		},
		{
			Name: "schema-qualified column completion",
			SQL:  replaceSchemaInText("SELECT main.t1.| FROM main.t1", defaultSchema),
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "GROUP BY completion",
			SQL:  "SELECT * FROM t1 GROUP BY |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "HAVING clause completion",
			SQL:  "SELECT * FROM t1 GROUP BY c1 HAVING |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "CTE completion in WITH clause",
			SQL:  "WITH cte1 AS (SELECT * FROM t1) SELECT * FROM |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "cte1"},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "CTE column completion",
			SQL:  "WITH cte1 AS (SELECT * FROM t1) SELECT cte1.| FROM cte1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "CTE column completion with explicit column list",
			SQL:  "WITH cte1 AS (SELECT c1 FROM t1) SELECT cte1.| FROM cte1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
			},
		},
		{
			Name: "CTE column completion with alias",
			SQL:  "WITH cte1 AS (SELECT t.c1 FROM t1 t) SELECT cte1.| FROM cte1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
			},
		},
		{
			Name: "multiple JOIN completion",
			SQL:  "SELECT * FROM t1 JOIN t2 ON t1.c1 = t2.c1 JOIN |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "UPDATE SET completion",
			SQL:  "UPDATE t1 SET |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "UPDATE WHERE completion",
			SQL:  "UPDATE t1 SET c1 = 1 WHERE |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "INSERT INTO completion",
			SQL:  "INSERT INTO |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "multiple CTEs completion",
			SQL:  "WITH cte1 AS (SELECT * FROM t1), cte2 AS (SELECT * FROM t2) SELECT * FROM |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "cte1"},
				{Type: CandidateTypeTable, Text: "cte2"},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "UNION SELECT completion",
			SQL:  "SELECT * FROM t1 UNION; SELECT |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "subquery in FROM clause",
			SQL:  "SELECT * FROM (SELECT * FROM t1) AS subq JOIN |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "subq"},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "subquery column completion",
			SQL:  "SELECT * FROM (SELECT * FROM t1) AS subq WHERE subq.|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "correlated subquery in WHERE",
			SQL:  "SELECT * FROM t1 WHERE EXISTS (SELECT * FROM |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "subquery in WHERE IN clause",
			SQL:  "SELECT * FROM t1 WHERE c1 IN (SELECT | FROM t2)",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "nested subquery completion",
			SQL:  "SELECT * FROM (SELECT * FROM (SELECT * FROM t1) AS t_inner) AS t_outer WHERE |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t_outer"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "DELETE target completion",
			SQL:  "DELETE FROM |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "DELETE WHERE completion",
			SQL:  "DELETE FROM t1 WHERE |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "INSERT column list completion",
			SQL:  "INSERT INTO t1 (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "inside a function call",
			SQL:  "SELECT count(|) FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "UPDATE SET value completion",
			SQL:  "UPDATE t1 SET c1 = |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "inside a call in a WHERE",
			SQL:  "SELECT * FROM t1 WHERE lower(|)",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a later argument of a call",
			SQL:  "SELECT substr(c1, 1, |) FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a CTE body is still a nested query",
			SQL:  "WITH x AS (SELECT | FROM t1) SELECT c1 FROM x",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a call with a space before its paren",
			SQL:  "SELECT count (|) FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a materialization hint is not a call",
			SQL:  "WITH x AS MATERIALIZED (SELECT | FROM t1) SELECT c1 FROM x",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "an IN list offers values and its clause, not every relation",
			SQL:  "SELECT * FROM t1 WHERE c2 IN (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a scalar subquery after a comparison",
			SQL:  "SELECT * FROM t1 WHERE c1 = (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "a grouping paren in a WHERE",
			SQL:  "SELECT * FROM t1 WHERE (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a grouping paren in the select list",
			SQL:  "SELECT (| FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a window spec is not a nested query",
			SQL:  "SELECT row_number() OVER (|) FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a grouping set is not a nested query",
			SQL:  "SELECT c1 FROM t1 GROUP BY GROUPING SETS ((|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a subquery in FROM still opens a query",
			SQL:  "SELECT * FROM (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "EXISTS still opens a query",
			SQL:  "SELECT * FROM t1 WHERE EXISTS (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
			},
		},
		{
			Name: "a row count is not a relation slot",
			SQL:  "SELECT * FROM t1 LIMIT |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "RETURNING names the columns of the row",
			SQL:  "DELETE FROM t1 RETURNING |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a fetch row count is not a relation slot",
			SQL:  "SELECT * FROM t1 FETCH FIRST |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a window definition is not a CTE body",
			SQL:  "SELECT c1 FROM t1 WINDOW w AS (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "EXPLAIN does not hide the statement under it",
			SQL:  "EXPLAIN SELECT | FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a join over a subquery still names a join column",
			SQL:  "SELECT * FROM t1 JOIN (SELECT c1 FROM t2) s ON |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "s"},
				{Type: CandidateTypeColumn, Text: "t1.c1"},
				{Type: CandidateTypeColumn, Text: "s.c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a later window definition is not a CTE body",
			SQL:  "SELECT c1 FROM t1 WINDOW w AS (ORDER BY c1), v AS (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "EXPLAIN does not hide a WHERE either",
			SQL:  "EXPLAIN SELECT c1 FROM t1 WHERE |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a join's USING list takes only a shared bare name",
			SQL:  "SELECT * FROM t1 JOIN t2 USING (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
			},
		},
		{
			Name: "a third USING joins onto what the first two made",
			SQL:  "WITH x AS (SELECT c2, c3 FROM t1) SELECT * FROM t1 JOIN t2 USING (c1) JOIN x USING (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c2"},
				{Type: CandidateTypeColumn, Text: "c3"},
			},
		},
		{
			Name: "a USING list with a side nobody knows still offers the other",
			SQL:  "SELECT * FROM t1 JOIN nowhere USING (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a table function's arguments are values, not relations",
			SQL:  "SELECT * FROM generate_series(|",
			Expected: []CompletionTestExpectation{},
		},
		{
			Name: "a table function joined to a table reads its columns",
			SQL:  "SELECT * FROM t1 JOIN generate_series(|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a CTE counted once is not an ambiguous column",
			SQL:  "WITH x AS (SELECT c2 FROM t1) SELECT | FROM x, t2",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "x"},
				{Type: CandidateTypeTable, Text: "t2"},
				{Type: CandidateTypeColumn, Text: "c2"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c3"},
			},
		},
		{
			Name: "DISTINCT select list completion",
			SQL:  "SELECT DISTINCT | FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeSchema, Text: defaultSchema},
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
	}
}

// GetCompletionCasesPostgreSQL returns the cases only PostgreSQL accepts: a
// column list that renames a relation's columns, which MySQL and SQLite have
// no syntax for.
func GetCompletionCasesPostgreSQL(defaultSchema, identifierQuote string) []CompletionTestCase {
	return []CompletionTestCase{
		{
			Name: "a column list renaming a relation names that relation's columns",
			SQL:  "SELECT * FROM t1 AS a (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "a rename without AS reads the same",
			SQL:  "SELECT * FROM t1 JOIN t2 b (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c3"},
			},
		},
		{
			Name: "a rename reads a schema-qualified relation",
			SQL:  "SELECT * FROM " + defaultSchema + ".t1 AS a (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
	}
}

// GetCompletionCasesPostgreSQLAndMySQL returns the cases those two accept and
// SQLite does not: a DELETE naming the relations it reads after USING.
func GetCompletionCasesPostgreSQLAndMySQL(defaultSchema, identifierQuote string) []CompletionTestCase {
	return []CompletionTestCase{
		{
			Name: "a DELETE reads the relations its USING names",
			SQL:  "DELETE FROM t1 USING t2 WHERE |",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeTable, Text: "t1"},
				{Type: CandidateTypeTable, Text: "t2"},
				{Type: CandidateTypeColumn, Text: "t1.c1"},
				{Type: CandidateTypeColumn, Text: "t2.c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
				{Type: CandidateTypeColumn, Text: "c3"},
			},
		},
		{
			Name: "a rename of a relation a DELETE uses",
			SQL:  "DELETE FROM t1 USING t2 AS b (|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c3"},
			},
		},
	}
}
