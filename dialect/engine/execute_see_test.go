package engine

import (
	"context"
	"database/sql"
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

func TestExecuteLocalWherePassesWithSeeDenied(t *testing.T) {
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

	// WHERE filter on see-denied column must still work (the engine evaluates
	// it; the value never leaves). Result projects only see-allowed columns.
	result := runQuery(ctx, conn, "SELECT id, age FROM users WHERE email = 'alice@example.com'")
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
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
	// gets masked. This verifies the default-deny behavior.
	result := runQuery(ctx, conn, "SELECT email FROM users ORDER BY id")
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
		"SELECT id FROM users WHERE EXISTS (SELECT email FROM users)",
		"SELECT id FROM users WHERE id IN (SELECT id FROM users WHERE email > '')",
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
		"SELECT count(*) FROM users WHERE email > ''",
		"SELECT count(*) FROM users WHERE EXISTS (SELECT email FROM users)",
		"SELECT count(*) FROM users WHERE id IN (SELECT id FROM users WHERE email > '')",
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

// A clause that chooses or orders rows returns none of them, so naming the
// hidden column there must not mask a column of the same name that is shown.
func TestExecuteLocalClauseSubqueryDoesNotMask(t *testing.T) {
	for _, sql := range []string{
		"SELECT c.email FROM contacts c WHERE EXISTS (SELECT u.email FROM users u)",
		"SELECT c.email FROM contacts c GROUP BY c.email HAVING EXISTS (SELECT u.email FROM users u)",
		"SELECT c.email FROM contacts c ORDER BY (SELECT u.email FROM users u LIMIT 1)",
		"SELECT c.email FROM contacts c LIMIT (SELECT count(u.email) FROM users u)",
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
		"SELECT email, count(*) FROM users GROUP BY email ORDER BY email",
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
