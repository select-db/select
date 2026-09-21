package core

// SeeCase is one statement and what the see rules must do with it. Deciding
// that needs no database, so a case is data.
//
// Cases MUST behave identically in every dialect. SQL only some dialects
// accept (RETURNING, ON CONFLICT, ON DUPLICATE KEY UPDATE) belongs beside the
// inspector that reads it.
//
// Columns are the result columns the statement hands back, in order, and are
// what the driver would report. Refused says the caller gets no rows at all,
// whether the statement was turned away before running or once its result
// columns showed a hidden column with nowhere to mask it. Masked are the
// positions in Columns whose values a role holding select but not see reads
// as MaskedValue.
type SeeCase struct {
	Name    string
	SQL     string
	Refused bool
	Columns []string
	Masked  []int
}

// SeeTestDBInstanceID is the database the see cases are checked against.
const SeeTestDBInstanceID = "db1"

// GetSeeTestMetadata describes two tables that share a column name, which is
// what tells a rule hiding one table's column from a rule hiding the other's.
func GetSeeTestMetadata() Metadata {
	return Metadata{
		DefaultSchema: "main",
		Schemas: []Schema{{
			Name: "main",
			Tables: []Table{
				{
					Name:       "users",
					PrimaryKey: []string{"id"},
					Columns: []Column{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
						{Name: "email", Type: "TEXT"},
						{Name: "age", Type: "INTEGER"},
					},
				},
				{
					Name:       "contacts",
					PrimaryKey: []string{"id"},
					Columns: []Column{
						{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
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
func GetSeeTestPermissions() CompiledPermissions {
	instance := SeeTestDBInstanceID
	schema := "main"
	users := "users"
	email := "email"
	return Compile([]PermissionEntry{
		{DbInstanceID: &instance, SchemaName: &schema, Action: ActionSelect, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: ActionInsert, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: ActionUpdate, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: ActionDelete, Effect: "allow"},
		{DbInstanceID: &instance, SchemaName: &schema, Action: ActionSee, Effect: "allow"},
		{
			DbInstanceID: &instance, SchemaName: &schema, TableName: &users, ColumnName: &email,
			Action: ActionSee, Effect: "deny",
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

		// --- Testing the column: refused, since masking cannot reach it ---
		{Name: "WHERE on the hidden column", SQL: "SELECT id FROM users WHERE email LIKE 'a%'", Refused: true},
		{Name: "WHERE through a derived table", SQL: "SELECT s.id FROM (SELECT id, email FROM users) s WHERE s.email LIKE 'a%'", Refused: true},
		{Name: "WHERE through a rename", SQL: "SELECT s.id FROM (SELECT id, email AS e FROM users) s WHERE s.e LIKE 'a%'", Refused: true},
		{Name: "WHERE through a CTE", SQL: "WITH q AS (SELECT id, email FROM users) SELECT q.id FROM q WHERE q.email LIKE 'a%'", Refused: true},
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
			Columns: nil,
		},
		{
			Name:    "a write reading only visible columns",
			SQL:     "INSERT INTO contacts (id, email) SELECT id, 'x' FROM users",
			Columns: nil,
		},
	}
}
