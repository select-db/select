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
	})
}

// TestSeeSQLiteSpecific covers the ways only SQLite has of reading a column:
// the FROM of an UPDATE, a FILTER on an aggregate, a named window, RETURNING
// and an upsert predicate.
func TestSeeSQLiteSpecific(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), []core.SeeCase{
		{Name: "UPDATE ... FROM filtered on it", SQL: "UPDATE contacts SET id = id FROM users WHERE users.email LIKE 'a%'", Refused: true},
		{Name: "a join condition in an UPDATE ... FROM", SQL: "UPDATE contacts SET id = id FROM users WHERE contacts.email = users.email", Refused: true},
		{Name: "FILTER on an aggregate", SQL: "SELECT count(*) FILTER (WHERE email LIKE 'a%') AS n FROM users", Refused: true},
		{Name: "a named window ordering by it", SQL: "SELECT id, row_number() OVER w AS rn FROM users WINDOW w AS (ORDER BY email)", Refused: true},
		{Name: "an upsert predicate reading it", SQL: "INSERT INTO users (id) VALUES (1) ON CONFLICT (id) DO UPDATE SET age = 1 WHERE users.email LIKE 'a%'", Refused: true},
		{
			Name:    "RETURNING the hidden column",
			SQL:     "UPDATE users SET age = 1 WHERE id = 1 RETURNING id, email",
			Columns: []string{"id", "email"},
			Masked:  []int{1},
		},
		{
			Name:    "RETURNING visible columns only",
			SQL:     "UPDATE users SET age = 1 WHERE id = 1 RETURNING id, age",
			Columns: []string{"id", "age"},
		},
	})
}
