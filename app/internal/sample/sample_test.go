package sample_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	sqlite "modernc.org/sqlite"

	"selectDb/internal/graph"
	"selectDb/internal/sample"
	"selectDb/internal/server"
)

func init() { sql.Register("sqlite3-sampletest", &sqlite.Driver{}) }

// seedInto points the app's path helpers at a throwaway directory and writes
// the sample workspace into it, the same call the app makes for a workspace it
// has just created.
func seedInto(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("APP_ENV", "dev")

	if err := server.WriteCurrentDomain(server.DefaultDomainForEnv()); err != nil {
		t.Fatalf("current domain: %v", err)
	}
	if err := sample.Write("test-workspace"); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	root, err := graph.WorkspaceRootPath("test-workspace")
	if err != nil {
		t.Fatalf("workspace root: %v", err)
	}
	return root
}

// A new workspace has to arrive complete. The guide walks a reader through
// this exact tree, so a missing file is a step in the documentation that
// cannot be followed.
func TestWriteSeedsTheDocumentedWorkspace(t *testing.T) {
	root := seedInto(t)

	for _, name := range []string{
		".lint",
		".gitignore",
		".env",
		"warehouse/db.config.json",
		"weekly_revenue.sql",
		"weekly_revenue.sql.metadata.json",
		"top_customers.sql",
		"cohorts.sql",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

// The database has to be connectable, not merely present: the placeholder this
// replaced pointed at a variable nothing defined, so a new workspace opened
// with a database that could never connect.
func TestWriteLeavesAQueryableDatabase(t *testing.T) {
	root := seedInto(t)

	env, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	dsn, ok := valueOf(string(env), "WAREHOUSE_DSN=")
	if !ok {
		t.Fatalf("no WAREHOUSE_DSN in .env:\n%s", env)
	}

	db, err := sql.Open("sqlite3-sampletest", dsn)
	if err != nil {
		t.Fatalf("open %s: %v", dsn, err)
	}
	defer func() { _ = db.Close() }()

	// The row counts the committed screenshots were taken of. They are fixed
	// because the generator is: a change here changes every published image.
	for table, want := range map[string]int{"customers": 240, "orders": 1065, "order_items": 2026} {
		var got int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Errorf("count %s: %v", table, err)
			continue
		}
		if got != want {
			t.Errorf("%s: %d rows, want %d", table, got, want)
		}
	}
}

// Nothing a person has touched may be overwritten. The app calls Write when it
// creates a workspace, and a second call must be able to add what is missing
// without taking anything back.
func TestWriteKeepsWhatIsAlreadyThere(t *testing.T) {
	root := seedInto(t)

	edited := filepath.Join(root, "weekly_revenue.sql")
	mine := []byte("SELECT 'mine';\n")
	if err := os.WriteFile(edited, mine, 0o600); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if err := sample.Write("test-workspace"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	after, err := os.ReadFile(edited)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) != string(mine) {
		t.Errorf("the second write clobbered an edited file:\n%s", after)
	}
}

// valueOf reads `key<value>` out of an .env, returning the first match.
func valueOf(env, key string) (string, bool) {
	for _, line := range splitLines(env) {
		if len(line) > len(key) && line[:len(key)] == key {
			return line[len(key):], true
		}
	}
	return "", false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
