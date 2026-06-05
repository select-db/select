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

// GetCompletionTestCases returns the standard completion test cases
// defaultSchema is used to replace "main" schema references in SQL and expected outputs
func GetCompletionTestCases(defaultSchema string) []CompletionTestCase {
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
			Name: "simple table columns",
			SQL:  "SELECT t1.| FROM t1",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "simple table columns",
			SQL:  "SELECT t1.|",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "simple table columns",
			SQL:  "SELECT \"t1\".| FROM \"t1\"",
			Expected: []CompletionTestExpectation{
				{Type: CandidateTypeColumn, Text: "c1"},
				{Type: CandidateTypeColumn, Text: "c2"},
			},
		},
		{
			Name: "simple table columns",
			SQL:  "SELECT \"t1\".|",
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
	}
}
