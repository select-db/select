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
	dir       string
	bucketDir string
	call      func(method, path string, body any) *httptest.ResponseRecorder
}

func newTestCellar(t *testing.T) testCellar {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	dir, bucketDir := t.TempDir(), t.TempDir()
	require.NoError(t, OpenDatabases(dir, bucketDir))
	t.Cleanup(func() { _ = CloseDatabases(context.Background()) })
	mux := http.NewServeMux()
	Register(mux, &privateKey.PublicKey, "local")
	grant, err := Grant{WorkspaceID: uuid.NewString(), CellarID: "local", MaxBytes: 1 << 20, MaxInFlight: 4}.Encode()
	require.NoError(t, err)
	token := signWith(t, privateKey)
	return testCellar{dir: dir, bucketDir: bucketDir, call: func(method, path string, body any) *httptest.ResponseRecorder {
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

// sync waits until the bucket holds every write to database id.
func (cellar testCellar) sync(t *testing.T, id string) {
	t.Helper()
	_, err := databases.store.SyncDB(context.Background(), filepath.Join(cellar.dir, id+".db"), true)
	require.NoError(t, err)
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
