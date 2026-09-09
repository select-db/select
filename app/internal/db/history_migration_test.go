package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"selectDb/internal/db/generated"

	"modernc.org/sqlite"
)

// Validates the SQLite-specific concerns introduced by 00012_history_columns:
// the ALTER ADD COLUMN rules, CURRENT_TIMESTAMP inserts, the 7-day window, and
// the window-function prune. Exercises the generated query layer end-to-end.
func TestHistoryMigrationAndQueries(t *testing.T) {
	if !slices.Contains(sql.Drivers(), "sqlite3") {
		sql.Register("sqlite3", &sqlite.Driver{})
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := RunMigrationsAt(dbPath); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	q := generated.New(conn)
	ctx := context.Background()

	// Insert 150 rows for one workspace; prune must cap at 100.
	for i := 0; i < 150; i++ {
		if _, err := q.CreateHistory(ctx, generated.CreateHistoryParams{
			ID:           "id-" + strconv.Itoa(i),
			Dsn:          "",
			Uri:          "",
			Statement:    "SELECT " + strconv.Itoa(i),
			Errors:       "[]",
			WorkspaceID:  "ws-1",
			DbInstanceID: "db-1",
		}); err != nil {
			t.Fatalf("CreateHistory %d: %v", i, err)
		}
	}
	// A second workspace, untouched by the per-workspace prune above.
	if _, err := q.CreateHistory(ctx, generated.CreateHistoryParams{
		ID: "other", Dsn: "", Uri: "", Statement: "SELECT 1", Errors: "[]",
		WorkspaceID: "ws-2", DbInstanceID: "db-2",
	}); err != nil {
		t.Fatalf("CreateHistory other: %v", err)
	}

	// Per-workspace keep-100 prune (mirrors the per-insert call).
	if err := q.PruneHistoryForWorkspace(ctx, "ws-1"); err != nil {
		t.Fatalf("PruneHistoryForWorkspace: %v", err)
	}

	rows, err := q.ListHistory(ctx, generated.ListHistoryParams{WorkspaceID: "ws-1", Limit: 1000, Offset: 0})
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(rows) != 100 {
		t.Fatalf("expected 100 rows after prune, got %d", len(rows))
	}
	if rows[0].CreatedAt.IsZero() {
		t.Fatalf("created_at not populated")
	}
	if rows[0].WorkspaceID != "ws-1" || rows[0].DbInstanceID != "db-1" {
		t.Fatalf("unexpected workspace/db ids: %+v", rows[0])
	}

	// Global window-function prune across all workspaces must not error.
	if err := q.PruneHistoryAllWorkspaces(ctx); err != nil {
		t.Fatalf("PruneHistoryAllWorkspaces: %v", err)
	}
	if err := q.DeleteHistoryOlderThan7Days(ctx); err != nil {
		t.Fatalf("DeleteHistoryOlderThan7Days: %v", err)
	}

	// ws-2 row survives (only 1 row, within keep-100 and within 7 days).
	ws2, err := q.ListHistory(ctx, generated.ListHistoryParams{WorkspaceID: "ws-2", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListHistory ws-2: %v", err)
	}
	if len(ws2) != 1 {
		t.Fatalf("expected ws-2 to retain 1 row, got %d", len(ws2))
	}
}

// 00018 rebuilds history to make created_at NOT NULL, which SQLite can express
// no other way. A rebuild moves every row, so this checks the rows survive the
// migration in both directions and that the constraint is real afterwards.
func TestHistoryCreatedAtRebuild(t *testing.T) {
	if !slices.Contains(sql.Drivers(), "sqlite3") {
		sql.Register("sqlite3", &sqlite.Driver{})
	}

	dbPath := filepath.Join(t.TempDir(), "rebuild.db")
	if err := RunMigrationsAt(dbPath); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()

	q := generated.New(conn)
	ctx := context.Background()
	for i := range 3 {
		if _, err := q.CreateHistory(ctx, generated.CreateHistoryParams{
			ID:           "rebuild-" + strconv.Itoa(i),
			Statement:    "SELECT " + strconv.Itoa(i),
			Errors:       "[]",
			WorkspaceID:  "ws-1",
			DbInstanceID: "db-1",
		}); err != nil {
			t.Fatalf("CreateHistory %d: %v", i, err)
		}
	}

	countRows := func(where string) int {
		t.Helper()
		var n int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM history "+where).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", where, err)
		}
		return n
	}

	// Down rebuilds back to a nullable column, Up rebuilds forward again. The
	// rows have to come through both copies.
	if err := RunGooseAt(dbPath, GooseDown, ""); err != nil {
		t.Fatalf("goose down: %v", err)
	}
	if n := countRows(""); n != 3 {
		t.Errorf("rolling back lost rows: got %d, want 3", n)
	}
	if err := RunGooseAt(dbPath, GooseUp, ""); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	if n := countRows(""); n != 3 {
		t.Errorf("re-applying lost rows: got %d, want 3", n)
	}
	if n := countRows("WHERE created_at IS NULL"); n != 0 {
		t.Errorf("%d rows came through with a null created_at", n)
	}

	// The point of the migration: the database refuses a null now, rather than
	// the codegen being told to assume one never arrives.
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO history (id, statement, errors, uri, dsn, created_at, workspace_id, db_instance_id)
		 VALUES ('null-created-at', 'SELECT 1', '[]', '', '', NULL, 'ws-1', 'db-1')`,
	); err == nil {
		t.Error("expected a NOT NULL violation inserting a null created_at")
	}
}
