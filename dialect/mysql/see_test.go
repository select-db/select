package mysql

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

// TestSeeMySQLSpecific covers the ways only MySQL has of reading a column: a
// multi-table UPDATE or DELETE, and backtick quoting.
func TestSeeMySQLSpecific(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), []core.SeeCase{
		{Name: "multi-table UPDATE filtered on it", SQL: "UPDATE users JOIN contacts ON contacts.id = users.id SET contacts.id = 1 WHERE users.email LIKE 'a%'", Refused: true},
		{Name: "multi-table UPDATE joined on it", SQL: "UPDATE users JOIN contacts ON contacts.email = users.email SET contacts.id = 1", Refused: true},
		{Name: "DELETE ... USING filtered on it", SQL: "DELETE FROM contacts USING contacts JOIN users ON users.id = contacts.id WHERE users.email LIKE 'a%'", Refused: true},
		{
			Name:    "a backtick quoted name in another case",
			SQL:     "SELECT `EMAIL` FROM users",
			Columns: []string{"EMAIL"},
			Masked:  []int{0},
		},
		{Name: "a backtick quoted name in a WHERE", SQL: "SELECT id FROM users WHERE `email` LIKE 'a%'", Refused: true},
	})
}
