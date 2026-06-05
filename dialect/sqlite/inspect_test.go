package sqlite

import (
	"testing"

	core "github.com/selectDb/dialect/core"
)

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

// TestInspectSQLiteSpecific covers SQLite-only inspect cases.
func TestInspectSQLiteSpecific(t *testing.T) {
	dialect := NewDialect()
	meta := core.GetInspectTestMetadata()
	inspector := NewInspector(dialect, meta)
	defaultSchema := meta.DefaultSchema

	cases := []core.InspectTestCase{
		{
			// SQLite supports ON CONFLICT DO UPDATE SET (UPSERT, since 3.24).
			Name: "INSERT ON CONFLICT DO UPDATE SET",
			SQL:  "INSERT INTO t1 (c1, c2) VALUES (1, 'foo') ON CONFLICT (c1) DO UPDATE SET c2 = EXCLUDED.c2",
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

	// Check operation
	if actual.Operation != expected.Operation {
		t.Errorf("%sexpected operation %q, got %q", prefix, expected.Operation, actual.Operation)
	}

	// Check tables
	if !compareTables(expected.Tables, actual.Tables) {
		t.Errorf("%stables mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Tables, actual.Tables)
	}

	// Check fields
	if !compareFields(expected.Fields, actual.Fields) {
		t.Errorf("%sfields mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Fields, actual.Fields)
	}

	// Check WHERE fields
	if !compareFields(expected.Where, actual.Where) {
		t.Errorf("%sWHERE fields mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Where, actual.Where)
	}

	// Check Subqueries
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
		// Compare alias pointers
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
		// Compare alias values (not pointers)
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
