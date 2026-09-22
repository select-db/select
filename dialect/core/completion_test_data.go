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
			Name:     "a table function's arguments are values, not relations",
			SQL:      "SELECT * FROM generate_series(|",
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

// CompletionKeywordCase states which words a caret can open a statement with.
// The keyword candidates are asserted exactly, so a word a dialect does not
// have is a failure rather than a harmless extra.
type CompletionKeywordCase struct {
	Name     string
	SQL      string
	Expected []string
}

// GetCompletionKeywordCases returns the keyword cases every dialect runs, with
// openers naming the words that dialect declares.
func GetCompletionKeywordCases(openers []string) []CompletionKeywordCase {
	return []CompletionKeywordCase{
		{Name: "an empty buffer opens a statement", SQL: "|", Expected: openers},
		{Name: "whitespace alone opens a statement", SQL: "   |", Expected: openers},
		{Name: "after a semicolon a statement opens again", SQL: "SELECT 1; |", Expected: openers},
		{Name: "after a comment a statement still opens", SQL: "-- a note\n|", Expected: openers},
		{Name: "a select list is not a statement start", SQL: "SELECT |", Expected: nil},
		{Name: "a FROM clause is not a statement start", SQL: "SELECT * FROM |", Expected: nil},
		{Name: "a WHERE clause is not a statement start", SQL: "SELECT * FROM t1 WHERE |", Expected: nil},
		{Name: "a half-written statement is not a statement start", SQL: "SELECT 1; SELECT |", Expected: nil},
		{Name: "the opening word being typed still opens one", SQL: "SEL|", Expected: openers},
		{Name: "one letter still opens one", SQL: "S|", Expected: openers},
		{Name: "a typed opener after a semicolon", SQL: "SELECT 1; SEL|", Expected: openers},
		{Name: "a column being typed is not a statement start", SQL: "SELECT c|", Expected: nil},
	}
}

// KeywordsOfGroup are the words of a dialect's keyword list that the named
// position allows, which is what the keyword cases expect to be offered.
func KeywordsOfGroup(d SQLDialect, group string) []string {
	allowed := keywordGroups[group]
	var words []string
	for _, word := range d.GetDefaultKeywords() {
		if allowed[strings.ToUpper(word)] {
			words = append(words, word)
		}
	}
	return words
}

// CompletionClauseCase names the group of words a caret allows. The words
// themselves are the dialect's, so one case covers three vocabularies.
type CompletionClauseCase struct {
	Name  string
	SQL   string
	Group string
}

// GetCompletionClauseCases names, for each caret, the group of words that
// position allows.
func GetCompletionClauseCases() []CompletionClauseCase {
	return []CompletionClauseCase{
		{"a finished select item takes FROM", "SELECT c1 |", "select_item"},
		{"a call is a finished select item", "SELECT count(c1) |", "select_item"},
		{"an aliased select item takes no second AS", "SELECT c1 AS x |", "aliased_select_item"},
		{"a finished relation takes a clause", "SELECT * FROM t1 |", "relation"},
		{"an aliased relation takes no second AS", "SELECT * FROM t1 AS a |", "aliased_relation"},
		{"a derived table is renamed by the name after it", "SELECT * FROM (SELECT 1) s |", "aliased_relation"},
		{"a relation renamed without AS too", "SELECT * FROM t1 a |", "aliased_relation"},
		{"a select item renamed without AS too", "SELECT c1 x |", "aliased_select_item"},
		{"a join word waits for JOIN", "SELECT * FROM t1 LEFT |", "join_word"},
		{"a bare CROSS too", "SELECT * FROM t1 CROSS |", "join_word"},
		{"IS waits for what it tests", "SELECT * FROM t1 WHERE c1 IS |", "is_test"},
		{"NOT too", "SELECT * FROM t1 WHERE c1 NOT |", "not_test"},
		{"a set operation waits for its query", "SELECT 1 UNION |", "set_operand"},
		{"and ALL does not repeat", "SELECT 1 UNION ALL |", "query_word"},
		{"a joined relation also takes ON", "SELECT * FROM t1 JOIN t2 |", "joined_relation"},
		{"a finished predicate takes AND", "SELECT * FROM t1 WHERE c1 = 1 |", "predicate"},
		{"a join predicate too", "SELECT * FROM t1 JOIN t2 ON t1.c1 = t2.c1 |", "predicate"},
		{"a sort item takes ASC", "SELECT * FROM t1 ORDER BY c1 |", "sort_item"},
		{"a group item takes HAVING", "SELECT * FROM t1 GROUP BY c1 |", "group_item"},
		{"an assignment takes WHERE", "UPDATE t1 SET c1 = 1 |", "assignment"},
		{"a row count takes OFFSET", "SELECT * FROM t1 LIMIT 10 |", "row_count"},
		{"a defined CTE takes its statement", "WITH x AS (SELECT 1) |", "after_cte"},
		{"a half-written predicate takes an operator", "SELECT * FROM t1 WHERE c1 |", ""},
		{"an empty select list takes no keyword", "SELECT |", ""},
		{"an empty FROM takes no keyword", "SELECT * FROM |", ""},
		{"a relation after a comma takes no keyword", "SELECT * FROM t1, |", ""},
		{"a follower being typed is still one", "SELECT * FROM t1 W|", "relation"},
		{"a predicate follower being typed too", "SELECT * FROM t1 WHERE c1 = 1 AN|", "predicate"},
		{"a name being typed is not a follower", "SELECT * FROM t|", ""},
		{"an INSERT target waits for VALUES", "INSERT INTO t1 |", "insert_target"},
		{"an INSERT column list too", "INSERT INTO t1 (c1) |", "insert_target"},
		{"an UPDATE target waits for SET", "UPDATE t1 |", "update_target"},
		{"a DELETE waits for FROM", "DELETE |", "delete_target"},
		{"a DELETE relation reads no join", "DELETE FROM t1 |", "delete_relation"},
		{"nor does a renamed one", "DELETE FROM t1 a |", "delete_aliased_relation"},
		{"a DELETE predicate returns rows", "DELETE FROM t1 WHERE c1 = 1 |", "delete_predicate"},
		{"an UPDATE predicate too", "UPDATE t1 SET c1 = 1 WHERE c1 = 2 |", "update_predicate"},
		{"a subquery in a DELETE is still a query", "DELETE FROM t1 WHERE c1 IN (SELECT c1 FROM t2 WHERE c2 = 1 |", "predicate"},
		{"a sort direction finishes the item", "SELECT * FROM t1 ORDER BY c1 ASC |", "sort_item"},
		{"a CASE test waits for THEN", "SELECT CASE WHEN c1 = 1 |", "case_test"},
		{"a CASE arm waits for WHEN or END", "SELECT CASE WHEN c1 = 1 THEN 2 |", "case_body"},
	}
}

// CompletionFunctionCase states whether a caret takes a call. The words are
// each dialect's hundreds of builtins, so the case asserts that they are
// offered rather than listing them.
type CompletionFunctionCase struct {
	Name    string
	SQL     string
	Offered bool
}

// GetCompletionFunctionCases returns the cases every dialect runs. A call
// stands wherever a value does, and nowhere a column is only being named.
func GetCompletionFunctionCases() []CompletionFunctionCase {
	return []CompletionFunctionCase{
		{"a select list takes a call", "SELECT |", true},
		{"a WHERE takes a call", "SELECT * FROM t1 WHERE |", true},
		{"an ORDER BY takes a call", "SELECT * FROM t1 ORDER BY |", true},
		{"a GROUP BY takes a call", "SELECT * FROM t1 GROUP BY |", true},
		{"an assignment value takes a call", "UPDATE t1 SET c1 = |", true},
		{"a VALUES row takes a call", "INSERT INTO t1 VALUES (|", true},
		{"an argument takes a call", "SELECT count(|) FROM t1", true},
		{"an assignment target does not", "UPDATE t1 SET |", false},
		{"an INSERT column list does not", "INSERT INTO t1 (|", false},
		{"a join's USING list does not", "SELECT * FROM t1 JOIN t2 USING (|", false},
		{"a relation's columns after its dot do not", "SELECT t1.|", false},
		{"a FROM clause does not", "SELECT * FROM |", false},
		{"a rename list does not", "SELECT * FROM t1 AS a (|", false},
	}
}
