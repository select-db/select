package core

type ExplainTestCase struct {
	Name string
	SQL  string
}

// GetExplainTestCases returns the standard set of explain test cases
// that every SQL dialect should support in its BuildExplainQuery tests.
func GetExplainTestCases() []ExplainTestCase {
	return []ExplainTestCase{
		{
			Name: "simple SELECT without semicolon",
			SQL:  "SELECT 1",
		},
		{
			Name: "simple SELECT with trailing semicolon",
			SQL:  "SELECT 1;",
		},
		{
			Name: "SELECT with whitespace and trailing semicolon",
			SQL:  "\n  SELECT * FROM t1 ;  \n",
		},
		{
			Name: "simple INSERT",
			SQL:  "INSERT INTO t1 (c1, c2) VALUES (1, 'foo');",
		},
		{
			Name: "simple UPDATE",
			SQL:  "UPDATE t1 SET c1 = 2 WHERE c2 = 'bar';",
		},
		{
			Name: "simple DELETE",
			SQL:  "DELETE FROM t1 WHERE c1 = 1;",
		},
		{
			Name: "simple CREATE TABLE",
			SQL:  "CREATE TABLE t3 (id INT PRIMARY KEY);",
		},
		{
			Name: "CTE with trailing semicolon",
			SQL:  "WITH cte AS (SELECT * FROM t1) SELECT * FROM cte;",
		},
		// Edge / failure‑prone shapes
		{
			Name: "EXPLAIN already present (basic)",
			SQL:  "EXPLAIN SELECT * FROM t1;",
		},
		{
			Name: "EXPLAIN already present (with options, lowercase)",
			SQL:  "explain (analyze, buffers) select * from t1;",
		},
		{
			Name: "multiple statements - two SELECTs",
			SQL:  "SELECT * FROM t1; SELECT * FROM t2;",
		},
		{
			Name: "multiple statements - DML then SELECT",
			SQL:  "UPDATE t1 SET c1 = 1 WHERE c2 = 'x'; SELECT * FROM t1;",
		},
		{
			Name: "multiple statements with CTE and semicolons",
			SQL:  "WITH cte AS (SELECT * FROM t1); SELECT * FROM cte;",
		},
		{
			Name: "nested CTEs",
			SQL:  "WITH a AS (SELECT * FROM t1), b AS (SELECT * FROM a) SELECT * FROM b;",
		},
		{
			Name: "transaction block with SELECT",
			SQL:  "BEGIN; SELECT * FROM t1; COMMIT;",
		},
		{
			Name: "empty query string",
			SQL:  "",
		},
		{
			Name: "whitespace‑only query",
			SQL:  "   \n\t  ",
		},
		{
			Name: "semicolon‑only query",
			SQL:  "  ;  ",
		},
	}
}
