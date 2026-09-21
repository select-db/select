package engine

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/selectDb/dialect/core"
)

// oracleShapes are the statements that read main.users.email without
// projecting it, one per way SQL has of asking: a correlated reference, a
// rename through a derived table or a CTE, a set operator that collapses
// duplicates, a name written in quotes or another case, a join that names its
// columns without an expression, a window or a FILTER, and DISTINCT. The last
// few are ordinary work, which must keep running.
var oracleShapes = []string{
	`SELECT "EMAIL" FROM users`,
	`SELECT "Email" FROM users`,
	`SELECT "email" FROM users`,
	"SELECT (SELECT count(*) FROM contacts c WHERE c.email = u.email) AS n, u.id FROM users u",
	"SELECT (SELECT count(*) FROM contacts c WHERE c.email = u.id) AS n, u.id FROM users u",
	"SELECT (SELECT count(*) FROM contacts c WHERE c.email = users.email) AS n, users.id FROM users",
	"SELECT (SELECT count(*) FROM contacts c WHERE c.email = users.id) AS n, users.id FROM users",
	"SELECT DISTINCT email FROM users",
	"SELECT DISTINCT email, id FROM users",
	"SELECT DISTINCT id FROM users",
	"SELECT EMAIL FROM users",
	"SELECT Email FROM users",
	"SELECT age, count(*) FROM users GROUP BY age",
	"SELECT count(*) FROM users",
	"SELECT count(DISTINCT email) FROM users",
	"SELECT email FROM users EXCEPT SELECT 'SECRET-alice'",
	"SELECT email FROM users EXCEPT SELECT 'nobody@nowhere'",
	"SELECT email FROM users EXCEPT SELECT 'zzz-1'",
	"SELECT email FROM users EXCEPT SELECT email FROM contacts",
	"SELECT email FROM users INTERSECT SELECT 'SECRET-alice'",
	"SELECT email FROM users INTERSECT SELECT 'nobody@nowhere'",
	"SELECT email FROM users INTERSECT SELECT 'zzz-1'",
	"SELECT email FROM users INTERSECT SELECT email FROM contacts",
	"SELECT email FROM users ORDER BY id",
	"SELECT email FROM users UNION ALL SELECT 'SECRET-alice'",
	"SELECT email FROM users UNION ALL SELECT 'nobody@nowhere'",
	"SELECT email FROM users UNION ALL SELECT 'zzz-1'",
	"SELECT email FROM users UNION ALL SELECT email FROM contacts",
	"SELECT email FROM users UNION SELECT 'SECRET-alice'",
	"SELECT email FROM users UNION SELECT 'nobody@nowhere'",
	"SELECT email FROM users UNION SELECT 'zzz-1'",
	"SELECT email FROM users UNION SELECT email FROM contacts",
	"SELECT id FROM (SELECT id, count(*) FILTER (WHERE email LIKE 'S%') AS n FROM users GROUP BY id) s WHERE s.n > 0",
	"SELECT id FROM (SELECT id, count(*) FILTER (WHERE email LIKE 'z%') AS n FROM users GROUP BY id) s WHERE s.n > 0",
	`SELECT id FROM users GROUP BY "EMAIL", id`,
	`SELECT id FROM users GROUP BY "Email", id`,
	`SELECT id FROM users GROUP BY "email", id`,
	"SELECT id FROM users GROUP BY EMAIL, id",
	"SELECT id FROM users GROUP BY Email, id",
	`SELECT id FROM users ORDER BY "EMAIL"`,
	`SELECT id FROM users ORDER BY "Email"`,
	`SELECT id FROM users ORDER BY "email"`,
	"SELECT id FROM users ORDER BY EMAIL",
	"SELECT id FROM users ORDER BY Email",
	`SELECT id FROM users WHERE "EMAIL" > ''`,
	`SELECT id FROM users WHERE "EMAIL" LIKE 'S%'`,
	`SELECT id FROM users WHERE "EMAIL" LIKE 'z%'`,
	`SELECT id FROM users WHERE "Email" > ''`,
	`SELECT id FROM users WHERE "Email" LIKE 'S%'`,
	`SELECT id FROM users WHERE "Email" LIKE 'z%'`,
	`SELECT id FROM users WHERE "email" > ''`,
	`SELECT id FROM users WHERE "email" LIKE 'S%'`,
	`SELECT id FROM users WHERE "email" LIKE 'z%'`,
	"SELECT id FROM users WHERE EMAIL > ''",
	"SELECT id FROM users WHERE EMAIL LIKE 'S%'",
	"SELECT id FROM users WHERE EMAIL LIKE 'z%'",
	"SELECT id FROM users WHERE Email > ''",
	"SELECT id FROM users WHERE Email LIKE 'S%'",
	"SELECT id FROM users WHERE Email LIKE 'z%'",
	"SELECT id FROM users WHERE age > 20",
	"SELECT id, count(*) OVER (PARTITION BY email) AS n FROM users",
	"SELECT id, email FROM users EXCEPT SELECT id, 'SECRET-alice' FROM contacts",
	"SELECT id, email FROM users EXCEPT SELECT id, 'nobody@nowhere' FROM contacts",
	"SELECT id, email FROM users EXCEPT SELECT id, 'zzz-1' FROM contacts",
	"SELECT id, email FROM users INTERSECT SELECT id, 'SECRET-alice' FROM contacts",
	"SELECT id, email FROM users INTERSECT SELECT id, 'nobody@nowhere' FROM contacts",
	"SELECT id, email FROM users INTERSECT SELECT id, 'zzz-1' FROM contacts",
	"SELECT id, email FROM users JOIN contacts c ON c.id = users.id",
	"SELECT id, email FROM users UNION ALL SELECT id, 'SECRET-alice' FROM contacts",
	"SELECT id, email FROM users UNION ALL SELECT id, 'nobody@nowhere' FROM contacts",
	"SELECT id, email FROM users UNION ALL SELECT id, 'zzz-1' FROM contacts",
	"SELECT id, email FROM users UNION SELECT id, 'SECRET-alice' FROM contacts",
	"SELECT id, email FROM users UNION SELECT id, 'nobody@nowhere' FROM contacts",
	"SELECT id, email FROM users UNION SELECT id, 'zzz-1' FROM contacts",
	"SELECT id, email, age FROM users",
	"SELECT id, row_number() OVER (ORDER BY email) AS rn FROM users",
	"SELECT id, row_number() OVER (ORDER BY id) AS rn FROM users",
	"SELECT id, row_number() OVER w AS rn FROM users WINDOW w AS (ORDER BY email)",
	"SELECT s.e FROM (SELECT email AS e FROM users) s",
	"SELECT s.e, s.id FROM (SELECT id, email AS e FROM users) s ORDER BY s.id",
	"SELECT s.email FROM (SELECT email AS email FROM users) s",
	"SELECT s.email, s.id FROM (SELECT id, email AS email FROM users) s ORDER BY s.id",
	"SELECT s.id FROM (SELECT id, email AS e FROM users) s GROUP BY s.e, s.id",
	"SELECT s.id FROM (SELECT id, email AS e FROM users) s ORDER BY s.e",
	"SELECT s.id FROM (SELECT id, email AS e FROM users) s ORDER BY s.e DESC",
	"SELECT s.id FROM (SELECT id, email AS e FROM users) s WHERE s.e LIKE 'S%'",
	"SELECT s.id FROM (SELECT id, email AS e FROM users) s WHERE s.e LIKE 'z%'",
	"SELECT s.id FROM (SELECT id, email AS email FROM users) s GROUP BY s.email, s.id",
	"SELECT s.id FROM (SELECT id, email AS email FROM users) s ORDER BY s.email",
	"SELECT s.id FROM (SELECT id, email AS email FROM users) s ORDER BY s.email DESC",
	"SELECT s.id FROM (SELECT id, email AS email FROM users) s WHERE s.email LIKE 'S%'",
	"SELECT s.id FROM (SELECT id, email AS email FROM users) s WHERE s.email LIKE 'z%'",
	"SELECT s.id FROM (SELECT id, email AS x FROM users) s GROUP BY s.x, s.id",
	"SELECT s.id FROM (SELECT id, email AS x FROM users) s ORDER BY s.x",
	"SELECT s.id FROM (SELECT id, email AS x FROM users) s ORDER BY s.x DESC",
	"SELECT s.id FROM (SELECT id, email AS x FROM users) s WHERE s.x LIKE 'S%'",
	"SELECT s.id FROM (SELECT id, email AS x FROM users) s WHERE s.x LIKE 'z%'",
	"SELECT s.x FROM (SELECT email AS x FROM users) s",
	"SELECT s.x, s.id FROM (SELECT id, email AS x FROM users) s ORDER BY s.id",
	"SELECT u.id FROM users u GROUP BY u.id HAVING EXISTS (SELECT 1 FROM contacts c WHERE c.email = u.email)",
	"SELECT u.id FROM users u GROUP BY u.id HAVING EXISTS (SELECT 1 FROM contacts c WHERE c.email = u.id)",
	"SELECT u.id FROM users u JOIN contacts c ON c.email = u.email",
	"SELECT u.id FROM users u JOIN contacts c ON c.id = u.id",
	"SELECT u.id FROM users u JOIN contacts c USING (email)",
	"SELECT u.id FROM users u LEFT JOIN contacts c USING (email)",
	"SELECT u.id FROM users u NATURAL JOIN contacts c",
	"SELECT u.id FROM users u ORDER BY (SELECT count(*) FROM contacts c WHERE c.email = u.email)",
	"SELECT u.id FROM users u ORDER BY (SELECT count(*) FROM contacts c WHERE c.email = u.id)",
	"SELECT u.id FROM users u WHERE EXISTS (SELECT 1 FROM contacts c WHERE c.email = u.email)",
	"SELECT u.id FROM users u WHERE EXISTS (SELECT 1 FROM contacts c WHERE c.email = u.id)",
	"SELECT u.id FROM users u WHERE u.id IN (SELECT c.id FROM contacts c WHERE c.email = u.email)",
	"SELECT u.id FROM users u WHERE u.id IN (SELECT c.id FROM contacts c WHERE c.email = u.id)",
	"SELECT users.id FROM users GROUP BY users.id HAVING EXISTS (SELECT 1 FROM contacts c WHERE c.email = users.email)",
	"SELECT users.id FROM users GROUP BY users.id HAVING EXISTS (SELECT 1 FROM contacts c WHERE c.email = users.id)",
	"SELECT users.id FROM users ORDER BY (SELECT count(*) FROM contacts c WHERE c.email = users.email)",
	"SELECT users.id FROM users ORDER BY (SELECT count(*) FROM contacts c WHERE c.email = users.id)",
	"SELECT users.id FROM users WHERE EXISTS (SELECT 1 FROM contacts c WHERE c.email = users.email)",
	"SELECT users.id FROM users WHERE EXISTS (SELECT 1 FROM contacts c WHERE c.email = users.id)",
	"SELECT users.id FROM users WHERE users.id IN (SELECT c.id FROM contacts c WHERE c.email = users.email)",
	"SELECT users.id FROM users WHERE users.id IN (SELECT c.id FROM contacts c WHERE c.email = users.id)",
	"WITH s AS (SELECT id, email AS e FROM users) SELECT s.id FROM s GROUP BY s.e, s.id",
	"WITH s AS (SELECT id, email AS e FROM users) SELECT s.id FROM s ORDER BY s.e",
	"WITH s AS (SELECT id, email AS e FROM users) SELECT s.id FROM s ORDER BY s.e DESC",
	"WITH s AS (SELECT id, email AS e FROM users) SELECT s.id FROM s WHERE s.e LIKE 'S%'",
	"WITH s AS (SELECT id, email AS e FROM users) SELECT s.id FROM s WHERE s.e LIKE 'z%'",
	"WITH s AS (SELECT id, email AS email FROM users) SELECT s.id FROM s GROUP BY s.email, s.id",
	"WITH s AS (SELECT id, email AS email FROM users) SELECT s.id FROM s ORDER BY s.email",
	"WITH s AS (SELECT id, email AS email FROM users) SELECT s.id FROM s ORDER BY s.email DESC",
	"WITH s AS (SELECT id, email AS email FROM users) SELECT s.id FROM s WHERE s.email LIKE 'S%'",
	"WITH s AS (SELECT id, email AS email FROM users) SELECT s.id FROM s WHERE s.email LIKE 'z%'",
	"WITH s AS (SELECT id, email AS x FROM users) SELECT s.id FROM s GROUP BY s.x, s.id",
	"WITH s AS (SELECT id, email AS x FROM users) SELECT s.id FROM s ORDER BY s.x",
	"WITH s AS (SELECT id, email AS x FROM users) SELECT s.id FROM s ORDER BY s.x DESC",
	"WITH s AS (SELECT id, email AS x FROM users) SELECT s.id FROM s WHERE s.x LIKE 'S%'",
	"WITH s AS (SELECT id, email AS x FROM users) SELECT s.id FROM s WHERE s.x LIKE 'z%'",
}

// twoUsersDB returns a database whose only difference from its twin is the
// value of the hidden column.
func twoUsersDB(t *testing.T, first, second string) (*sql.DB, *core.Metadata) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY, email TEXT, age INTEGER);
		CREATE TABLE contacts(id INTEGER PRIMARY KEY, email TEXT);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users VALUES (1,?,30),(2,?,25)`, first, second); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO contacts VALUES (1,'carol@example.com')`); err != nil {
		t.Fatal(err)
	}
	column := func(name, kind string) core.Column { return core.Column{Name: name, Type: kind} }
	meta := &core.Metadata{DefaultSchema: "main", Schemas: []core.Schema{{
		Name: "main",
		Tables: []core.Table{
			{Name: "users", PrimaryKey: []string{"id"}, Columns: []core.Column{
				{Name: "id", Type: "INTEGER", IsPrimaryKey: true}, column("email", "TEXT"), column("age", "INTEGER")}},
			{Name: "contacts", PrimaryKey: []string{"id"}, Columns: []core.Column{
				{Name: "id", Type: "INTEGER", IsPrimaryKey: true}, column("email", "TEXT")}},
		},
	}}}
	return db, meta
}

// A statement that runs must answer the same on two databases that differ only
// in the hidden column. Where it does not, it has reported that column: not
// necessarily as a value, since a row count, a row order or which rows came
// back says as much, one answer at a time. Masking cannot catch those, which
// is why the check refuses rather than masks.
func TestExecuteLocalAnswersNothingAboutAHiddenColumn(t *testing.T) {
	perms := compileFor("db1",
		core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
			ColumnName: sptr("email"), Action: "see", Effect: "deny"},
	)
	observe := func(sql, first, second string) string {
		db, meta := twoUsersDB(t, first, second)
		conn := Conn{DB: db, Meta: meta, Perms: perms}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result := runQuery(ctx, conn, sql)
		if len(result.Errors) > 0 {
			if strings.Contains(strings.Join(result.Errors, " "), "permission denied") {
				return "refused"
			}
			return "not valid SQLite"
		}
		var seen strings.Builder
		fmt.Fprintf(&seen, "rows=%d|", result.RowCount)
		for _, row := range result.Rows {
			for _, value := range row {
				fmt.Fprintf(&seen, "%v,", value)
			}
			seen.WriteString(";")
		}
		return seen.String()
	}

	ran := 0
	for _, shape := range oracleShapes {
		answer := observe(shape, "SECRET-alice", "SECRET-bob")
		if answer == "refused" || answer == "not valid SQLite" {
			continue
		}
		ran++
		if other := observe(shape, "zzz-1", "zzz-2"); other != answer {
			t.Errorf("answers for the hidden column:\n  %s\n  one database: %.120s\n  the other:    %.120s",
				shape, answer, other)
		}
	}
	if ran == 0 {
		t.Fatal("every shape was refused, so the run proves nothing about the ones that should not be")
	}
	t.Logf("%d of %d shapes ran and answered the same either way", ran, len(oracleShapes))
}
