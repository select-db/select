package engine

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/selectDb/dialect/core"
)

// setupUsersDB returns a sqlite in-memory DB with a small users table populated
// with two rows. The metadata reflects the schema so the inspector can resolve
// columns.
func setupUsersDB(t *testing.T) (*sql.DB, *core.Metadata) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY, email TEXT NOT NULL, age INTEGER)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users VALUES (1,'alice@example.com',30),(2,'bob@example.com',25)`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	meta := &core.Metadata{
		DefaultSchema: "main",
		Schemas: []core.Schema{{
			Name: "main",
			Tables: []core.Table{{
				Name:       "users",
				PrimaryKey: []string{"id"},
				Columns: []core.Column{
					{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
					{Name: "email", Type: "TEXT"},
					{Name: "age", Type: "INTEGER"},
				},
			}},
		}},
	}
	return db, meta
}

func sptr(s string) *string { return &s }

func compileFor(dbID string, entries ...core.PermissionEntry) core.CompiledPermissions {
	for i := range entries {
		entries[i].DbInstanceID = sptr(dbID)
	}
	return core.Compile(entries)
}

func runQuery(ctx context.Context, conn Conn, sql string) *Result {
	return ExecuteLocal(ctx, conn, DBInstance{ID: "db1", DBType: "sqlite"}, sql, Options{})
}

func TestExecuteLocalMasksSeeDeniedColumn(t *testing.T) {
	db, meta := setupUsersDB(t)
	conn := Conn{
		DB:   db,
		Meta: meta,
		Perms: compileFor("db1",
			// select on every column (table wildcard), no see on email
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("id"), Action: "see", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("age"), Action: "see", Effect: "allow"},
		),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "SELECT id, email, age FROM users ORDER BY id")
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
	for i, row := range result.Rows {
		if row[1] != core.MaskedValue {
			t.Errorf("row %d: email = %v, want %q", i, row[1], core.MaskedValue)
		}
		if row[0] == core.MaskedValue {
			t.Errorf("row %d: id wrongly masked: %v", i, row[0])
		}
		if row[2] == core.MaskedValue {
			t.Errorf("row %d: age wrongly masked: %v", i, row[2])
		}
	}
}

func TestExecuteLocalMasksNullInSeeDeniedColumn(t *testing.T) {
	db, meta := setupUsersDB(t)
	// a see-denied cell that is NULL must still mask, or its emptiness leaks
	if _, err := db.Exec(`INSERT INTO users VALUES (3,'carol@example.com',NULL)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	conn := Conn{
		DB:   db,
		Meta: meta,
		Perms: compileFor("db1",
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("id"), Action: "see", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("email"), Action: "see", Effect: "allow"},
		),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "SELECT id, email, age FROM users ORDER BY id")
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	for i, row := range result.Rows {
		if row[2] != core.MaskedValue {
			t.Errorf("row %d: age = %v, want %q (NULL must be masked)", i, row[2], core.MaskedValue)
		}
	}
}

// A predicate on a hidden column answers questions about its values one at a
// time, and enough answers are the value. Masking hides a column from the eye;
// refusing here is what hides it from the query.
func TestExecuteLocalPredicateOnHiddenColumnIsRefused(t *testing.T) {
	for _, sql := range []string{
		"SELECT id, age FROM users WHERE email = 'alice@example.com'",
		"SELECT id FROM users WHERE email LIKE 'a%'",
		"SELECT count(*) FROM users WHERE email > ''",
		"SELECT id FROM users WHERE id IN (SELECT id FROM users WHERE email > '')",
		"UPDATE users SET age = 1 WHERE email = 'alice@example.com'",
		"DELETE FROM users WHERE email = 'alice@example.com'",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "delete", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatalf("ran, returning %v", result.Rows)
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
		})
	}
}

func TestExecuteLocalRejectsFunctionOverSeeDenied(t *testing.T) {
	db, meta := setupUsersDB(t)
	conn := Conn{
		DB:   db,
		Meta: meta,
		Perms: compileFor("db1",
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("id"), Action: "see", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("age"), Action: "see", Effect: "allow"},
		),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "SELECT LENGTH(email) FROM users")
	if len(result.Errors) == 0 {
		t.Fatalf("expected permission error, got rows=%d", result.RowCount)
	}
}

func TestExecuteLocalRejectsAggregateOnSeeDenied(t *testing.T) {
	// COUNT(email) is the aggregation-bypass vector the see permission is
	// meant to block. The column appears inside a function call so it's
	// classified as a derived projection: any source field that is not the
	// bare Source of an Output entry must be see-allowed.
	db, meta := setupUsersDB(t)
	conn := Conn{
		DB:   db,
		Meta: meta,
		Perms: compileFor("db1",
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("id"), Action: "see", Effect: "allow"},
		),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "SELECT COUNT(email) FROM users")
	if len(result.Errors) == 0 {
		t.Fatalf("expected permission error, got rows=%d", result.RowCount)
	}
}

// NOTE: GROUP BY / ORDER BY / HAVING / DISTINCT references are not currently
// tracked by any dialect inspector. They don't end up in stmt.Fields. As a
// result, queries like `SELECT COUNT(*) FROM users GROUP BY email` slip through
// every permission check today (including the existing select permission, not
// just see). Fixing this requires per-dialect inspector changes to walk these
// clauses and surface their column refs. Tracked as follow-up work.

func TestExecuteLocalSelectStarMasksOnlyDenied(t *testing.T) {
	db, meta := setupUsersDB(t)
	conn := Conn{
		DB:   db,
		Meta: meta,
		Perms: compileFor("db1",
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("id"), Action: "see", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("age"), Action: "see", Effect: "allow"},
		),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "SELECT * FROM users ORDER BY id")
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(result.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %v", result.Columns)
	}
	for i, row := range result.Rows {
		// driver order is id, email, age
		if row[1] != core.MaskedValue {
			t.Errorf("row %d: email = %v, want masked", i, row[1])
		}
		if row[0] == core.MaskedValue || row[2] == core.MaskedValue {
			t.Errorf("row %d: non-email masked: %v", i, row)
		}
	}
}

func TestExecuteLocalSchemaDriftFailsClosed(t *testing.T) {
	db, meta := setupUsersDB(t)
	// Lie about the schema: rename email to "secret" in metadata. The
	// inspector will produce Output[1].Column = "secret", but the driver
	// returns "email" for that position. The assertion must fail.
	meta.Schemas[0].Tables[0].Columns[1].Name = "secret"

	conn := Conn{
		DB:   db,
		Meta: meta,
		Perms: compileFor("db1",
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("id"), Action: "see", Effect: "allow"},
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("age"), Action: "see", Effect: "allow"},
		),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "SELECT * FROM users")
	if len(result.Errors) == 0 {
		t.Fatalf("expected schema drift error, got %d rows", result.RowCount)
	}
}

func TestExecuteLocalNoSeeRulesNoMasking(t *testing.T) {
	db, meta := setupUsersDB(t)
	conn := Conn{
		DB:   db,
		Meta: meta,
		Perms: compileFor("db1",
			core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
		),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No see rules at all: every Field is see-denied, every bare projection
	// gets masked. This verifies the default-deny behavior. Nothing is ordered
	// or grouped on: with no see rules an ORDER BY column is a refusal, which
	// TestExecuteLocalOrderByHiddenColumnRefused covers.
	result := runQuery(ctx, conn, "SELECT email FROM users")
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	for i, row := range result.Rows {
		if row[0] != core.MaskedValue {
			t.Errorf("row %d: email = %v, want masked", i, row[0])
		}
	}
}

// seeEmailDenied is the policy the tests below share: select on the table, see
// on every column but email.
func seeEmailDenied() core.CompiledPermissions {
	return compileFor("db1",
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), Action: "select", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("id"), Action: "see", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"), ColumnName: sptr("age"), Action: "see", Effect: "allow"},
	)
}

// A derived table returns the column to the outer select, under its own name or
// a new one. The see check reads the whole statement, so it finds the column
// where it was read rather than where it comes out.
func TestExecuteLocalMasksThroughDerivedTable(t *testing.T) {
	for _, sql := range []string{
		"SELECT e FROM (SELECT email AS e FROM users) q",
		"SELECT q.email FROM (SELECT email FROM users) q",
		"SELECT q.* FROM (SELECT email FROM users) q",
		"SELECT * FROM (SELECT email FROM users) q",
		"WITH q AS (SELECT email AS e FROM users) SELECT e FROM q",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("unexpected errors: %v", result.Errors)
			}
			if result.RowCount == 0 {
				t.Fatal("no rows, so nothing here was checked")
			}
			for i, row := range result.Rows {
				if row[0] != core.MaskedValue {
					t.Errorf("row %d: email = %v, want %q", i, row[0], core.MaskedValue)
				}
			}
		})
	}
}

// A subquery in the select list returns the column under a name of the outer
// statement's choosing, which no field accounts for. There is no position to
// mask, so the statement is refused rather than run.
func TestExecuteLocalRejectsSubqueryOverSeeDenied(t *testing.T) {
	for _, sql := range []string{
		"SELECT (SELECT email FROM users LIMIT 1) AS x",
		"SELECT id, (SELECT email FROM users LIMIT 1) AS x FROM users",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatalf("ran, returning %v", result.Rows)
			}
		})
	}
}

// Naming the column in a filter is not reading its value, which is what a WHERE
// on it already is. A subquery filtering on it stays ordinary work.
func TestExecuteLocalFilterSubqueryPassesWithSeeDenied(t *testing.T) {
	for _, sql := range []string{
		"SELECT id FROM users WHERE EXISTS (SELECT 1 FROM users)",
		"SELECT id FROM users WHERE id IN (SELECT id FROM users)",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount != 2 {
				t.Fatalf("expected 2 rows, got %d", result.RowCount)
			}
			for i, row := range result.Rows {
				if row[0] == core.MaskedValue {
					t.Errorf("row %d: id wrongly masked", i)
				}
			}
		})
	}
}

// A result column no field accounts for refuses only over the columns that can
// reach the row. Counting rows a filter selected is the same work as counting
// rows a WHERE selected, and neither returns the hidden column.
func TestExecuteLocalAggregateOverFilterSubqueryPasses(t *testing.T) {
	for _, sql := range []string{
		"SELECT count(*) FROM users WHERE id > 0",
		"SELECT count(*) FROM users WHERE EXISTS (SELECT 1 FROM users)",
		"SELECT count(*) FROM users WHERE id IN (SELECT id FROM users WHERE age > 0)",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount != 1 || result.Rows[0][0] == core.MaskedValue {
				t.Fatalf("expected one unmasked count, got %+v", result.Rows)
			}
		})
	}
}

// setupContactsDB adds a second table sharing the column name, so a mask on one
// is attributable: only main.users.email is hidden.
func setupContactsDB(t *testing.T) (*sql.DB, *core.Metadata) {
	t.Helper()
	db, meta := setupUsersDB(t)
	if _, err := db.Exec(`CREATE TABLE contacts(id INTEGER PRIMARY KEY, email TEXT NOT NULL)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO contacts VALUES (1,'carol@example.com')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	meta.Schemas[0].Tables = append(meta.Schemas[0].Tables, core.Table{
		Name:       "contacts",
		PrimaryKey: []string{"id"},
		Columns: []core.Column{
			{Name: "id", Type: "INTEGER", IsPrimaryKey: true},
			{Name: "email", Type: "TEXT"},
		},
	})
	return db, meta
}

// A clause that chooses or orders rows returns none of them, so naming a
// visible column there must not mask a column of the same name that is shown.
func TestExecuteLocalClauseSubqueryDoesNotMask(t *testing.T) {
	for _, sql := range []string{
		"SELECT c.email FROM contacts c WHERE EXISTS (SELECT u.id FROM users u)",
		"SELECT c.email FROM contacts c GROUP BY c.email HAVING EXISTS (SELECT u.id FROM users u)",
		"SELECT c.email FROM contacts c ORDER BY (SELECT u.id FROM users u LIMIT 1)",
		"SELECT c.email FROM contacts c LIMIT (SELECT count(u.id) FROM users u)",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount != 1 {
				t.Fatalf("expected one row, got %d", result.RowCount)
			}
			if result.Rows[0][0] != "carol@example.com" {
				t.Errorf("contacts.email = %v, want it shown", result.Rows[0][0])
			}
		})
	}
}

// A result column that is not a column of a table sits beside masked ones all
// the time: a literal, a count, a window function. It must not turn masking
// into refusal, since every hidden column here has a position to mask.
func TestExecuteLocalComputedColumnBesideMaskedOne(t *testing.T) {
	for _, sql := range []string{
		"SELECT id, email, 'lit' AS tag FROM users ORDER BY id",
		"SELECT email, count(*) FROM users GROUP BY id ORDER BY id",
		"SELECT id, email, row_number() OVER (ORDER BY id) AS rn FROM users ORDER BY id",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount == 0 {
				t.Fatal("no rows, so nothing here was checked")
			}
			masked := false
			for _, v := range result.Rows[0] {
				if v == core.MaskedValue {
					masked = true
				}
			}
			if !masked {
				t.Errorf("email reached the row: %+v", result.Rows[0])
			}
		})
	}
}

// Two scopes may spell a column the same way. The statement's own projection
// says which one comes out, so a hidden column elsewhere must not claim it.
func TestExecuteLocalNameReusedInAnotherScope(t *testing.T) {
	for _, sql := range []string{
		"SELECT u.id FROM users u JOIN (SELECT email AS id FROM users) q ON 1=1 LIMIT 1",
		"WITH q AS (SELECT email AS id FROM users) SELECT id FROM users ORDER BY id LIMIT 1",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount != 1 {
				t.Fatalf("expected one row, got %d", result.RowCount)
			}
			if result.Rows[0][0] == core.MaskedValue {
				t.Error("users.id masked by a name another scope reuses")
			}
		})
	}
}

// A CTE named after a table reads the CTE, not the table. Reading the table's
// columns as well hides a value the role may see, since a hidden column of the
// shadowed table claims the result column the CTE returns.
func TestExecuteLocalCTEShadowingATableDoesNotMask(t *testing.T) {
	db, meta := setupContactsDB(t)
	conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
		core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("contacts"),
			ColumnName: sptr("email"), Action: "see", Effect: "deny"},
	)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "WITH contacts AS (SELECT email FROM users) SELECT * FROM contacts ORDER BY email")
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.RowCount == 0 {
		t.Fatal("no rows, so nothing here was checked")
	}
	if result.Rows[0][0] == core.MaskedValue {
		t.Errorf("users.email masked by a rule on the table the CTE shadows")
	}
}

// A qualified name is the real table even where a CTE shadows the bare one, so
// the star over it selects the table's columns and a rule on them still hides
// their values.
func TestExecuteLocalQualifiedNameIsNotTheCTE(t *testing.T) {
	db, meta := setupContactsDB(t)
	conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
		core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("contacts"),
			ColumnName: sptr("email"), Action: "see", Effect: "deny"},
	)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "WITH contacts AS (SELECT email FROM users) SELECT * FROM main.contacts")
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.RowCount == 0 {
		t.Fatal("no rows, so nothing here was checked")
	}
	email := slices.Index(result.Columns, "email")
	if email < 0 {
		t.Fatalf("no email column in %v, so nothing here was checked", result.Columns)
	}
	if result.Rows[0][email] != core.MaskedValue {
		t.Errorf("contacts.email = %v, want %q: the CTE shadows the bare name only",
			result.Rows[0][email], core.MaskedValue)
	}
}

// A derived table resolves to the columns it reads. A CTE anywhere in the
// statement must not change that: the inspector walks its subqueries by
// position, and handing it the wrong list left the alias resolving to a table
// that does not exist, which no rule names and every wildcard allows.
func TestExecuteLocalDerivedTableUnderACTEStillMasks(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM (SELECT email FROM users) q",
		"WITH z AS (SELECT 1) SELECT * FROM (SELECT email FROM users) q",
		"WITH z AS (SELECT id FROM contacts) SELECT * FROM (SELECT email FROM users) q",
		"WITH contacts AS (SELECT email FROM users) SELECT * FROM (SELECT * FROM users) q",
		"WITH a AS (SELECT id FROM users), b AS (SELECT id FROM contacts) SELECT * FROM (SELECT email FROM users) q",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount == 0 {
				t.Fatal("no rows, so nothing here was checked")
			}
			email := slices.Index(result.Columns, "email")
			if email < 0 {
				t.Fatalf("no email column in %v, so nothing here was checked", result.Columns)
			}
			if result.Rows[0][email] != core.MaskedValue {
				t.Errorf("users.email = %v, want %q", result.Rows[0][email], core.MaskedValue)
			}
		})
	}
}

// A CTE may rename what it returns through a column list, which the inspectors
// do not follow. The column then names no table, so it cannot be masked; it
// must not be shown either.
func TestExecuteLocalCTEColumnListDoesNotLeak(t *testing.T) {
	for _, sql := range []string{
		"WITH r(x) AS (SELECT email FROM users) SELECT x FROM r",
		"WITH RECURSIVE r(x) AS (SELECT email FROM users LIMIT 1) SELECT x FROM r",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				return // refused, which hides it too
			}
			for i, row := range result.Rows {
				for _, v := range row {
					if s, ok := v.(string); ok && strings.Contains(s, "@example.com") {
						t.Errorf("row %d returned users.email in full: %v", i, v)
					}
				}
			}
		})
	}
}

// RETURNING hands rows back from a write, and they are rows like any other. A
// role that may change a row but not see a column must not read the column by
// asking a no-op write to return it.
func TestExecuteLocalReturningMasksHiddenColumn(t *testing.T) {
	for _, sql := range []string{
		"UPDATE users SET age = age WHERE id = 1 RETURNING email",
		"UPDATE users SET age = age WHERE id = 1 RETURNING id, email",
		"UPDATE users SET age = age WHERE id = 1 RETURNING *",
		"DELETE FROM users WHERE id = 2 RETURNING email",
		"INSERT INTO users (id, email) VALUES (9, 'ninth@example.com') RETURNING email",
		"INSERT INTO users (id, email) VALUES (9, 'ninth@example.com') RETURNING *",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "insert", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "delete", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				return // refused, which hides it too
			}
			if result.RowCount == 0 {
				t.Fatal("no rows, so nothing here was checked")
			}
			for i, row := range result.Rows {
				for _, v := range row {
					if s, ok := v.(string); ok && strings.Contains(s, "@example.com") {
						t.Errorf("row %d returned users.email in full: %v", i, row)
					}
				}
			}
		})
	}
}

// The row a write returns that the role may see comes back in full.
func TestExecuteLocalReturningShowsVisibleColumns(t *testing.T) {
	db, meta := setupUsersDB(t)
	conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
		core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
			ColumnName: sptr("email"), Action: "see", Effect: "deny"},
	)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := runQuery(ctx, conn, "UPDATE users SET age = 31 WHERE id = 1 RETURNING id, age")
	if len(result.Errors) != 0 {
		t.Fatalf("ordinary work refused: %v", result.Errors)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected one row, got %d", result.RowCount)
	}
	for _, v := range result.Rows[0] {
		if v == core.MaskedValue {
			t.Errorf("a column the role may see was masked: %v", result.Rows[0])
		}
	}
}

// A filter compares what it selects against something, which answers a question
// about those values exactly as a WHERE on them does. The shortest form is a
// scalar subquery: each spelling of the pattern returns a row count, and enough
// of them are the value.
func TestExecuteLocalFilterSelectingAHiddenColumnIsRefused(t *testing.T) {
	for _, sql := range []string{
		"SELECT count(*) FROM users WHERE (SELECT email FROM users WHERE id = 1) LIKE 'a%'",
		"SELECT id FROM users WHERE EXISTS (SELECT email FROM users)",
		"SELECT id FROM users WHERE id IN (SELECT id FROM users WHERE email > '')",
		"SELECT id FROM users ORDER BY (SELECT email FROM users LIMIT 1)",
		"SELECT id FROM users GROUP BY id HAVING EXISTS (SELECT email FROM users)",
		"UPDATE users SET age = 1 WHERE (SELECT email FROM users WHERE id = 1) LIKE 'a%'",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatalf("ran, returning %v", result.Rows)
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
		})
	}
}

// GROUP BY, ORDER BY, HAVING and a JOIN condition read a column without
// projecting it, so each one answers questions about a hidden column a row at a
// time: the ordering of two rows, whether a group exists, whether a join
// matched. They are the same oracle as WHERE and are refused the same way.
func TestExecuteLocalOrderByHiddenColumnRefused(t *testing.T) {
	for _, sql := range []string{
		"SELECT id FROM users ORDER BY email",
		"SELECT id FROM users ORDER BY email DESC LIMIT 1",
		"SELECT count(*) FROM users GROUP BY email",
		"SELECT id, count(*) FROM users GROUP BY id HAVING max(email) > 'm'",
		"SELECT DISTINCT email FROM users",
		"SELECT id, row_number() OVER (ORDER BY email) AS rn FROM users",
		"SELECT id, count(*) OVER (PARTITION BY email) AS n FROM users",
		"SELECT id FROM (SELECT id, count(*) FILTER (WHERE email LIKE 'a%') AS n FROM users GROUP BY id) s WHERE s.n > 0",
		"SELECT u.id FROM users u JOIN contacts c ON c.email = u.email",
		"SELECT u.id FROM users u JOIN contacts c USING (email)",
		"SELECT u.id FROM users u NATURAL JOIN contacts c",
		"SELECT u.id FROM users u LEFT JOIN contacts c ON c.id = u.id AND u.email > 'm'",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatalf("ran, returning %v", result.Rows)
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
		})
	}
}

// The same clauses on a visible column are ordinary work. A single hidden
// column in the projection is masked, not refused.
func TestExecuteLocalOrderByVisibleColumnRuns(t *testing.T) {
	for _, sql := range []string{
		"SELECT id, email FROM users ORDER BY id",
		"SELECT age, count(*) FROM users GROUP BY age HAVING count(*) > 0 ORDER BY age",
		"SELECT u.id, u.email FROM users u JOIN contacts c ON c.id = u.id ORDER BY u.id",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount == 0 {
				t.Fatal("no rows, so nothing here was checked")
			}
			if i := slices.Index(result.Columns, "email"); i >= 0 {
				if result.Rows[0][i] != core.MaskedValue {
					t.Errorf("email = %v, want masked", result.Rows[0][i])
				}
			}
		})
	}
}

// A write stores what it reads, and stored rows are out of reach of masking: a
// hidden column copied into a table the caller may select from is the value
// itself, not an answer about it.
func TestExecuteLocalWriteReadingHiddenColumnIsRefused(t *testing.T) {
	for _, sql := range []string{
		"INSERT INTO contacts (email) SELECT email FROM users",
		"INSERT INTO contacts (id, email) VALUES (9, (SELECT email FROM users LIMIT 1))",
		"UPDATE contacts SET email = (SELECT email FROM users LIMIT 1)",
		"INSERT INTO contacts (email) SELECT x FROM (SELECT email AS x FROM users) s",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "insert", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatal("ran, so the hidden values are in contacts now")
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
			var stored string
			_ = db.QueryRow(`SELECT coalesce(group_concat(email), '') FROM contacts`).Scan(&stored)
			if strings.Contains(stored, "@example.com") && strings.Contains(stored, "alice") {
				t.Errorf("hidden value reached contacts: %q", stored)
			}
		})
	}
}

// A write that reads only visible columns is ordinary work.
func TestExecuteLocalWriteReadingVisibleColumnRuns(t *testing.T) {
	for _, sql := range []string{
		"INSERT INTO contacts (id, email) SELECT id + 10, 'x' FROM users",
		"UPDATE contacts SET id = id + 100",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "insert", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if result := runQuery(ctx, conn, sql); len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
		})
	}
}

// An UPDATE ... FROM filters on the relations it joins against, and its row
// count answers for them exactly as a WHERE on the target does.
func TestExecuteLocalWriteFilteringOnHiddenColumnIsRefused(t *testing.T) {
	for _, sql := range []string{
		"UPDATE contacts SET id = id + 1 FROM users WHERE users.email LIKE 'a%'",
		"UPDATE contacts SET id = id + 1 FROM users WHERE users.id = contacts.id AND users.email > 'm'",
		"DELETE FROM contacts WHERE id IN (SELECT id FROM users WHERE email LIKE 'a%')",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "delete", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatal("ran, so the row count answered for the hidden column")
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
		})
	}
}

// A derived table or a CTE renames a hidden column without unhiding it. The
// outer statement testing it under the new name is the same oracle as testing
// it under the old one.
func TestExecuteLocalPredicateThroughDerivedTableIsRefused(t *testing.T) {
	for _, sql := range []string{
		"SELECT s.id FROM (SELECT id, email FROM users) s WHERE s.email LIKE 'a%'",
		"SELECT x FROM (SELECT id AS x, email FROM users) s WHERE s.email LIKE 'a%'",
		"SELECT x FROM (SELECT id AS x, email FROM users) s ORDER BY s.email",
		"WITH q AS (SELECT id, email FROM users) SELECT q.id FROM q WHERE q.email LIKE 'a%'",
		"WITH q AS (SELECT id, email FROM users) SELECT q.id FROM q GROUP BY q.email",
		"WITH q AS (SELECT id, email FROM users) SELECT q.id FROM q ORDER BY q.email",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatalf("ran, returning %d rows, which answers for the hidden column", result.RowCount)
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
		})
	}
}

// The same shapes over a visible column stay ordinary work.
func TestExecuteLocalPredicateThroughDerivedTableRuns(t *testing.T) {
	for _, sql := range []string{
		"SELECT s.id, s.email FROM (SELECT id, email FROM users) s WHERE s.id > 0",
		"WITH q AS (SELECT id, email FROM users) SELECT q.id, q.email FROM q ORDER BY q.id",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: seeEmailDenied()}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
			if result.RowCount == 0 {
				t.Fatal("no rows, so nothing here was checked")
			}
			if i := slices.Index(result.Columns, "email"); i >= 0 && result.Rows[0][i] != core.MaskedValue {
				t.Errorf("email = %v, want masked", result.Rows[0][i])
			}
		})
	}
}

// An upsert chooses which rows it updates, and the predicate it chooses them
// with reads the stored row, not the one being inserted.
func TestExecuteLocalUpsertPredicateOnHiddenColumnIsRefused(t *testing.T) {
	for _, sql := range []string{
		"INSERT INTO users (id, age) VALUES (1, 9) ON CONFLICT (id) DO UPDATE SET age = 9 WHERE users.email LIKE 'a%'",
		"INSERT INTO users (id, age) VALUES (1, 9) ON CONFLICT (id) DO UPDATE SET age = length(users.email)",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "insert", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatal("ran, so the rows it touched answer for the hidden column")
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
		})
	}
}

// An upsert that names no hidden column is ordinary work, including one that
// writes to a hidden column: storing a value is not reading it.
func TestExecuteLocalUpsertOnVisibleColumnsRuns(t *testing.T) {
	for _, sql := range []string{
		"INSERT INTO users (id, email, age) VALUES (1, 'z', 9) ON CONFLICT (id) DO UPDATE SET age = 9 WHERE users.id = 1",
		"INSERT INTO users (id, email) VALUES (1, 'x') ON CONFLICT (id) DO UPDATE SET email = 'y'",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupUsersDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "insert", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "update", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if result := runQuery(ctx, conn, sql); len(result.Errors) != 0 {
				t.Fatalf("ordinary work refused: %v", result.Errors)
			}
		})
	}
}

// Derived tables are paired with the statements behind them by the order the
// FROM list names them. Aliases that do not read in that order used to pair an
// alias with another subquery's columns, which resolved a hidden column to
// nothing and let the predicate through.
func TestExecuteLocalDerivedTableAliasesOutOfOrder(t *testing.T) {
	for _, sql := range []string{
		"SELECT z.id FROM (SELECT id, email FROM users) z, (SELECT id FROM contacts) y WHERE z.email LIKE 'a%'",
		"SELECT z.id FROM (SELECT id, email FROM users) z JOIN (SELECT id FROM contacts) b ON b.id = z.id ORDER BY z.email",
	} {
		t.Run(sql, func(t *testing.T) {
			db, meta := setupContactsDB(t)
			conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
				core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
				core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
					ColumnName: sptr("email"), Action: "see", Effect: "deny"},
			)}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runQuery(ctx, conn, sql)
			if len(result.Errors) == 0 {
				t.Fatalf("ran, returning %d rows, which answers for the hidden column", result.RowCount)
			}
			if !strings.Contains(strings.Join(result.Errors, " "), "see") {
				t.Errorf("refused for the wrong reason: %v", result.Errors)
			}
		})
	}
}

// The column each alias returns is the one its own subquery read, whichever
// order the aliases sort in.
func TestExecuteLocalDerivedTableAliasesKeepTheirColumns(t *testing.T) {
	db, meta := setupContactsDB(t)
	conn := Conn{DB: db, Meta: meta, Perms: compileFor("db1",
		core.PermissionEntry{SchemaName: sptr("main"), Action: "select", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), Action: "see", Effect: "allow"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("users"),
			ColumnName: sptr("email"), Action: "see", Effect: "deny"},
	)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// z reads the hidden column, y a visible one of another table.
	result := runQuery(ctx, conn,
		"SELECT z.email, y.id FROM (SELECT email FROM users) z, (SELECT id FROM contacts) y")
	if len(result.Errors) != 0 {
		t.Fatalf("ordinary work refused: %v", result.Errors)
	}
	if result.RowCount == 0 {
		t.Fatal("no rows, so nothing here was checked")
	}
	if result.Rows[0][0] != core.MaskedValue {
		t.Errorf("z.email = %v, want masked", result.Rows[0][0])
	}
	if result.Rows[0][1] == core.MaskedValue {
		t.Error("y.id wrongly masked")
	}
}
