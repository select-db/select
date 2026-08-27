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

// A valid 1x1 base64 PNG: short, and shaped like what the server stores. The
// local column's CHECK constraint accepts it, which is half the point — a value
// that failed the constraint would fail the upsert.
const testLogo = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func restoreWorkspace(t *testing.T, q *generated.Queries, payload map[string]any) generated.GetWorkspaceByIDRow {
	t.Helper()
	require.NoError(t, Restore(context.Background(), q, payload, nil))
	row, err := q.GetWorkspaceByID(context.Background(), "ws-1")
	require.NoError(t, err)
	return row
}

// The logo is server-authoritative on the pull path: what the server sends wins,
// including an explicit null, which is how a removal reaches other members.
func TestRestore_Logo(t *testing.T) {
	q := newTestQueries(t)

	row := restoreWorkspace(t, q, map[string]any{"id": "ws-1", "name": "ws", "logo": testLogo})
	require.Equal(t, testLogo, row.Logo.Or(""), "a pulled logo should be stored")

	row = restoreWorkspace(t, q, map[string]any{"id": "ws-1", "name": "ws", "logo": nil})
	require.False(t, row.Logo.Valid, "an explicit null should clear the logo")
}

// A payload from a server that predates the column omits the key entirely; the
// local value has to survive that rather than being wiped on every pull.
func TestRestore_LogoAbsentKeyKeepsLocalValue(t *testing.T) {
	q := newTestQueries(t)

	restoreWorkspace(t, q, map[string]any{"id": "ws-1", "name": "ws", "logo": testLogo})
	row := restoreWorkspace(t, q, map[string]any{"id": "ws-1", "name": "ws"})
	require.Equal(t, testLogo, row.Logo.Or(""), "an absent key should leave the logo alone")
}

// The local mirror carries the same CHECK the backend column does, so a value
// that is not base64 PNG cannot land even if it somehow reached the client.
func TestRestore_LogoConstraintRejectsNonPNG(t *testing.T) {
	q := newTestQueries(t)

	err := Restore(context.Background(), q, map[string]any{
		"id": "ws-1", "name": "ws",
		"logo": "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjwvc3ZnPg==",
	}, nil)
	require.Error(t, err, "the local CHECK constraint should refuse a non-PNG logo")
}
