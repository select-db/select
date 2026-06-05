package sqlite

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
	sqlitedriver "modernc.org/sqlite"
)

func init() {
	// Register pure-Go SQLite driver for tests (app registers it in production).
	sql.Register("sqlite3", &sqlitedriver.Driver{})
}

func TestGetTablesAndViews(t *testing.T) {
	// Create a temporary SQLite database
	tmpFile, err := os.CreateTemp("", "test_schema_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()
	tmpFileName := tmpFile.Name()

	// Ensure cleanup of temporary file
	t.Cleanup(func() {
		// Remove file, ignore errors if it doesn't exist or is locked
		_ = os.Remove(tmpFileName)
	})

	// Create database with some test objects
	db, err := sql.Open("sqlite3", tmpFileName)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Create test view
	_, err = db.Exec(`
		CREATE VIEW test_view AS
		SELECT id, name FROM test_table;
	`)
	if err != nil {
		t.Fatalf("failed to create test view: %v", err)
	}

	// Test metadata discovery
	dialect := NewDialect()
	testDB, err := dialect.OpenDB("file:" + tmpFileName + "?mode=ro")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	ctx := context.Background()

	tables, err := dialect.GetTables(ctx, testDB, "main")
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	views, err := dialect.GetViews(ctx, testDB, "main")
	if err != nil {
		t.Fatalf("GetViews failed: %v", err)
	}

	// Verify tables
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	table := tables[0]
	if table.Name != "test_table" {
		t.Errorf("expected table name 'test_table', got %s", table.Name)
	}

	if len(table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.Columns))
	}

	// Verify columns
	columnMap := make(map[string]core.Column)
	for _, col := range table.Columns {
		columnMap[col.Name] = col
	}

	if col, ok := columnMap["id"]; !ok {
		t.Error("missing column 'id'")
	} else {
		if col.Nullable {
			t.Error("column 'id' should not be nullable")
		}
	}

	if col, ok := columnMap["name"]; !ok {
		t.Error("missing column 'name'")
	} else {
		if col.Nullable {
			t.Error("column 'name' should not be nullable")
		}
	}

	if col, ok := columnMap["email"]; !ok {
		t.Error("missing column 'email'")
	} else {
		if !col.Nullable {
			t.Error("column 'email' should be nullable")
		}
	}

	// Verify views
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	view := views[0]
	if view.Name != "test_view" {
		t.Errorf("expected view name 'test_view', got %s", view.Name)
	}

	if len(view.Columns) != 2 {
		t.Fatalf("expected 2 columns in view, got %d", len(view.Columns))
	}
}

func TestGetCurrentSchema(t *testing.T) {
	// Create a temporary SQLite database
	tmpFile, err := os.CreateTemp("", "test_current_schema_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()
	tmpFileName := tmpFile.Name()

	// Ensure cleanup of temporary file
	t.Cleanup(func() {
		_ = os.Remove(tmpFileName)
	})

	// Test metadata discovery
	dialect := NewDialect()
	testDB, err := dialect.OpenDB("file:" + tmpFileName + "?mode=ro")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	currentSchema, err := dialect.GetCurrentSchema(context.Background(), testDB)
	if err != nil {
		t.Fatalf("GetCurrentSchema failed: %v", err)
	}

	// SQLite should always return "main" as the current schema
	if currentSchema != "main" {
		t.Errorf("expected current schema 'main', got %s", currentSchema)
	}
}

func TestGetSchemaDDLFromMetadata(t *testing.T) {
	// Create a temporary SQLite database
	tmpFile, err := os.CreateTemp("", "test_ddl_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()
	tmpFileName := tmpFile.Name()

	// Ensure cleanup of temporary file
	t.Cleanup(func() {
		// Remove file, ignore errors if it doesn't exist or is locked
		_ = os.Remove(tmpFileName)
	})

	// Create database with test objects
	db, err := sql.Open("sqlite3", tmpFileName)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create test objects
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);

		CREATE INDEX idx_users_name ON users(name);

		CREATE VIEW user_names AS SELECT name FROM users;
	`)
	if err != nil {
		t.Fatalf("failed to create test objects: %v", err)
	}

	// Test DDL extraction from metadata
	dialect := NewDialect()
	testDB, err := dialect.OpenDB("file:" + tmpFileName + "?mode=ro")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	ctx := context.Background()

	// Get all objects with their DDL
	tables, err := dialect.GetTables(ctx, testDB, "main")
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	views, err := dialect.GetViews(ctx, testDB, "main")
	if err != nil {
		t.Fatalf("GetViews failed: %v", err)
	}

	indexes, err := dialect.GetIndexes(ctx, testDB, "main")
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	// Reconstruct DDL from metadata
	var ddlStatements []string
	for _, table := range tables {
		if table.DDL != "" {
			ddlStatements = append(ddlStatements, table.DDL)
		}
	}
	for _, view := range views {
		if view.DDL != "" {
			ddlStatements = append(ddlStatements, view.DDL)
		}
	}
	for _, index := range indexes {
		if index.DDL != "" {
			ddlStatements = append(ddlStatements, index.DDL)
		}
	}

	ddl := strings.Join(ddlStatements, "\n\n")

	if ddl == "" {
		t.Fatal("DDL is empty")
	}

	// Verify DDL contains expected statements
	if !strings.Contains(ddl, "CREATE TABLE users") {
		t.Error("DDL missing CREATE TABLE statement")
	}

	if !strings.Contains(ddl, "CREATE INDEX idx_users_name") {
		t.Error("DDL missing CREATE INDEX statement")
	}

	if !strings.Contains(ddl, "CREATE VIEW user_names") {
		t.Error("DDL missing CREATE VIEW statement")
	}
}
