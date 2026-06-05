package syncer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// newTestDB opens an in-memory SQLite DB and runs all migrations.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	// Single connection so all queries share the same in-memory DB.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	goose.SetBaseFS(nil)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(db, "../migrations"))
	return db
}

// newTestSyncer creates a Syncer whose HTTP calls go to the provided handler.
func newTestSyncer(t *testing.T, db *sql.DB, handler http.Handler) *Syncer {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	s := &Syncer{
		Queries: generated.New(db),
		FetchFunc: func(ctx context.Context, method, endpoint string, payload interface{}, headers map[string]string, response interface{}) error {
			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			req, err := http.NewRequestWithContext(ctx, method, srv.URL+"/"+endpoint, bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := srv.Client().Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if response != nil {
				return json.Unmarshal(b, response)
			}
			return nil
		},
	}
	return s
}

// fakeSyncHandler returns a handler that always responds with the given SyncResponse.
func fakeSyncHandler(res SyncResponse) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})
}

// seedUser inserts a user row and returns its ID.
func seedUser(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO user (id, name) VALUES (?, ?)`, id, name)
	require.NoError(t, err)
}

// seedWorkspace inserts a workspace row.
func seedWorkspace(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO workspace (id, name) VALUES (?, ?)`, id, name)
	require.NoError(t, err)
}

// seedWorkspaceToUser inserts a workspace_to_user row.
func seedWorkspaceToUser(t *testing.T, db *sql.DB, id, workspaceID, userID string, current bool) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO workspace_to_user (id, workspace_id, user_id, current) VALUES (?, ?, ?, ?)`,
		id, workspaceID, userID, current)
	require.NoError(t, err)
}

// seedRole inserts a role row.
func seedRole(t *testing.T, db *sql.DB, id, workspaceID, name string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO role (id, workspace_id, name) VALUES (?, ?, ?)`, id, workspaceID, name)
	require.NoError(t, err)
}

// seedCommit inserts a pending mutation_commit row.
func seedCommit(t *testing.T, db *sql.DB, id, userID, workspaceID, tableName string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO mutation_commit (id, created_at, operation, table_name, object_id, payload, user_id, workspace_id)
		 VALUES (?, datetime('now'), 'UPDATE', ?, ?, '{}', ?, ?)`,
		id, tableName, id, userID, workspaceID,
	)
	require.NoError(t, err)
}
