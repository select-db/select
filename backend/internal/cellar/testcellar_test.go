package cellar

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/stretchr/testify/require"
)

// testCellar is a cellar over a temp dir, called the way the backend calls it.
type testCellar struct {
	dir        string
	replicaDir string
	databases  *Databases
	call       func(method, path string, body any) *httptest.ResponseRecorder
}

func newTestCellar(t *testing.T) testCellar {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	dir, replicaDir := t.TempDir(), t.TempDir()
	databases, err := OpenDatabases(dir, replicaDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = databases.Close(context.Background()) })
	mux := http.NewServeMux()
	Register(mux, databases, &privateKey.PublicKey, "local")
	grant, err := Grant{WorkspaceID: uuid.NewString(), CellarID: "local", MaxBytes: 1 << 20, MaxInFlight: 4}.Encode()
	require.NoError(t, err)
	token := signWith(t, privateKey)
	return testCellar{dir: dir, replicaDir: replicaDir, databases: databases, call: func(method, path string, body any) *httptest.ResponseRecorder {
		var encoded bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&encoded).Encode(body))
		}
		req := httptest.NewRequest(method, path, &encoded)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set(GrantHeader, grant)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}}
}

// createNotes creates database id holding a note table with rows a and b.
func (cellar testCellar) createNotes(t *testing.T, id string) {
	t.Helper()
	rec := cellar.call("PUT", "/datasources/"+id, CreateRequest{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cellar.exec(t, id, "CREATE TABLE note (body TEXT); INSERT INTO note VALUES ('a'), ('b')")
}

func (cellar testCellar) exec(t *testing.T, id, statement string) {
	t.Helper()
	conn, err := sql.Open("sqlite", filepath.Join(cellar.dir, id+".db"))
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Exec(statement)
	require.NoError(t, err)
}

// sync waits until the replica holds every write to database id.
func (cellar testCellar) sync(t *testing.T, id string) {
	t.Helper()
	db := cellar.databases.store.FindDB(filepath.Join(cellar.dir, id+".db"))
	require.NotNil(t, db, "not replicating")
	require.NoError(t, db.SyncAndWait(context.Background()))
}

func (cellar testCellar) replicating(id string) bool {
	return cellar.databases.store.FindDB(filepath.Join(cellar.dir, id+".db")) != nil
}

func (cellar testCellar) countNotes(t *testing.T, id string) int {
	t.Helper()
	conn, err := sql.Open("sqlite", "file:"+filepath.Join(cellar.dir, id+".db")+"?mode=ro")
	require.NoError(t, err)
	defer conn.Close()
	var count int
	require.NoError(t, conn.QueryRow("SELECT count(*) FROM note").Scan(&count))
	return count
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var coded arrowstream.Error
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&coded), rec.Body.String())
	return coded.Code
}
