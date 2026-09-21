package postgresql

import (
	"testing"

	"github.com/selectDb/dialect/core/testutil"
)

// TestSee checks this dialect against the shared see cases. A statement that
// tests a hidden column must be refused, and one that returns it must be
// masked at the right positions.
func TestSee(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), testutil.GetSeeTestCases())
	testutil.RunSeeCases(t, NewDialect(), testutil.GetSeeCasesPostgreSQLAndSQLite())
}

// TestSeePostgreSQLSpecific covers what only PostgreSQL reads: the USING of a
// DELETE, RETURNING on an INSERT, and a name in double quotes.
func TestSeePostgreSQLSpecific(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), []testutil.SeeCase{
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
	})
}
