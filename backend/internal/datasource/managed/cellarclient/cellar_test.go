package cellarclient_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"backend/e2e"
	"backend/internal/cellar"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { e2e.Run(m) }

type managedDB struct {
	fixture e2e.Fixture
	id      string
	dir     string
}

// newManagedDB seeds a managed database the owner may see, read, write and
// manage but not delete from, served by a cellar over a temp dir.
func newManagedDB(t *testing.T) managedDB {
	t.Helper()
	fixture := e2e.Setup(t)
	id := uuid.NewString()
	_, err := fixture.Conn.Exec(`INSERT INTO app.datasource (id, workspace_id, name, db_type, cellar_id, state)
		VALUES ($1::uuid, $2::uuid, 'notes', 'sqlite', 'local', 'hot')`, id, fixture.Actor.WorkspaceID)
	require.NoError(t, err)
	for _, action := range []string{"see", "select", "insert", "manage"} {
		_, err := fixture.Conn.Exec(`INSERT INTO app.permission (role_id, workspace_id, datasource_id, action, effect)
			VALUES ($1::uuid, $2::uuid, $3, $4, 'allow')`, fixture.Actor.RoleID, fixture.Actor.WorkspaceID, id, action)
		require.NoError(t, err)
	}

	dir := e2e.ServeCellar(t)
	_, err = cellarclient.Create(context.Background(), cellarclient.DSN(cellarclient.CellarID, id, fixture.Actor.WorkspaceID, 1<<20, 0), "", "")
	require.NoError(t, err)
	seedConn, err := sql.Open("sqlite", filepath.Join(dir, id+".db"))
	require.NoError(t, err)
	_, err = seedConn.Exec(`CREATE TABLE note (id INTEGER PRIMARY KEY, body TEXT); INSERT INTO note (body) VALUES ('hello')`)
	require.NoError(t, err)
	require.NoError(t, seedConn.Close())
	return managedDB{fixture: fixture, id: id, dir: dir}
}

func TestManagedDatasourceRunsOnTheCellar(t *testing.T) {
	database := newManagedDB(t)

	rows, failure := e2e.Execute(t, database.fixture, database.id, "SELECT body FROM note")
	require.Nil(t, failure)
	require.Equal(t, [][]any{{"hello"}}, rows)

	_, failure = e2e.Execute(t, database.fixture, database.id, "INSERT INTO note (body) VALUES ('again')")
	require.Nil(t, failure)

	_, failure = e2e.Execute(t, database.fixture, database.id, "INSERT INTO note (body) VALUES ('once'); INSERT INTO note (id, body) VALUES (1, 'taken')")
	require.Equal(t, cellar.CodeSQLError, failure.Code)
	require.Contains(t, failure.Message, "UNIQUE constraint failed", "SQLite's message reaches the caller as it is")
	rows, _ = e2e.Execute(t, database.fixture, database.id, "SELECT count(*) FROM note WHERE body = 'once'")
	require.Equal(t, [][]any{{"1"}}, rows, "a failed script is never run twice")

	_, failure = e2e.Execute(t, database.fixture, database.id, "DELETE FROM note")
	require.Contains(t, failure.Message, "permission", "the backend answers permissions")

	e2e.RequireEvent(t, database.fixture.Conn, "query", "executed")

	path := "/datasources/" + database.id
	rec := e2e.Do(t, database.fixture.H, http.MethodGet, path+"/schema", database.fixture.Actor.Token, map[string]any{"workspace_id": database.fixture.Actor.WorkspaceID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = e2e.Do(t, database.fixture.H, http.MethodPost, path+"/ping", database.fixture.Actor.Token, map[string]any{"workspace_id": database.fixture.Actor.WorkspaceID})
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = e2e.Do(t, database.fixture.H, http.MethodGet, path+"/dump", database.fixture.Actor.Token, map[string]any{"workspace_id": database.fixture.Actor.WorkspaceID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestManagedDatasourceRefusesHostileSQL(t *testing.T) {
	database := newManagedDB(t)
	outside := filepath.Join(t.TempDir(), "out.db")

	for _, stmt := range []string{
		"ATTACH DATABASE '" + outside + "' AS o",
		"VACUUM INTO '" + outside + "'",
		"SELECT load_extension('/tmp/x')",
		"PRAGMA temp_store_directory = '/tmp'",
		"pragma 'temp_store_directory' = '/tmp'",
		"PRAGMA main.max_page_count = 1000000000",
		"PRAGMA /* c */ writable_schema = 1",
		"PRAGMA query_only = 0",
		"SELECT * FROM pragma_database_list",
		"SELECT * FROM \"PRAGMA_compile_options\"",
	} {
		t.Run(stmt, func(t *testing.T) {
			_, failure := e2e.Execute(t, database.fixture, database.id, stmt)
			// Refused by the backend's permission check (no code) or by the cellar.
			require.NotNil(t, failure)
			require.Contains(t, []string{"", cellar.CodeForbiddenStatement, cellar.CodeSQLError}, failure.Code)
			require.NotContains(t, failure.Message, database.dir, "errors never name a path")
		})
	}
	require.NoFileExists(t, outside)

	_, failure := e2e.Execute(t, database.fixture, database.id, "PRAGMA table_info(note)")
	require.Nil(t, failure, "introspection PRAGMAs are allowed")
}

func TestManagedDatasourceWithoutCellar(t *testing.T) {
	database := newManagedDB(t)
	cellarclient.URL = ""
	rec := e2e.Do(t, database.fixture.H, http.MethodPost, "/datasources/"+database.id+"/execute", database.fixture.Actor.Token,
		map[string]any{"workspace_id": database.fixture.Actor.WorkspaceID, "sql": "SELECT 1"})
	require.Equal(t, http.StatusNotImplemented, rec.Code)
	require.Contains(t, rec.Body.String(), "not enabled")
}

func TestManagedDatasourceWithCellarDown(t *testing.T) {
	database := newManagedDB(t)
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	cellarclient.URL = down.URL

	_, failure := e2e.Execute(t, database.fixture, database.id, "SELECT 1")
	require.Equal(t, cellar.CodeUnavailable, failure.Code)
	require.NotContains(t, failure.Message, strings.TrimPrefix(down.URL, "http://"), "errors never name the cellar")

	rec := e2e.Do(t, database.fixture.H, http.MethodGet, "/datasources/"+database.id+"/schema", database.fixture.Actor.Token,
		map[string]any{"workspace_id": database.fixture.Actor.WorkspaceID})
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.NotContains(t, rec.Body.String(), strings.TrimPrefix(down.URL, "http://"))
}
