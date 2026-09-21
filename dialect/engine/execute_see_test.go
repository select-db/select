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

// A predicate on a hidden column answers questions about its values one at a
// time, and enough answers are the value. Masking hides a column from the eye;

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

// A clause that chooses or orders rows returns none of them, so naming a
// A result column that is not a column of a table sits beside masked ones all
// the time: a literal, a count, a window function. It must not turn masking
// Two scopes may spell a column the same way. The statement's own projection
// A CTE named after a table reads the CTE, not the table. Reading the table's
// columns as well hides a value the role may see, since a hidden column of the
// A qualified name is the real table even where a CTE shadows the bare one, so
// the star over it selects the table's columns and a rule on them still hides
// A derived table resolves to the columns it reads. A CTE anywhere in the
// statement must not change that: the inspector walks its subqueries by
// position, and handing it the wrong list left the alias resolving to a table
// A CTE may rename what it returns through a column list, which the inspectors
// do not follow. The column then names no table, so it cannot be masked; it
// RETURNING hands rows back from a write, and they are rows like any other. A
// role that may change a row but not see a column must not read the column by
// A filter compares what it selects against something, which answers a question
// about those values exactly as a WHERE on them does. The shortest form is a
// scalar subquery: each spelling of the pattern returns a row count, and enough
// GROUP BY, ORDER BY, HAVING and a JOIN condition read a column without
// projecting it, so each one answers questions about a hidden column a row at a
// time: the ordering of two rows, whether a group exists, whether a join
// The same clauses on a visible column are ordinary work. A single hidden
// A write stores what it reads, and stored rows are out of reach of masking: a
// hidden column copied into a table the caller may select from is the value
// An UPDATE ... FROM filters on the relations it joins against, and its row
// A derived table or a CTE renames a hidden column without unhiding it. The
// outer statement testing it under the new name is the same oracle as testing
// An upsert chooses which rows it updates, and the predicate it chooses them
// An upsert that names no hidden column is ordinary work, including one that
// Derived tables are paired with the statements behind them by the order the
// FROM list names them. Aliases that do not read in that order used to pair an
// alias with another subquery's columns, which resolved a hidden column to
// The column each alias returns is the one its own subquery read, whichever
// A derived table or a CTE that renames a hidden column hands it up under the
// Renaming a hidden column and returning it is still masking, not refusal, and
// The plain form of a set operator collapses duplicate rows, so the row count
// says whether a value the caller supplies is in the hidden column. ALL keeps
// A subquery is read against its own FROM, so a column it takes from the
// statement around it names a relation that is not in its scope. It is still
// A column written in quotes is the same column, and SQLite matches it
