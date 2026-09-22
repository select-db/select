package testutil

import core "github.com/selectDb/dialect/core"

// SeeCase is one statement and what the see rules must do with it. Deciding
// that needs no database, so a case is data.
//
// Cases MUST behave identically in every dialect. SQL only some dialects
// accept (RETURNING, ON CONFLICT, ON DUPLICATE KEY UPDATE) belongs beside the
// inspector that reads it.
//
// Columns are the result columns the statement hands back, in order, and are
// what the driver would report. A case that runs states them, writing an
// empty slice where the statement returns none: leaving them out checks no
// masking at all, which the runner refuses rather than reports green. Refused says the caller gets no rows at all,
// whether the statement was turned away before running or once its result
// columns showed a hidden column with nowhere to mask it. Masked are the
// positions in Columns whose values a role holding select but not see reads
// as core.MaskedValue.
type SeeCase struct {
	Name    string
	SQL     string
	Refused bool
	Columns []string
	Masked  []int
}

// TestDBInstanceID is the database the see cases are checked against.
const TestDBInstanceID = "db1"

// GetSeeTestMetadata describes two tables that share a column name, which is
// what tells a rule hiding one table's column from a rule hiding the other's.
func GetSeeTestMetadata() core.Metadata {
	return core.Metadata{
		DefaultSchema: "main",
		Schemas: []core.Schema{{
			Name: "main",
			Tables: []core.Table{
				{
					Name:       "users",
					PrimaryKey: []string{"id"},
					Columns: []core.Column{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
						{Name: "email", Type: "TEXT"},
						{Name: "age", Type: "INTEGER"},
					},
				},
				{
					Name:       "contacts",
					PrimaryKey: []string{"id"},
					Columns: []core.Column{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
						{Name: "email", Type: "TEXT"},
					},
				},
			},
			// A view is a relation of its own and the rules name its columns,
			// not the ones its body reads. Creating one takes manage, which is
			// what this rests on.
			Views: []core.Table{
				{
					Name: "v_users",
					Columns: []core.Column{
						{Name: "id", Type: "INTEGER"},
						{Name: "email", Type: "TEXT"},
					},
				},
			},
		}},
	}
}

// GetSeeTestPermissions is the policy the cases are read against: every row
// action and see on the schema, with see denied on main.users.email alone.
// main.contacts.email stays visible, so a case that confuses the two shows up.
func GetSeeTestPermissions() core.CompiledPermissions {
	instance := TestDBInstanceID
	schema := "main"
	users := "users"
	email := "email"
	return core.Compile([]core.PermissionEntry{
		{DbInstanceID: &instance, SchemaName: &schema, Action: core.ActionSelect, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: core.ActionInsert, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: core.ActionUpdate, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: core.ActionDelete, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: core.ActionSee, Effect: "allow"},
		{
			DbInstanceID: &instance, SchemaName: &schema, TableName: &users, ColumnName: &email,
			Action: core.ActionSee, Effect: "deny",
		},
	})
}

// GetSeeTestCases returns one case per way SQL has of reading a column.
func GetSeeTestCases() []SeeCase {
	return []SeeCase{
		// --- Returning the column: masked, never refused ---
		{
			Name:    "hidden column beside visible ones",
			SQL:     "SELECT id, email, age FROM users",
			Columns: []string{"id", "email", "age"},
			Masked:  []int{1},
		},
		{
			Name:    "hidden column alone",
			SQL:     "SELECT email FROM users",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "star over the table",
			SQL:     "SELECT * FROM users",
			Columns: []string{"id", "email", "age"},
			Masked:  []int{1},
		},
		{
			Name:    "a column of another table spelled the same is not hidden",
			SQL:     "SELECT email FROM contacts",
			Columns: []string{"email"},
		},
		{
			// Both come out called "email", so which position is which is not
			// in the result columns. Masking both is the fail-closed reading,
			// and the visible one pays for it.
			Name:    "two columns of the same name, one of them hidden",
			SQL:     "SELECT u.email, c.email FROM users u JOIN contacts c ON c.id = u.id",
			Columns: []string{"email", "email"},
			Masked:  []int{0, 1},
		},
		{
			// Named apart, they are told apart.
			Name:    "the same two named apart",
			SQL:     "SELECT u.email AS mine, c.email AS theirs FROM users u JOIN contacts c ON c.id = u.id",
			Columns: []string{"mine", "theirs"},
			Masked:  []int{0},
		},
		{
			Name:    "renamed on the way out",
			SQL:     "SELECT email AS e FROM users",
			Columns: []string{"e"},
			Masked:  []int{0},
		},
		{
			Name:    "handed up by a derived table",
			SQL:     "SELECT s.email FROM (SELECT email FROM users) s",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "renamed by a derived table",
			SQL:     "SELECT s.e FROM (SELECT email AS e FROM users) s",
			Columns: []string{"e"},
			Masked:  []int{0},
		},
		{
			Name:    "handed up by a CTE",
			SQL:     "WITH q AS (SELECT email FROM users) SELECT q.email FROM q",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "a literal beside a hidden column",
			SQL:     "SELECT id, email, 'x' AS tag FROM users",
			Columns: []string{"id", "email", "tag"},
			Masked:  []int{1},
		},

		{
			// The CTE takes the bare name, so the statement reads its columns
			// and not the table's, whose rule does not reach them.
			Name:    "a CTE shadowing the table the rule is on",
			SQL:     "WITH users AS (SELECT email FROM contacts) SELECT * FROM users",
			Columns: []string{"email"},
		},
		{
			// Qualifying the name reaches past the CTE to the table itself.
			Name:    "the same name qualified, which is the table",
			SQL:     "WITH users AS (SELECT email FROM contacts) SELECT * FROM main.users",
			Columns: []string{"id", "email", "age"},
			Masked:  []int{1},
		},
		{
			Name:    "a star over a derived table",
			SQL:     "SELECT * FROM (SELECT email FROM users) q",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "a star over a derived table beside a CTE",
			SQL:     "WITH q AS (SELECT id FROM contacts) SELECT * FROM (SELECT email FROM users) s",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			// The statements behind the names are paired with them by the
			// order the statement declares them, so more than one CTE before a
			// derived table is where a pairing by any other order goes wrong.
			Name:    "a star over a derived table behind two CTEs",
			SQL:     "WITH a AS (SELECT id FROM users), b AS (SELECT id FROM contacts) SELECT * FROM (SELECT email FROM users) q",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "the same with a CTE that reads no table",
			SQL:     "WITH z AS (SELECT 1) SELECT * FROM (SELECT email FROM users) q",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			// The metadata has not caught up with the table, so a column comes
			// back that no field accounts for. The hidden one still has its own
			// position, so the row is masked rather than refused.
			Name:    "a result column the metadata does not know",
			SQL:     "SELECT * FROM users",
			Columns: []string{"id", "email", "age", "added_since"},
			Masked:  []int{1},
		},

		{
			Name:    "a qualified star over a derived table",
			SQL:     "SELECT q.* FROM (SELECT email FROM users) q",
			Columns: []string{"email"},
			Masked:  []int{0},
		},

		{
			Name:    "a CTE that renames it in its body",
			SQL:     "WITH q AS (SELECT email AS e FROM users) SELECT e FROM q",
			Columns: []string{"e"},
			Masked:  []int{0},
		},

		{
			// The subquery chooses rows rather than returning them, so the
			// column it reads is tested, and a visible one of the same name
			// comes back as itself.
			Name:    "a filter subquery beside a column of the same name",
			SQL:     "SELECT c.email FROM contacts c WHERE EXISTS (SELECT u.id FROM users u)",
			Columns: []string{"email"},
		},
		{
			Name:    "a filter subquery in ORDER BY",
			SQL:     "SELECT c.email FROM contacts c ORDER BY (SELECT u.id FROM users u LIMIT 1)",
			Columns: []string{"email"},
		},
		{
			Name:    "a filter subquery in HAVING",
			SQL:     "SELECT c.email FROM contacts c GROUP BY c.email HAVING EXISTS (SELECT u.id FROM users u)",
			Columns: []string{"email"},
		},
		{
			// The derived table calls a hidden column by the name a visible
			// one has elsewhere. What the statement returns is the outer
			// column, and the name it reuses does not hide it.
			Name:    "a name reused in another scope",
			SQL:     "SELECT u.id FROM users u JOIN (SELECT email AS id FROM users) q ON 1=1",
			Columns: []string{"id"},
		},
		{
			Name:    "two derived tables whose aliases do not sort in FROM order",
			SQL:     "SELECT z.email, y.id FROM (SELECT email FROM users) z, (SELECT id FROM contacts) y",
			Columns: []string{"email", "id"},
			Masked:  []int{0},
		},

		// --- Testing the column: refused, since masking cannot reach it ---
		{Name: "WHERE on the hidden column", SQL: "SELECT id FROM users WHERE email LIKE 'a%'", Refused: true},
		{Name: "WHERE through a derived table", SQL: "SELECT s.id FROM (SELECT id, email FROM users) s WHERE s.email LIKE 'a%'", Refused: true},
		{Name: "WHERE through a rename", SQL: "SELECT s.id FROM (SELECT id, email AS e FROM users) s WHERE s.e LIKE 'a%'", Refused: true},
		{Name: "WHERE through a CTE", SQL: "WITH q AS (SELECT id, email FROM users) SELECT q.id FROM q WHERE q.email LIKE 'a%'", Refused: true},
		// Unqualified, only the scope says which relation holds the name, so a
		// resolver that stops at the tables in FROM drops the test silently.
		{Name: "WHERE on it unqualified over a derived table", SQL: "SELECT id FROM (SELECT id, email FROM users) q WHERE email LIKE 'a%'", Refused: true},
		{Name: "WHERE on it unqualified over a star derived table", SQL: "SELECT id FROM (SELECT * FROM users) q WHERE email LIKE 'a%'", Refused: true},
		{Name: "WHERE on it unqualified over a CTE", SQL: "WITH q AS (SELECT id, email FROM users) SELECT id FROM q WHERE email LIKE 'a%'", Refused: true},
		{Name: "GROUP BY it unqualified over a derived table", SQL: "SELECT count(*) FROM (SELECT id, email FROM users) q GROUP BY email", Refused: true},
		{Name: "GROUP BY it unqualified over a CTE", SQL: "WITH q AS (SELECT id, email FROM users) SELECT count(*) FROM q GROUP BY email", Refused: true},
		{Name: "ORDER BY it unqualified over a derived table", SQL: "SELECT id FROM (SELECT id, email FROM users) q ORDER BY email", Refused: true},
		{Name: "ORDER BY it unqualified over a CTE", SQL: "WITH q AS (SELECT id, email FROM users) SELECT id FROM q ORDER BY email", Refused: true},
		{Name: "HAVING on it unqualified over a derived table", SQL: "SELECT id FROM (SELECT id, email FROM users) q GROUP BY id HAVING max(email) > 'm'", Refused: true},
		{Name: "HAVING on it unqualified over a CTE", SQL: "WITH q AS (SELECT id, email FROM users) SELECT id FROM q GROUP BY id HAVING max(email) > 'm'", Refused: true},
		{Name: "a rename tested unqualified over a derived table", SQL: "SELECT id FROM (SELECT id, email AS e FROM users) q WHERE e LIKE 'a%'", Refused: true},
		{Name: "ORDER BY the hidden column", SQL: "SELECT id FROM users ORDER BY email", Refused: true},
		{Name: "GROUP BY the hidden column", SQL: "SELECT count(*) FROM users GROUP BY email", Refused: true},
		{Name: "HAVING on the hidden column", SQL: "SELECT id, count(*) FROM users GROUP BY id HAVING max(email) > 'm'", Refused: true},
		{Name: "DISTINCT over the hidden column", SQL: "SELECT DISTINCT email FROM users", Refused: true},
		{Name: "join condition on the hidden column", SQL: "SELECT u.id FROM users u JOIN contacts c ON c.email = u.email", Refused: true},
		{Name: "join on a USING list naming it", SQL: "SELECT u.id FROM users u JOIN contacts c USING (email)", Refused: true},
		{Name: "natural join pairing on it", SQL: "SELECT u.id FROM users u NATURAL JOIN contacts c", Refused: true},
		{Name: "an inline window ordering by it", SQL: "SELECT id, row_number() OVER (ORDER BY email) AS rn FROM users", Refused: true},
		{Name: "a correlated reference to it", SQL: "SELECT u.id FROM users u WHERE EXISTS (SELECT 1 FROM contacts c WHERE c.email = u.email)", Refused: true},
		{Name: "a subquery selecting it for a comparison", SQL: "SELECT count(*) FROM users WHERE (SELECT email FROM users WHERE id = 1) LIKE 'a%'", Refused: true},
		{
			// The subquery returns it under a name of the outer statement's
			// choosing, which no field accounts for, so there is no position
			// to mask and the statement is refused instead.
			Name: "a subquery returning it under a name of its own",
			SQL:  "SELECT (SELECT email FROM users LIMIT 1) AS x", Refused: true,
			Columns: []string{"x"},
		},
		{
			Name: "the same beside the table's own columns",
			SQL:  "SELECT id, (SELECT email FROM users LIMIT 1) AS x FROM users", Refused: true,
			Columns: []string{"id", "x"},
		},
		{
			// A CTE column list renames it before the outer statement sees it,
			// and the name it gives is not a column of any table.
			Name: "a CTE column list renaming it",
			SQL:  "WITH r(x) AS (SELECT email FROM users) SELECT x FROM r", Refused: true,
			Columns: []string{"x"},
		},
		{Name: "a set operator collapsing duplicates of it", SQL: "SELECT email FROM users UNION SELECT 'a@b.c'", Refused: true},
		{Name: "INTERSECT, which collapses them too", SQL: "SELECT email FROM users INTERSECT SELECT 'a@b.c'", Refused: true},
		{Name: "EXCEPT, which collapses them too", SQL: "SELECT email FROM users EXCEPT SELECT 'a@b.c'", Refused: true},
		{
			// The expression comes out under one name, so there is one place
			// to mask and the value never leaves.
			Name:    "an expression over it, named",
			SQL:     "SELECT upper(email) AS shouted FROM users",
			Columns: []string{"shouted"},
			Masked:  []int{0},
		},
		{
			// Unnamed, the dialects call it whatever they call it, and none of
			// those names is the column's. Nothing accounts for the hidden
			// field, so the statement is refused rather than guessed at.
			Name: "an expression over it, unnamed", SQL: "SELECT upper(email) FROM users",
			Refused: true, Columns: []string{"whatever the driver calls it"},
		},
		{Name: "a write filtered on it", SQL: "UPDATE users SET age = 1 WHERE email LIKE 'a%'", Refused: true},
		{Name: "a delete filtered on it", SQL: "DELETE FROM users WHERE email LIKE 'a%'", Refused: true},
		{Name: "a write reading it into another table", SQL: "INSERT INTO contacts (email) SELECT email FROM users", Refused: true},

		{Name: "a comparison on it", SQL: "SELECT count(*) FROM users WHERE email > ''", Refused: true},
		{Name: "an IN subquery filtered on it", SQL: "SELECT id FROM users WHERE id IN (SELECT id FROM users WHERE email > '')", Refused: true},
		{Name: "an EXISTS subquery selecting it", SQL: "SELECT id FROM users WHERE EXISTS (SELECT email FROM users)", Refused: true},
		{Name: "a HAVING subquery selecting it", SQL: "SELECT id FROM users GROUP BY id HAVING EXISTS (SELECT email FROM users)", Refused: true},
		{Name: "an ORDER BY subquery selecting it", SQL: "SELECT id FROM users ORDER BY (SELECT email FROM users LIMIT 1)", Refused: true},
		{
			Name: "a recursive CTE renaming it in its column list",
			SQL:  "WITH RECURSIVE r(x) AS (SELECT email FROM users) SELECT x FROM r", Refused: true,
			Columns: []string{"x"},
		},
		{Name: "a correlated subquery in ORDER BY", SQL: "SELECT u.id FROM users u ORDER BY (SELECT count(*) FROM contacts c WHERE c.email = u.email)", Refused: true},
		{Name: "a correlated subquery in HAVING", SQL: "SELECT u.id FROM users u GROUP BY u.id HAVING EXISTS (SELECT 1 FROM contacts c WHERE c.email = u.email)", Refused: true},
		{Name: "a join condition naming it beside a visible one", SQL: "SELECT u.id FROM users u LEFT JOIN contacts c ON c.id = u.id AND u.email > 'm'", Refused: true},
		{Name: "an ORDER BY on it with a limit", SQL: "SELECT id FROM users ORDER BY email DESC LIMIT 1", Refused: true},
		{Name: "a window partitioned by it", SQL: "SELECT id, count(*) OVER (PARTITION BY email) AS n FROM users", Refused: true},
		{Name: "a derived table grouping on a rename of it", SQL: "SELECT s.id FROM (SELECT id, email AS e FROM users) s GROUP BY s.e, s.id", Refused: true},
		{Name: "a derived table ordering on a rename of it", SQL: "SELECT s.id FROM (SELECT id, email AS e FROM users) s ORDER BY s.e", Refused: true},
		{Name: "a CTE grouping on it", SQL: "WITH q AS (SELECT id, email FROM users) SELECT q.id FROM q GROUP BY q.email", Refused: true},
		{Name: "a CTE ordering on it", SQL: "WITH q AS (SELECT id, email FROM users) SELECT q.id FROM q ORDER BY q.email", Refused: true},
		{Name: "a write reading it through a rename", SQL: "INSERT INTO contacts (email) SELECT x FROM (SELECT email AS x FROM users) s", Refused: true},
		{Name: "a write reading it through a scalar subquery", SQL: "INSERT INTO contacts (id, email) VALUES (9, (SELECT email FROM users LIMIT 1))", Refused: true},
		{Name: "a write setting a column from it", SQL: "UPDATE contacts SET email = (SELECT email FROM users LIMIT 1)", Refused: true},
		{Name: "a write copying it into another column of its own table", SQL: "UPDATE users SET age = length(email)", Refused: true},
		{Name: "a write filtered on it under an alias", SQL: "UPDATE users AS u SET age = 1 WHERE u.email LIKE 'a%'", Refused: true},
		{Name: "a delete filtered on it under an alias", SQL: "DELETE FROM users AS u WHERE u.email LIKE 'a%'", Refused: true},
		{Name: "a write copying it into another column under an alias", SQL: "UPDATE users AS u SET age = length(u.email)", Refused: true},
		{Name: "a write filtered on it through a subquery", SQL: "UPDATE users SET age = 1 WHERE (SELECT email FROM users WHERE id = 1) LIKE 'a%'", Refused: true},
		{Name: "a delete filtered on it through a subquery", SQL: "DELETE FROM contacts WHERE id IN (SELECT id FROM users WHERE email LIKE 'a%')", Refused: true},
		{Name: "a write correlated on it", SQL: "UPDATE contacts SET id = id WHERE EXISTS (SELECT 1 FROM users u WHERE u.email = contacts.email)", Refused: true},

		// --- Neither: ordinary work that must keep running ---
		{
			Name:    "a set operator that keeps duplicates",
			SQL:     "SELECT email FROM users UNION ALL SELECT 'a@b.c'",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "tests and returns visible columns only",
			SQL:     "SELECT id, age FROM users WHERE age > 20 ORDER BY id",
			Columns: []string{"id", "age"},
		},
		{
			// Aliased because the dialects name an unaliased count differently,
			// and Columns are what the driver reports.
			Name:    "groups and orders on a visible column",
			SQL:     "SELECT age, count(*) AS n FROM users GROUP BY age ORDER BY age",
			Columns: []string{"age", "n"},
		},
		{
			Name:    "joins on a visible column",
			SQL:     "SELECT u.id, u.email FROM users u JOIN contacts c ON c.id = u.id",
			Columns: []string{"id", "email"},
			Masked:  []int{1},
		},
		{
			Name:    "writes to the hidden column without reading it",
			SQL:     "UPDATE users SET email = 'x' WHERE id = 1",
			Columns: []string{},
		},
		{
			Name:    "a write reading only visible columns",
			SQL:     "INSERT INTO contacts (id, email) SELECT id, 'x' FROM users",
			Columns: []string{},
		},
		{Name: "a write filtered on a visible column", SQL: "UPDATE contacts SET id = id + 100", Columns: []string{}},
		{
			// The right is held on the view, so a rule hiding a column of
			// users says nothing about a column of v_users. What the body
			// reads is not in the metadata, so the opacity is a rule about
			// where the right sits rather than something reachable from here.
			Name:    "a rule on a table does not reach the view over it",
			SQL:     "SELECT email FROM v_users",
			Columns: []string{"email"},
		},
		{
			Name:    "a filter subquery over visible columns",
			SQL:     "SELECT id, age FROM users WHERE id IN (SELECT id FROM users WHERE age > 0)",
			Columns: []string{"id", "age"},
		},
		{
			Name:    "a correlated reference to a visible column",
			SQL:     "SELECT u.id FROM users u WHERE EXISTS (SELECT 1 FROM contacts c WHERE c.id = u.id)",
			Columns: []string{"id"},
		},
		{
			Name:    "a derived table over visible columns",
			SQL:     "SELECT s.a FROM (SELECT age AS a FROM users) s WHERE s.a > 0",
			Columns: []string{"a"},
		},
		{
			Name:    "a CTE returning it, ordered on a visible column",
			SQL:     "WITH q AS (SELECT id, email FROM users) SELECT q.id, q.email FROM q ORDER BY q.id",
			Columns: []string{"id", "email"},
			Masked:  []int{1},
		},
	}
}

// GetSeeCasesPostgreSQLAndSQLite are cases in SQL that PostgreSQL and SQLite
// both accept and MySQL does not: RETURNING, ON CONFLICT, UPDATE ... FROM, a
// FILTER on an aggregate, a named window, and a subquery in LIMIT. A dialect
// that reads this SQL runs them beside the shared cases.
func GetSeeCasesPostgreSQLAndSQLite() []SeeCase {
	return []SeeCase{
		{
			Name:    "RETURNING a star over the written table",
			SQL:     "UPDATE users SET age = age WHERE id = 1 RETURNING *",
			Columns: []string{"id", "email", "age"},
			Masked:  []int{1},
		},
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
			Name:    "a DELETE returning it",
			SQL:     "DELETE FROM users WHERE id = 2 RETURNING email",
			Columns: []string{"email"},
			Masked:  []int{0},
		},
		{
			Name:    "an upsert writing to it without reading it",
			SQL:     "INSERT INTO users (id, email) VALUES (1, 'x') ON CONFLICT (id) DO UPDATE SET email = 'y'",
			Columns: []string{},
		},
		{
			Name:    "an upsert that reads nothing hidden",
			SQL:     "INSERT INTO users (id, age) VALUES (1, 2) ON CONFLICT (id) DO UPDATE SET age = 3",
			Columns: []string{},
		},
		{
			Name:    "a filter subquery in LIMIT",
			SQL:     "SELECT c.email FROM contacts c LIMIT (SELECT count(u.id) FROM users u)",
			Columns: []string{"email"},
		},
		{Name: "UPDATE ... FROM filtered on it", SQL: "UPDATE contacts SET id = id FROM users WHERE users.email LIKE 'a%'", Refused: true},
		{Name: "a join condition in an UPDATE ... FROM", SQL: "UPDATE contacts SET id = id FROM users WHERE contacts.email = users.email", Refused: true},
		{Name: "a named window ordering by it", SQL: "SELECT id, row_number() OVER w AS rn FROM users WINDOW w AS (ORDER BY email)", Refused: true},
		{Name: "an upsert predicate reading it", SQL: "INSERT INTO users (id) VALUES (1) ON CONFLICT (id) DO UPDATE SET age = 1 WHERE users.email LIKE 'a%'", Refused: true},
		{Name: "an upsert reading it into another column", SQL: "INSERT INTO users (id) VALUES (1) ON CONFLICT (id) DO UPDATE SET age = length(users.email)", Refused: true},
		// A FILTER reaches the hidden column whether the aggregate it guards
		// is returned or only tested, and each dialect's grammar takes one of
		// the two spellings more readily.
		{Name: "FILTER on a returned aggregate", SQL: "SELECT count(*) FILTER (WHERE email LIKE 'a%') AS n FROM users", Refused: true},
		{Name: "FILTER on an aggregate in HAVING", SQL: "SELECT count(*) AS n FROM users GROUP BY id HAVING count(*) FILTER (WHERE email LIKE 'a%') > 0", Refused: true},
	}
}
