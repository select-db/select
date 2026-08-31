package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithDefensiveMode(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "bare path gains a query string",
			dsn:  "/tmp/example.db",
			want: "/tmp/example.db?_defensive=1",
		},
		{
			name: "existing parameters are appended to",
			dsn:  "/tmp/example.db?_pragma=busy_timeout(5000)",
			want: "/tmp/example.db?_pragma=busy_timeout(5000)&_defensive=1",
		},
		{
			name: "file: URI keeps its parameters",
			dsn:  "file:/tmp/example.db?_journal_mode=WAL",
			want: "file:/tmp/example.db?_journal_mode=WAL&_defensive=1",
		},
		{
			name: "in-memory database",
			dsn:  ":memory:",
			want: ":memory:?_defensive=1",
		},
		{
			name: "empty query string is not doubled up",
			dsn:  "/tmp/example.db?",
			want: "/tmp/example.db?&_defensive=1",
		},
		{
			name: "explicit opt-out is honoured",
			dsn:  "/tmp/example.db?_defensive=0",
			want: "/tmp/example.db?_defensive=0",
		},
		{
			name: "explicit opt-in is not duplicated",
			dsn:  "/tmp/example.db?_defensive=1",
			want: "/tmp/example.db?_defensive=1",
		},
		{
			name: "journal_mode=OFF is left alone",
			dsn:  "/tmp/example.db?_journal_mode=OFF",
			want: "/tmp/example.db?_journal_mode=OFF",
		},
		{
			name: "journal_mode=off is matched case-insensitively",
			dsn:  "/tmp/example.db?_journal_mode=off",
			want: "/tmp/example.db?_journal_mode=off",
		},
		{
			name: "the _journal alias is honoured too",
			dsn:  "/tmp/example.db?_journal=OFF",
			want: "/tmp/example.db?_journal=OFF",
		},
		{
			name: "the alias wins over the primary key, as it does in the driver",
			dsn:  "/tmp/example.db?_journal_mode=OFF&_journal=WAL",
			want: "/tmp/example.db?_journal_mode=OFF&_journal=WAL&_defensive=1",
		},
		{
			name: "an unparseable query string is left to the driver to reject",
			dsn:  "/tmp/example.db?%zz",
			want: "/tmp/example.db?%zz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withDefensiveMode(tt.dsn); got != tt.want {
				t.Errorf("withDefensiveMode(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

// TestOpenDBEnablesDefensiveMode checks the flag actually reaches the
// connection, rather than only that the DSN was spelled correctly.
// PRAGMA writable_schema=ON is the operation defensive mode suppresses most
// visibly: SQLite reports success and leaves the setting off.
func TestOpenDBEnablesDefensiveMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "defensive.db")

	d := NewDialect()
	db, err := d.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec("PRAGMA writable_schema=ON"); err != nil {
		t.Fatalf("PRAGMA writable_schema=ON returned an error: %v", err)
	}

	var writable int
	if err := db.QueryRow("PRAGMA writable_schema").Scan(&writable); err != nil {
		t.Fatalf("failed to read back writable_schema: %v", err)
	}
	if writable != 0 {
		t.Errorf("writable_schema = %d, want 0: defensive mode did not take effect", writable)
	}
}

// TestOpenDBRespectsOptOut is the counterpart: without the safety net the same
// PRAGMA takes effect, so the test above is measuring defensive mode and not
// some unrelated default.
func TestOpenDBRespectsOptOut(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "permissive.db")

	d := NewDialect()
	db, err := d.OpenDB(dbPath + "?_defensive=0")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec("PRAGMA writable_schema=ON"); err != nil {
		t.Fatalf("PRAGMA writable_schema=ON returned an error: %v", err)
	}

	var writable int
	if err := db.QueryRow("PRAGMA writable_schema").Scan(&writable); err != nil {
		t.Fatalf("failed to read back writable_schema: %v", err)
	}
	if writable != 1 {
		t.Errorf("writable_schema = %d, want 1: the opt-out was not honoured", writable)
	}
}

// TestOpenDBLeavesOrdinaryWorkAlone guards the blast radius: defensive mode is
// only worth defaulting on if it is invisible to normal use of a database.
func TestOpenDBLeavesOrdinaryWorkAlone(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ordinary.db")

	d := NewDialect()
	db, err := d.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	statements := []string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`,
		`INSERT INTO items (name) VALUES ('first'), ('second')`,
		`CREATE INDEX items_name ON items (name)`,
		`CREATE VIEW item_names AS SELECT name FROM items`,
		`CREATE VIRTUAL TABLE docs USING fts5(body)`,
		`INSERT INTO docs (body) VALUES ('the quick brown fox')`,
		`VACUUM`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM docs WHERE docs MATCH 'fox'`).Scan(&count); err != nil {
		t.Fatalf("failed to query the fts5 table: %v", err)
	}
	if count != 1 {
		t.Errorf("fts5 match returned %d rows, want 1", count)
	}

	// Reading a shadow table stays allowed; only writing to one is blocked.
	if err := db.QueryRow(`SELECT count(*) FROM docs_data`).Scan(&count); err != nil {
		t.Fatalf("failed to read the fts5 shadow table: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO docs_data (id, block) VALUES (99, x'00')`); err == nil {
		t.Error("writing to an fts5 shadow table succeeded, want it blocked")
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file is missing after ordinary use: %v", err)
	}
}
