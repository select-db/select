package core

// InspectOperation represents the type of SQL operation
type InspectOperation string

const (
	InspectOpSelect   InspectOperation = "select"
	InspectOpInsert   InspectOperation = "insert"
	InspectOpUpdate   InspectOperation = "update"
	InspectOpDelete   InspectOperation = "delete"
	InspectOpCreate   InspectOperation = "create"
	InspectOpAlter    InspectOperation = "alter"
	InspectOpDrop     InspectOperation = "drop"
	InspectOpTruncate InspectOperation = "truncate"
	InspectOpGrant    InspectOperation = "grant"
	InspectOpRevoke   InspectOperation = "revoke"
	InspectOpUnknown  InspectOperation = "unknown"
)

// InspectField represents a resolved field/column reference in a SQL statement
type InspectField struct {
	Name      string  // Column name
	Alias     *string // Optional alias (e.g., SELECT c1 AS col1)
	Table     string  // Resolved table name
	Schema    string  // Resolved schema name
	StartLine int     // 1-based line of the identifier token (0 = unknown)
	StartCol  int     // 0-based column
	EndCol    int     // 0-based exclusive end column
}

// InspectTable represents a resolved table reference in a SQL statement
type InspectTable struct {
	Name      string  // Table name
	Alias     *string // Optional alias (e.g., FROM t1 AS a)
	Schema    string  // Resolved schema name
	StartLine int     // 1-based line of the identifier token (0 = unknown)
	StartCol  int     // 0-based column
	EndCol    int     // 0-based exclusive end column
}

// InspectStatement represents the structured analysis of a SQL statement
type InspectStatement struct {
	Operation  InspectOperation   // The operation type
	Fields     []InspectField     // Fields/columns involved (for SELECT, UPDATE SET, INSERT columns)
	Tables     []InspectTable     // Tables involved
	Where      []InspectField     // Fields used in WHERE clause (for permission/filtering context)
	Subqueries []InspectStatement // Nested CTEs and subqueries - allows recursive permission checking
}

// InspectTestCase represents a single inspect test case.
// Cases here MUST work identically across every dialect. Dialect-specific
// statements (RETURNING, ON CONFLICT, TRUNCATE, ON DUPLICATE KEY UPDATE…)
// live alongside the inspector that handles them.
type InspectTestCase struct {
	Name     string
	SQL      string
	Expected []InspectStatement // Array since SQL can contain multiple statements
}

// GetInspectTestMetadata returns the standard metadata used for completion tests
func GetInspectTestMetadata() Metadata {
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
			{
				// second schema for cross-schema permission tests
				Name: "other",
				Tables: []Table{
					{
						Name: "t3",
						Columns: []Column{
							{Name: "c1", Type: "INTEGER"},
							{Name: "c4", Type: "TEXT"},
						},
					},
				},
			},
		},
	}
}

func GetInspectTestCases(defaultSchema string) []InspectTestCase {
	return []InspectTestCase{
		// Single column
		{
			Name: "SELECT single column",
			SQL:  "SELECT c1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Multiple columns
		{
			Name: "SELECT multiple columns",
			SQL:  "SELECT c1, c3 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Star expands to all columns
		{
			Name: "SELECT star",
			SQL:  "SELECT * FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// Table-qualified star
		{
			Name: "SELECT table.star",
			SQL:  "SELECT t1.* FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// Column with table prefix
		{
			Name: "SELECT with table prefix",
			SQL:  "SELECT t2.c1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Column alias
		{
			Name: "SELECT with column alias",
			SQL:  "SELECT c1 AS col1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Alias: ptr("col1"), Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Table alias
		{
			Name: "SELECT with table alias",
			SQL:  "SELECT a.c1 FROM t2 AS a",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Alias: ptr("a"), Schema: defaultSchema},
					},
				},
			},
		},
		// JOIN
		{
			Name: "SELECT with JOIN",
			SQL:  "SELECT t1.c1, t2.c3 FROM t1 JOIN t2 ON t1.c1 = t2.c1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// WHERE clause
		{
			Name: "SELECT with WHERE",
			SQL:  "SELECT c1 FROM t2 WHERE c3 = 'foo'",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// CTE - fields resolve to underlying table, Subqueries shows what CTE accesses
		{
			Name: "SELECT with CTE",
			SQL:  "WITH cte AS (SELECT c1, c2 FROM t1) SELECT c1 FROM cte",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t1", Schema: defaultSchema},
								{Name: "c2", Table: "t1", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t1", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
		// Aggregate function COUNT
		{
			Name: "SELECT with COUNT",
			SQL:  "SELECT COUNT(c1) FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Aggregate function with alias
		{
			Name: "SELECT with SUM and alias",
			SQL:  "SELECT SUM(c1) AS total FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Alias: ptr("total"), Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// COUNT star
		{
			Name: "SELECT COUNT star",
			SQL:  "SELECT COUNT(*) FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields:    []InspectField{},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// GROUP BY
		{
			Name: "SELECT with GROUP BY",
			SQL:  "SELECT c1, COUNT(c3) FROM t2 GROUP BY c1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Schema-qualified table
		{
			Name: "SELECT with schema-qualified table",
			SQL:  "SELECT c1 FROM main.t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t2", Schema: "main"},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: "main"},
					},
				},
			},
		},
		// Subquery in FROM - fields resolve to underlying table, Subqueries shows what subquery accesses
		{
			Name: "SELECT from subquery",
			SQL:  "SELECT c1 FROM (SELECT c1, c2 FROM t1) AS sub",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t1", Schema: defaultSchema},
								{Name: "c2", Table: "t1", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t1", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
		// Star with JOIN - expands to all columns from both tables
		{
			Name: "SELECT star with JOIN",
			SQL:  "SELECT * FROM t1 JOIN t2 ON t1.c1 = t2.c1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
						{Name: "c1", Table: "t2", Schema: defaultSchema},
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Multiple statements
		{
			Name: "Multiple SELECT statements",
			SQL:  "SELECT c1 FROM t1; SELECT c3 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		// Expression with column
		{
			Name: "SELECT with expression",
			SQL:  "SELECT c1 + 1 AS calc FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Alias: ptr("calc"), Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},

		// --- INSERT ---

		// Basic insert: Fields = explicitly listed columns
		{
			Name: "INSERT with column list",
			SQL:  "INSERT INTO t1 (c1, c2) VALUES (1, 'foo')",
			Expected: []InspectStatement{
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// Single column insert
		{
			Name: "INSERT single column",
			SQL:  "INSERT INTO t1 (c1) VALUES (1)",
			Expected: []InspectStatement{
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// No column list: expand to all table columns using metadata
		{
			Name: "INSERT without column list expands to all columns",
			SQL:  "INSERT INTO t1 VALUES (1, 'foo')",
			Expected: []InspectStatement{
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// VALUES with subquery expression: t2 must appear in Subqueries for permission check
		{
			Name: "INSERT VALUES with subquery expression",
			SQL:  "INSERT INTO t1 (c1) VALUES ((SELECT c1 FROM t2 WHERE c1 = 1))",
			Expected: []InspectStatement{
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							Where: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
		// INSERT-SELECT: insert target columns in Fields, source SELECT in Subqueries
		{
			Name: "INSERT with SELECT",
			SQL:  "INSERT INTO t1 (c1, c2) SELECT c1, c3 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
								{Name: "c3", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
		// Schema-qualified insert
		{
			Name: "INSERT schema-qualified",
			SQL:  "INSERT INTO main.t1 (c1, c2) VALUES (1, 'foo')",
			Expected: []InspectStatement{
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: "main"},
						{Name: "c2", Table: "t1", Schema: "main"},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: "main"},
					},
				},
			},
		},

		// --- UPDATE ---

		// SET columns go to Fields, WHERE columns go to Where
		{
			Name: "UPDATE single column with WHERE",
			SQL:  "UPDATE t1 SET c2 = 'foo' WHERE c1 = 1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpUpdate,
					Fields: []InspectField{
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "UPDATE multiple columns no WHERE",
			SQL:  "UPDATE t1 SET c1 = 1, c2 = 'foo'",
			Expected: []InspectStatement{
				{
					Operation: InspectOpUpdate,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "UPDATE schema-qualified",
			SQL:  "UPDATE main.t1 SET c2 = 'foo' WHERE c1 = 1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpUpdate,
					Fields: []InspectField{
						{Name: "c2", Table: "t1", Schema: "main"},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: "main"},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: "main"},
					},
				},
			},
		},
		// SET value from subquery: subquery tables must be checked too
		{
			Name: "UPDATE with subquery in SET",
			SQL:  "UPDATE t1 SET c2 = (SELECT c3 FROM t2 WHERE t2.c1 = 1) WHERE c1 = 5",
			Expected: []InspectStatement{
				{
					Operation: InspectOpUpdate,
					Fields: []InspectField{
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c3", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							Where: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// --- DELETE ---

		// Fields is empty for DELETE, no specific columns modified, but WHERE columns tracked
		{
			Name: "DELETE with WHERE",
			SQL:  "DELETE FROM t1 WHERE c1 = 1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpDelete,
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "DELETE all rows",
			SQL:  "DELETE FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpDelete,
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "DELETE schema-qualified",
			SQL:  "DELETE FROM main.t1 WHERE c1 = 1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpDelete,
					Tables: []InspectTable{
						{Name: "t1", Schema: "main"},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: "main"},
					},
				},
			},
		},

		// --- DDL ---

		{
			Name: "DROP TABLE",
			SQL:  "DROP TABLE t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpDrop,
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "DROP TABLE schema-qualified",
			SQL:  "DROP TABLE main.t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpDrop,
					Tables: []InspectTable{
						{Name: "t1", Schema: "main"},
					},
				},
			},
		},
		{
			Name: "ALTER TABLE ADD COLUMN",
			SQL:  "ALTER TABLE t1 ADD COLUMN c4 TEXT",
			Expected: []InspectStatement{
				{
					Operation: InspectOpAlter,
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// CREATE TABLE: target table is not in metadata (new table)
		{
			Name: "CREATE TABLE",
			SQL:  "CREATE TABLE t_new (c1 INTEGER, c2 TEXT)",
			Expected: []InspectStatement{
				{
					Operation: InspectOpCreate,
					Tables: []InspectTable{
						{Name: "t_new", Schema: defaultSchema},
					},
				},
			},
		},

		// --- UNION ---

		// Both sides of UNION must be inspected, both tables need permission
		{
			Name: "UNION both sides inspected",
			SQL:  "SELECT c1 FROM t1 UNION SELECT c1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "UNION ALL both sides inspected",
			SQL:  "SELECT c1 FROM t1 UNION ALL SELECT c1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},

		// --- Multi-column expressions ---

		// All column refs in an expression must be captured, not just the first
		{
			Name: "SELECT binary expression captures both columns",
			SQL:  "SELECT c1 + c2 AS calc FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						// alias belongs to the expression, not to individual source columns
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "SELECT COALESCE captures all argument columns",
			SQL:  "SELECT COALESCE(c1, c2) AS val FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		// CASE WHEN: all column refs across condition and branches must be captured
		{
			Name: "SELECT CASE WHEN captures all referenced columns",
			SQL:  "SELECT CASE WHEN c1 > 0 THEN c2 ELSE c1 END AS result FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						// c1 appears in WHEN and ELSE, deduplicated, first occurrence wins
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},

		// --- Subquery in WHERE ---

		// Subquery tables must appear in Subqueries so permission check is recursive
		{
			Name: "SELECT with IN subquery in WHERE",
			SQL:  "SELECT c1 FROM t1 WHERE c1 IN (SELECT c1 FROM t2)",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
		{
			Name: "SELECT with EXISTS subquery in WHERE",
			SQL:  "SELECT c1 FROM t1 WHERE EXISTS (SELECT 1 FROM t2 WHERE t2.c1 = t1.c1)",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					// Outer WHERE has no direct column refs, only EXISTS(subquery).
					// The subquery itself is tracked in Subqueries for recursive permission checking.
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							// Correlated ref t1.c1 resolves against t2's refs only, not captured here.
							// Permission for t1 is already enforced by the outer query.
							Where: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// --- Unknown tables ---

		// Unknown table: Schema="" since it cannot be resolved from metadata.
		// Permission checker must treat Schema="" as deny, never silently allow.
		// Columns attributed to the unknown table are included for completeness but the
		// table-level deny is the authoritative signal.
		{
			Name: "SELECT from unknown table",
			SQL:  "SELECT c1 FROM unknown_table",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "unknown_table", Schema: ""},
					},
					Tables: []InspectTable{
						{Name: "unknown_table", Schema: ""},
					},
				},
			},
		},
		// Unknown column on known table: field cannot be resolved, must not be silently dropped
		{
			Name: "SELECT unknown column from known table",
			SQL:  "SELECT unknown_col FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "unknown_col", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},

		// --- Cross-schema ---

		{
			Name: "SELECT from other schema",
			SQL:  "SELECT c1 FROM other.t3",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t3", Schema: "other"},
					},
					Tables: []InspectTable{
						{Name: "t3", Schema: "other"},
					},
				},
			},
		},
		{
			Name: "SELECT cross-schema JOIN",
			SQL:  "SELECT t1.c1, t3.c4 FROM main.t1 JOIN other.t3 ON t1.c1 = t3.c1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: "main"},
						{Name: "c4", Table: "t3", Schema: "other"},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: "main"},
						{Name: "t3", Schema: "other"},
					},
				},
			},
		},

		// --- Mixed multi-statement ---

		{
			Name: "SELECT then INSERT multi-statement",
			SQL:  "SELECT c1 FROM t1; INSERT INTO t1 (c1) VALUES (1)",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},

		// Scalar subquery in SELECT list reads a second table invisibly.
		{
			Name: "SELECT scalar subquery in SELECT list",
			SQL:  "SELECT c1, (SELECT c3 FROM t2 WHERE c1 = 1) AS val FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c3", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							Where: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// Correlated scalar subquery in SELECT list.
		{
			Name: "SELECT correlated scalar subquery in SELECT list",
			SQL:  "SELECT c1, (SELECT c3 FROM t2 WHERE t2.c1 = t1.c1) AS val FROM t1",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c3", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							// t2.c1 is inside subquery scope; t1.c1 is a correlated ref
							// from outer scope, not resolved here, already checked by outer query.
							Where: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// INTERSECT, right side is accessed and must be permission-checked.
		{
			Name: "INTERSECT both sides inspected",
			SQL:  "SELECT c1 FROM t1 INTERSECT SELECT c1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},

		// EXCEPT, right side is scanned even though its rows are excluded from output.
		{
			Name: "EXCEPT both sides inspected",
			SQL:  "SELECT c1 FROM t1 EXCEPT SELECT c1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
				},
			},
		},

		// three-way UNION ALL, all three branches must be captured.
		{
			Name: "UNION ALL three branches all inspected",
			SQL:  "SELECT c1 FROM t1 UNION ALL SELECT c1 FROM t2 UNION ALL SELECT c1 FROM other.t3",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c1", Table: "t2", Schema: defaultSchema},
						{Name: "c1", Table: "t3", Schema: "other"},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
						{Name: "t3", Schema: "other"},
					},
				},
			},
		},

		// UNION where the second branch hits an unknown table.
		{
			Name: "UNION second branch unknown table",
			SQL:  "SELECT c1 FROM t1 UNION SELECT c1 FROM unknown_table",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c1", Table: "unknown_table", Schema: ""},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "unknown_table", Schema: ""},
					},
				},
			},
		},

		// CTE named identically to a real table.
		{
			Name: "CTE shadows real table name resolves to underlying table",
			SQL:  "WITH t2 AS (SELECT c1 FROM t1) SELECT c1 FROM t2",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t1", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t1", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// DELETE with a subquery in WHERE reads a second table.
		{
			Name: "DELETE with subquery in WHERE",
			SQL:  "DELETE FROM t1 WHERE c1 IN (SELECT c1 FROM t2)",
			Expected: []InspectStatement{
				{
					Operation: InspectOpDelete,
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// UPDATE with a subquery in WHERE, same vector as DELETE above.
		{
			Name: "UPDATE with subquery in WHERE",
			SQL:  "UPDATE t1 SET c2 = 'x' WHERE c1 IN (SELECT c1 FROM t2)",
			Expected: []InspectStatement{
				{
					Operation: InspectOpUpdate,
					Fields: []InspectField{
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// Deeply nested subquery (3 levels).
		{
			Name: "SELECT deeply nested subquery",
			SQL:  "SELECT c1 FROM t1 WHERE c1 IN (SELECT c1 FROM t2 WHERE c1 IN (SELECT c1 FROM other.t3))",
			Expected: []InspectStatement{
				{
					Operation: InspectOpSelect,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							Where: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Subqueries: []InspectStatement{
								{
									Operation: InspectOpSelect,
									Fields: []InspectField{
										{Name: "c1", Table: "t3", Schema: "other"},
									},
									Tables: []InspectTable{
										{Name: "t3", Schema: "other"},
									},
								},
							},
						},
					},
				},
			},
		},

		// INSERT SELECT from an aliased source, alias must not mask the real table.
		{
			Name: "INSERT SELECT from aliased source",
			SQL:  "INSERT INTO t1 (c1) SELECT a.c1 FROM t2 AS a",
			Expected: []InspectStatement{
				{
					Operation: InspectOpInsert,
					Fields: []InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Subqueries: []InspectStatement{
						{
							Operation: InspectOpSelect,
							Fields: []InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Tables: []InspectTable{
								{Name: "t2", Alias: ptr("a"), Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
	}
}

// ptr is a helper to create string pointers for test cases
func ptr(s string) *string {
	return &s
}
