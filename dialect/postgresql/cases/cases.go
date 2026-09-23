// Package cases holds the case tables only PostgreSQL expresses. They live next to
// the dialect rather than inside its test files so agentprobe -export-cases can
// read them; the dialect's tests run them.
package cases

import (
	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

// InspectCases are the inspect results only PostgreSQL syntax produces.
func InspectCases(defaultSchema string) []core.InspectTestCase {
	return []core.InspectTestCase{
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
}

// SeeCases are the see cases only PostgreSQL expresses.
func SeeCases() []testutil.SeeCase {
	return []testutil.SeeCase{
		{
			Name:    "RETURNING the hidden column alone",
			SQL:     "UPDATE users SET age = age WHERE id = 1 RETURNING email",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "an INSERT returning it",
			SQL:     "INSERT INTO users (id, email) VALUES (9, 'ninth@example.com') RETURNING email",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{Name: "DELETE ... USING filtered on it", SQL: "DELETE FROM contacts USING users WHERE users.email LIKE 'a%'", Refused: true},
		{
			Name:    "a quoted name in another case",
			SQL:     `SELECT "EMAIL" FROM users`,
			Columns: []string{"EMAIL"},
			Masked:  []int{0},
		},
		{Name: "a quoted name in a WHERE", SQL: `SELECT id FROM users WHERE "email" LIKE 'a%'`, Refused: true},
	}
}
