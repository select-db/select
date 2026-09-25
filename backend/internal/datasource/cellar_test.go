package datasource_test

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"backend/e2e"
	"backend/internal/auth"
	"backend/internal/cellar"
	"backend/internal/datasource"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/stretchr/testify/require"
)

type managedDB struct {
	f   e2e.Fixture
	id  string
	dir string
}

// newManagedDB seeds a managed database the owner may see, read, write and
// manage but not delete from, served by a cellar over a temp dir.
func newManagedDB(t *testing.T) managedDB {
	t.Helper()
	f := e2e.Setup(t)
	id := uuid.NewString()
	_, err := f.Conn.Exec(`INSERT INTO app.datasource (id, workspace_id, name, db_type, cellar_id, state)
		VALUES ($1::uuid, $2::uuid, 'notes', 'sqlite', 'local', 'hot')`, id, f.Actor.WorkspaceID)
	require.NoError(t, err)
	for _, action := range []string{"see", "select", "insert", "manage"} {
		_, err := f.Conn.Exec(`INSERT INTO app.permission (role_id, workspace_id, db_instance_id, action, effect)
			VALUES ($1::uuid, $2::uuid, $3, $4, 'allow')`, f.Actor.RoleID, f.Actor.WorkspaceID, id, action)
		require.NoError(t, err)
	}

	dir := t.TempDir()
	file, err := sql.Open("sqlite", filepath.Join(dir, id+".db"))
	require.NoError(t, err)
	_, err = file.Exec(`CREATE TABLE note (id INTEGER PRIMARY KEY, body TEXT); INSERT INTO note (body) VALUES ('hello')`)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	pub, err := auth.PublicKey()
	require.NoError(t, err)
	srv := httptest.NewServer(cellar.Handler(cellar.NewFiles(dir), pub, "local"))
	t.Cleanup(srv.Close)
	datasource.UseCellar(datasource.NewCellarClient(srv.URL))
	t.Cleanup(func() { datasource.UseCellar(nil) })
	return managedDB{f: f, id: id, dir: dir}
}

// run executes stmt through the backend and returns its rows or its error.
func (m managedDB) run(t *testing.T, stmt string) ([][]any, string) {
	t.Helper()
	rec := e2e.Do(t, m.f.H, http.MethodPost, "/datasources/"+m.id+"/execute", m.f.Actor.Token,
		map[string]any{"workspace_id": m.f.Actor.WorkspaceID, "sql": stmt})
	require.Equalf(t, http.StatusOK, rec.Code, "execute: %s", rec.Body.String())
	stream, err := arrowstream.NewStream(io.NopCloser(rec.Body))
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()
	if _, err := stream.Columns(); err != nil {
		return nil, err.Error()
	}
	var rows [][]any
	for {
		row, ok, err := stream.Next()
		if err != nil {
			return nil, err.Error()
		}
		if !ok {
			break
		}
		rows = append(rows, row)
	}
	if _, _, _, err := stream.Summary(); err != nil {
		return nil, err.Error()
	}
	return rows, ""
}

func TestManagedDatabaseRunsOnTheCellar(t *testing.T) {
	m := newManagedDB(t)

	rows, errMsg := m.run(t, "SELECT body FROM note")
	require.Empty(t, errMsg)
	require.Equal(t, [][]any{{"hello"}}, rows)

	_, errMsg = m.run(t, "INSERT INTO note (body) VALUES ('again')")
	require.Empty(t, errMsg)

	_, errMsg = m.run(t, "DELETE FROM note")
	require.Contains(t, errMsg, "permission", "the caller's permissions reach the cellar")

	e2e.RequireEvent(t, m.f.Conn, "query", "executed")

	path := "/datasources/" + m.id
	rec := e2e.Do(t, m.f.H, http.MethodGet, path+"/schema", m.f.Actor.Token, map[string]any{"workspace_id": m.f.Actor.WorkspaceID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = e2e.Do(t, m.f.H, http.MethodPost, path+"/ping", m.f.Actor.Token, map[string]any{"workspace_id": m.f.Actor.WorkspaceID})
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = e2e.Do(t, m.f.H, http.MethodGet, path+"/dump", m.f.Actor.Token, map[string]any{"workspace_id": m.f.Actor.WorkspaceID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestManagedDatabaseRefusesHostileSQL(t *testing.T) {
	m := newManagedDB(t)
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
			_, errMsg := m.run(t, stmt)
			require.NotEmpty(t, errMsg)
			require.NotContains(t, errMsg, m.dir, "errors never name a path")
		})
	}
	require.NoFileExists(t, outside)

	_, errMsg := m.run(t, "PRAGMA table_info(note)")
	require.Empty(t, errMsg, "introspection PRAGMAs are allowed")
}

func TestManagedDatabaseWithoutCellar(t *testing.T) {
	m := newManagedDB(t)
	datasource.UseCellar(nil)
	rec := e2e.Do(t, m.f.H, http.MethodPost, "/datasources/"+m.id+"/execute", m.f.Actor.Token,
		map[string]any{"workspace_id": m.f.Actor.WorkspaceID, "sql": "SELECT 1"})
	require.Equal(t, http.StatusNotImplemented, rec.Code)
	require.Contains(t, rec.Body.String(), "not enabled")
}

func TestManagedDatabaseWithCellarDown(t *testing.T) {
	m := newManagedDB(t)
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	datasource.UseCellar(datasource.NewCellarClient(down.URL))

	_, errMsg := m.run(t, "SELECT 1")
	require.Contains(t, errMsg, "unavailable")
	require.NotContains(t, errMsg, strings.TrimPrefix(down.URL, "http://"), "errors never name the cellar")

	rec := e2e.Do(t, m.f.H, http.MethodGet, "/datasources/"+m.id+"/schema", m.f.Actor.Token,
		map[string]any{"workspace_id": m.f.Actor.WorkspaceID})
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.NotContains(t, rec.Body.String(), strings.TrimPrefix(down.URL, "http://"))
}
