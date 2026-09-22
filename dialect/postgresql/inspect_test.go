package postgresql

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

// TestInspectPostgreSQLSpecific covers syntax that only exists in PostgreSQL.
func TestInspectPostgreSQLSpecific(t *testing.T) {
	dialect := NewDialect()
	meta := core.GetInspectTestMetadata()
	inspector := NewInspector(dialect, meta)
	defaultSchema := meta.DefaultSchema

	cases := []core.InspectTestCase{
		{
			Name: "INSERT with RETURNING",
			SQL:  "INSERT INTO t1 (c1) VALUES (1) RETURNING c2",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpInsert,
					Fields: []core.InspectField{
						{Name: "c1", Table: "t1", Schema: defaultSchema},
					},
					Tables: []core.InspectTable{
						{Name: "t1", Schema: defaultSchema},
					},
					Also: []core.InspectStatement{
						{Operation: core.InspectOpSelect, Tables: []core.InspectTable{{Name: "t1", Schema: defaultSchema}}, Fields: []core.InspectField{{Name: "c2", Table: "t1", Schema: defaultSchema}}},
					},
				},
			},
		},
		{
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
					Also: []core.InspectStatement{
						{Operation: core.InspectOpUpdate, Tables: []core.InspectTable{{Name: "t1", Schema: defaultSchema}}, Fields: []core.InspectField{{Name: "c2", Table: "t1", Schema: defaultSchema}}},
					},
				},
			},
		},
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
		{
			// DISTINCT ON collapses rows on the expressions it names, so those
			// are the tested columns. The projection is returned as it stands.
			Name: "SELECT DISTINCT ON",
			SQL:  "SELECT DISTINCT ON (c1) c1, c3 FROM t2",
			Expected: []core.InspectStatement{
				{
					Operation: core.InspectOpSelect,
					Fields: []core.InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
						{Name: "c3", Table: "t2", Schema: defaultSchema},
					},
					Tables: []core.InspectTable{
						{Name: "t2", Schema: defaultSchema},
					},
					Where: []core.InspectField{
						{Name: "c1", Table: "t2", Schema: defaultSchema},
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
