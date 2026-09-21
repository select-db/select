package sqlite

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
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
				testutil.CompareResult(t, i, expected, results[i])
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
				testutil.CompareResult(t, i, expected, results[i])
			}
		})
	}
}
