// Package cases holds the case tables only SQLite expresses. They live next to
// the dialect rather than inside its test files so agentprobe -export-cases can
// read them; the dialect's tests run them.
package cases

import (
	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

// InspectCases are the inspect results only SQLite syntax produces.
func InspectCases(defaultSchema string) []core.InspectTestCase {
	return []core.InspectTestCase{
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
					Also: []core.InspectStatement{
						{Operation: core.InspectOpUpdate, Tables: []core.InspectTable{{Name: "t1", Schema: defaultSchema}}, Fields: []core.InspectField{{Name: "c2", Table: "t1", Schema: defaultSchema}}},
					},
				},
			},
		},
	}
}

// SeeCases are the see cases only SQLite expresses.
func SeeCases() []testutil.SeeCase {
	return []testutil.SeeCase{
		{
			Name:    "double quotes around the hidden column",
			SQL:     `SELECT "email" FROM users`,
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "backticks around the hidden column",
			SQL:     "SELECT `email` FROM users",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "square brackets around the hidden column",
			SQL:     "SELECT [email] FROM users",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "a quoted rename of it",
			SQL:     "SELECT `email` AS e FROM users",
			Columns: []string{"e"},
			Masked:  []int{0},
		},
		{
			Name:    "a quoting the metadata does not spell, since SQLite ignores case",
			SQL:     "SELECT `EMAIL` FROM users",
			Columns: []string{"EMAIL"},
			Masked:  []int{0},
		},
		{
			Name:    "brackets around it in another case",
			SQL:     "SELECT [Email] FROM users",
			Columns: []string{"Email"},
			Masked:  []int{0},
		},
		{Name: "backticks in a WHERE", SQL: "SELECT id FROM users WHERE `email` LIKE 'a%'", Refused: true},
		{Name: "backticks and another case in a WHERE", SQL: "SELECT id FROM users WHERE `EMAIL` LIKE 'a%'", Refused: true},
		{Name: "brackets and another case in an ORDER BY", SQL: "SELECT id FROM users ORDER BY [EMAIL]", Refused: true},
		{Name: "square brackets in a WHERE", SQL: "SELECT id FROM users WHERE [email] LIKE 'a%'", Refused: true},
		{Name: "backticks in an ORDER BY", SQL: "SELECT id FROM users ORDER BY `email`", Refused: true},
		{Name: "backticks in a GROUP BY", SQL: "SELECT count(*) FROM users GROUP BY `email`", Refused: true},
		{
			Name:    "quoting visible columns changes nothing",
			SQL:     "SELECT `id`, `age` FROM users WHERE `age` > 20",
			Columns: []string{"id", "age"},
		},
	}
}
