package postgresql

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

// TestSeePostgreSQLSpecific covers the ways only PostgreSQL has of reading a
// column: the FROM of an UPDATE, the USING of a DELETE, a FILTER on an
// aggregate, a named window, RETURNING and an upsert predicate.
func TestSeePostgreSQLSpecific(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), []core.SeeCase{
		{
			Name:    "RETURNING a star over the written table",
			SQL:     "UPDATE users SET age = age WHERE id = 1 RETURNING *",
			Columns: []string{"id", "email", "age"},
			Masked:  []int{1},
		},
		{
			Name:    "RETURNING the hidden column alone",
			SQL:     "UPDATE users SET age = age WHERE id = 1 RETURNING email",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "a DELETE returning it",
			SQL:     "DELETE FROM users WHERE id = 2 RETURNING email",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "an INSERT returning it",
			SQL:     "INSERT INTO users (id, email) VALUES (9, 'ninth@example.com') RETURNING email",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "an upsert writing to it without reading it",
			SQL:     "INSERT INTO users (id, email) VALUES (1, 'x') ON CONFLICT (id) DO UPDATE SET email = 'y'",
			Columns: nil,
		},
		{
			// MySQL takes no subquery in LIMIT, so this is not a shared case.
			Name:    "a filter subquery in LIMIT",
			SQL:     "SELECT c.email FROM contacts c LIMIT (SELECT count(u.id) FROM users u)",
			Columns: []string{"email"},
		},
		{Name: "UPDATE ... FROM filtered on it", SQL: "UPDATE contacts SET id = id FROM users WHERE users.email LIKE 'a%'", Refused: true},
		{Name: "DELETE ... USING filtered on it", SQL: "DELETE FROM contacts USING users WHERE users.email LIKE 'a%'", Refused: true},
		{Name: "a join condition in an UPDATE ... FROM", SQL: "UPDATE contacts SET id = id FROM users WHERE contacts.email = users.email", Refused: true},
		{Name: "FILTER on an aggregate", SQL: "SELECT count(*) AS n FROM users GROUP BY id HAVING count(*) FILTER (WHERE email LIKE 'a%') > 0", Refused: true},
		{Name: "a named window ordering by it", SQL: "SELECT id, row_number() OVER w AS rn FROM users WINDOW w AS (ORDER BY email)", Refused: true},
		{Name: "an upsert predicate reading it", SQL: "INSERT INTO users (id) VALUES (1) ON CONFLICT (id) DO UPDATE SET age = 1 WHERE users.email LIKE 'a%'", Refused: true},
		{Name: "an upsert reading it into another column", SQL: "INSERT INTO users (id) VALUES (1) ON CONFLICT (id) DO UPDATE SET age = length(users.email)", Refused: true},
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
		{
			Name:    "a quoted name in another case",
			SQL:     `SELECT "EMAIL" FROM users`,
			Columns: []string{"EMAIL"},
			Masked:  []int{0},
		},
		{Name: "a quoted name in a WHERE", SQL: `SELECT id FROM users WHERE "email" LIKE 'a%'`, Refused: true},
		{
			Name:    "an upsert that reads nothing hidden",
			SQL:     "INSERT INTO users (id, age) VALUES (1, 2) ON CONFLICT (id) DO UPDATE SET age = 3",
			Columns: nil,
		},
	})
}
