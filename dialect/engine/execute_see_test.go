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
