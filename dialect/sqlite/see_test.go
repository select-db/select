package sqlite

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

// TestSee checks this dialect against the shared see cases. A statement that
// tests a hidden column must be refused, and one that returns it must be
// masked at the right positions.
func TestSee(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), core.GetSeeTestCases())
}

// TestSeeQuoting covers the three ways SQLite lets a statement quote a name.
// A column wearing quotes the dialect does not strip matches no rule, so it
// comes back in the clear.
func TestSeeQuoting(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), []core.SeeCase{
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
		{Name: "backticks in a WHERE", SQL: "SELECT id FROM users WHERE `email` LIKE 'a%'", Refused: true},
		{Name: "square brackets in a WHERE", SQL: "SELECT id FROM users WHERE [email] LIKE 'a%'", Refused: true},
		{Name: "backticks in an ORDER BY", SQL: "SELECT id FROM users ORDER BY `email`", Refused: true},
		{Name: "backticks in a GROUP BY", SQL: "SELECT count(*) FROM users GROUP BY `email`", Refused: true},
		{
			Name:    "quoting visible columns changes nothing",
			SQL:     "SELECT `id`, `age` FROM users WHERE `age` > 20",
			Columns: []string{"id", "age"},
		},
	})
}
