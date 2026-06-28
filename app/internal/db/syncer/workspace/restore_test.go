package workspace

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"selectDb/internal/db/generated"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	sql.Register("sqlite3", new(sqlite.Driver))
	os.Exit(m.Run())
}

func newTestQueries(t *testing.T) *generated.Queries {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	goose.SetBaseFS(nil)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(db, "../../migrations"))
	return generated.New(db)
}

func seedWorkspaceWithLimits(t *testing.T, q *generated.Queries, id string, timeout, size int) {
	t.Helper()
	ctx := context.Background()
	_, err := q.CreateWorkspace(ctx, generated.CreateWorkspaceParams{ID: id, Name: "ws"})
	require.NoError(t, err)
	require.NoError(t, q.UpdateWorkspaceExecutionLimits(ctx, generated.UpdateWorkspaceExecutionLimitsParams{
		ID:                 id,
		StatementTimeoutMs: int64(timeout),
		MaxResultSizeMb:    int64(size),
	}))
}

// A pulled workspace row that omits the limit fields (older backend that does
// not yet manage them) must NOT reset the locally stored limits.
func TestRestore_PreservesLocalLimitsWhenPayloadOmitsThem(t *testing.T) {
	q := newTestQueries(t)
	const id = "ws-1"
	seedWorkspaceWithLimits(t, q, id, 5000, 50)

	payload := map[string]any{"id": id, "name": "ws"}
	require.NoError(t, Restore(context.Background(), q, payload, nil))

	got, err := q.GetWorkspaceByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, int64(5000), got.StatementTimeoutMs, "statement timeout must be preserved")
	require.Equal(t, int64(50), got.MaxResultSizeMb, "max result size must be preserved")
}

// When the payload carries the limit fields (backend manages them), they win.
func TestRestore_AppliesLimitsFromPayload(t *testing.T) {
	q := newTestQueries(t)
	const id = "ws-2"
	seedWorkspaceWithLimits(t, q, id, 5000, 50)

	payload := map[string]any{
		"id":                   id,
		"name":                 "ws",
		"statement_timeout_ms": float64(8000), // JSON numbers decode to float64
		"max_result_size_mb":   float64(60),
	}
	require.NoError(t, Restore(context.Background(), q, payload, nil))

	got, err := q.GetWorkspaceByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, int64(8000), got.StatementTimeoutMs)
	require.Equal(t, int64(60), got.MaxResultSizeMb)
}

// A brand-new workspace pulled without limit fields gets the column defaults.
func TestRestore_NewWorkspaceUsesDefaults(t *testing.T) {
	q := newTestQueries(t)
	const id = "ws-3"

	payload := map[string]any{"id": id, "name": "ws"}
	require.NoError(t, Restore(context.Background(), q, payload, nil))

	got, err := q.GetWorkspaceByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, int64(30000), got.StatementTimeoutMs)
	require.Equal(t, int64(100), got.MaxResultSizeMb)
}
