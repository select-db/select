package mysql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
)

func ptr(s string) *string { return &s }

func TestInspect(t *testing.T) {
	dialect := NewDialect()
	meta := core.GetInspectTestMetadata()
	inspector := NewInspector(dialect, meta)

	testCases := core.GetInspectTestCases(meta.DefaultSchema)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			results := inspector.Inspect(tc.SQL)

			if len(results) != len(tc.Expected) {
				t.Errorf("Expected %d results, got %d", len(tc.Expected), len(results))
				return
			}

			for i, expected := range tc.Expected {
				compareResult(t, i, expected, results[i])
			}
		})
	}
}

// TestInspectMySQLSpecific covers syntax that doesn't fit the shared test set
// because it's MySQL-only (ON DUPLICATE KEY UPDATE, multi-table DELETE, etc.).
func TestInspectMySQLSpecific(t *testing.T) {
	dialect := NewDialect()
	meta := core.GetInspectTestMetadata()
	inspector := NewInspector(dialect, meta)
	defaultSchema := meta.DefaultSchema

	cases := []core.InspectTestCase{
		{
			Name: "INSERT ON DUPLICATE KEY UPDATE",
			SQL:  "INSERT INTO t1 (c1, c2) VALUES (1, 'foo') ON DUPLICATE KEY UPDATE c2 = VALUES(c2)",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "INSERT SET form",
			SQL:  "INSERT INTO t1 SET c1 = 1, c2 = 'foo'",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "Backtick-quoted identifiers",
			SQL:  "SELECT `c1` FROM `t1`",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpSelect,
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "Database-qualified table",
			SQL:  "SELECT c1 FROM `main`.`t1`",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpSelect,
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: "main"},
					},
					Tables: []core.InspectTable{
						{Name: "t1", Schema: "main"},
					},
				},
			},
		},

		// --- TRUNCATE (MySQL supports it; SQLite does not) ---

		{
			Name: "TRUNCATE",
			SQL:  "TRUNCATE t1",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpTruncate,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "TRUNCATE schema-qualified",
			SQL:  "TRUNCATE main.t1",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpTruncate,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: "main"},
					},
				},
			},
		},

		// --- Writes: every table targeted must be permission-checked ---

		{
			// Multi-table DELETE writes to both t1 and t2.
			Name: "Multi-table DELETE with JOIN",
			SQL:  "DELETE t1, t2 FROM t1 JOIN t2 ON t1.c1 = t2.c1 WHERE t1.c2 = 'x'",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpDelete,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
					Where: []core.InspectField{
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			// Multi-table DELETE with aliases, aliases must resolve back to real tables.
			Name: "Multi-table DELETE with aliases",
			SQL:  "DELETE a, b FROM t1 AS a JOIN t2 AS b ON a.c1 = b.c1",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpDelete,
					Tables: []core.InspectTable{
						{Name: "t1", Alias: ptr("a"), Schema: defaultSchema},
						{Name: "t2", Alias: ptr("b"), Schema: defaultSchema},
					},
				},
			},
		},
		{
			// DELETE … USING multi-table form. Only t1 is the target.
			Name: "DELETE USING form",
			SQL:  "DELETE FROM t1 USING t1 JOIN t2 ON t1.c1 = t2.c1 WHERE t2.c3 = 'x'",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpDelete,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Where: []core.InspectField{
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "Schema-qualified DELETE",
			SQL:  "DELETE FROM main.t1 WHERE c1 = 1",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpDelete,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: "main"},
					},
					Where: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: "main"},
					},
				},
			},
		},
		{
			// Multi-table UPDATE writes to t1 via the JOIN.
			Name: "Multi-table UPDATE with JOIN",
			SQL:  "UPDATE t1 JOIN t2 ON t1.c1 = t2.c1 SET t1.c2 = t2.c3 WHERE t2.c1 = 5",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpUpdate,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Where: []core.InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "Multi-table UPDATE writes to both tables",
			SQL:  "UPDATE t1 JOIN t2 ON t1.c1 = t2.c1 SET t1.c2 = 'x', t2.c3 = 'y'",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpUpdate,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c2", Table: "t1", Schema: defaultSchema},
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "Schema-qualified UPDATE",
			SQL:  "UPDATE main.t1 SET c2 = 'foo' WHERE c1 = 1",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpUpdate,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: "main"},
					},
					Fields: []core.InspectField{
						{Name: "c2", Table: "t1", Schema: "main"},
					},
					Where: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: "main"},
					},
				},
			},
		},
		{
			// REPLACE behaves like INSERT for permission purposes.
			Name: "REPLACE INTO with column list",
			SQL:  "REPLACE INTO t1 (c1, c2) VALUES (1, 'foo')",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			Name: "REPLACE INTO SELECT",
			SQL:  "REPLACE INTO t1 (c1, c2) SELECT c1, c3 FROM t2",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []core.InspectStatement{
						{
							Operation: core.InspectOpSelect,
							Tables: []core.InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							Fields: []core.InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
								{Name: "c3", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
		{
			// INSERT IGNORE, same permission shape as INSERT.
			Name: "INSERT IGNORE",
			SQL:  "INSERT IGNORE INTO t1 (c1) VALUES (1)",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
				},
			},
		},
		{
			// ON DUPLICATE KEY UPDATE with a subquery in the SET clause, the
			// subquery's source table must still be permission-checked.
			Name: "INSERT ON DUPLICATE KEY UPDATE with subquery",
			SQL:  "INSERT INTO t1 (c1, c2) VALUES (1, 'foo') ON DUPLICATE KEY UPDATE c2 = (SELECT c3 FROM t2 WHERE c1 = 1)",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []core.InspectStatement{
						{
							Operation: core.InspectOpSelect,
							Tables: []core.InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							Fields: []core.InspectField{
								{Name: "c3", Table: "t2", Schema: defaultSchema},
							},
							Where: []core.InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
		{
			// INSERT ... VALUES with multiple rows that reference subqueries.
			Name: "INSERT VALUES with subquery in MySQL",
			SQL:  "INSERT INTO t1 (c1) VALUES ((SELECT c1 FROM t2 WHERE c3 = 'x'))",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []core.InspectStatement{
						{
							Operation: core.InspectOpSelect,
							Tables: []core.InspectTable{
								{Name: "t2", Schema: defaultSchema},
							},
							Fields: []core.InspectField{
								{Name: "c1", Table: "t2", Schema: defaultSchema},
							},
							Where: []core.InspectField{
								{Name: "c3", Table: "t2", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},

		// --- Identifier handling: must normalize so permission lookups match ---

		{
			// MySQL identifiers normalize to lower case for our purposes;
			// uppercase SQL must still resolve to the lowercase metadata.
			Name: "Uppercase identifiers",
			SQL:  "SELECT C1 FROM T1 WHERE C2 = 'x'",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpSelect,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Where: []core.InspectField{
						{Name: "c2", Table: "t1", Schema: defaultSchema},
					},
				},
			},
		},

		// --- SELECT-side MySQL quirks ---

		{
			Name: "SELECT FROM DUAL",
			SQL:  "SELECT 1 FROM DUAL",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpSelect,
					Fields:    nil,
				},
			},
		},
		{
			Name: "SELECT with CROSS JOIN",
			SQL:  "SELECT t1.c1, t2.c3 FROM t1 CROSS JOIN t2",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpSelect,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
						{Name: "t2", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
				},
			},
		},
		{
			// MySQL 8 CTE on top of a SELECT, same shape as the shared test
			// case but exercises the MySQL-side WithClause walking.
			Name: "WITH cte then SELECT",
			SQL:  "WITH cte AS (SELECT c1 FROM t1) SELECT c1 FROM cte",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpSelect,
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Subqueries: []core.InspectStatement{
						{
							Operation: core.InspectOpSelect,
							Tables: []core.InspectTable{
								{Name: "t1", Schema: defaultSchema},
							},
							Fields: []core.InspectField{
								{Name: "c1", Table: "t1", Schema: defaultSchema},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			results := inspector.Inspect(tc.SQL)
			if len(results) != len(tc.Expected) {
				t.Errorf("Expected %d results, got %d", len(tc.Expected), len(results))
				return
			}
			for i, expected := range tc.Expected {
				compareResult(t, i, expected, results[i])
			}
		})
	}
}

func compareResult(t *testing.T, idx int, expected, actual core.InspectStatement) {
	prefix := ""
	if idx >= 0 {
		prefix = "Result " + string(rune('0'+idx)) + ": "
	}

	if actual.Operation != expected.Operation {
		t.Errorf("%sexpected operation %q, got %q", prefix, expected.Operation, actual.Operation)
	}

	if !compareTables(expected.Tables, actual.Tables) {
		t.Errorf("%stables mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Tables, actual.Tables)
	}

	if !compareFields(expected.Fields, actual.Fields) {
		t.Errorf("%sfields mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Fields, actual.Fields)
	}

	if !compareFields(expected.Where, actual.Where) {
		t.Errorf("%sWHERE fields mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Where, actual.Where)
	}

	if len(expected.Subqueries) != len(actual.Subqueries) {
		t.Errorf("%ssubqueries count mismatch: expected %d, got %d", prefix, len(expected.Subqueries), len(actual.Subqueries))
	} else {
		for j, expSub := range expected.Subqueries {
			compareResult(t, -1, expSub, actual.Subqueries[j])
		}
	}
}

func compareTables(expected, actual []core.InspectTable) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].Name != actual[i].Name {
			return false
		}
		if expected[i].Schema != actual[i].Schema {
			return false
		}
		if (expected[i].Alias == nil) != (actual[i].Alias == nil) {
			return false
		}
		if expected[i].Alias != nil && actual[i].Alias != nil && *expected[i].Alias != *actual[i].Alias {
			return false
		}
	}
	return true
}

func compareFields(expected, actual []core.InspectField) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].Name != actual[i].Name {
			return false
		}
		if expected[i].Table != actual[i].Table {
			return false
		}
		if expected[i].Schema != actual[i].Schema {
			return false
		}
		expectedAlias := ""
		actualAlias := ""
		if expected[i].Alias != nil {
			expectedAlias = *expected[i].Alias
		}
		if actual[i].Alias != nil {
			actualAlias = *actual[i].Alias
		}
		if expectedAlias != actualAlias {
			return false
		}
	}
	return true
}
