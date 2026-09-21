package engine

import (
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/selectDb/dialect/core/testutil"
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

// A statement that runs must answer the same on two databases that differ only
// in the hidden column. Where it does not, it has reported that column: not
// necessarily as a value, since a row count, a row order or which rows came
// back says as much, one answer at a time. Masking cannot catch those, which
// is why the check refuses rather than masks.
func TestExecuteLocalAnswersNothingAboutAHiddenColumn(t *testing.T) {
	perms := testutil.GetSeeTestPermissions()
	observe := func(sql, first, second string) testutil.SeeAnswer {
		db, meta, err := testutil.SeeOracleDB(first, second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result := runQuery(ctx, Conn{DB: db, Meta: meta, Perms: perms}, sql)
		return testutil.ReadSeeAnswer(result.RowCount, result.Rows, result.Errors)
	}

	ran := 0
	for _, shape := range oracleShapes {
		answer := observe(shape, "SECRET-alice", "SECRET-bob")
		if answer.Outcome != testutil.SeeRan {
			continue
		}
		ran++
		if answer.Leaked {
			t.Errorf("a value of the hidden column reached a row:\n  %s", shape)
			continue
		}
		if other := observe(shape, "zzz-1", "zzz-2"); other.Rows != answer.Rows {
			t.Errorf("answers for the hidden column:\n  %s\n  one database: %.120s\n  the other:    %.120s",
				shape, answer.Rows, other.Rows)
		}
	}
	if ran == 0 {
		t.Fatal("every shape was refused, so the run proves nothing about the ones that should not be")
	}
	t.Logf("%d of %d shapes ran and answered the same either way", ran, len(oracleShapes))
}
