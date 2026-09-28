package cellar

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/stretchr/testify/require"
)

// testCellar is a cellar over a temp dir, called the way the backend calls it.
type testCellar struct {
	dir  string
	call func(method, path string, body any) *httptest.ResponseRecorder
}

func newTestCellar(t *testing.T) testCellar {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	dir := t.TempDir()
	mux := http.NewServeMux()
	Register(mux, dir, &privateKey.PublicKey, "local")
	grant, err := Grant{WorkspaceID: uuid.NewString(), CellarID: "local", MaxBytes: 1 << 20, MaxInFlight: 4}.Encode()
	require.NoError(t, err)
	token := signWith(t, privateKey)
	return testCellar{dir: dir, call: func(method, path string, body any) *httptest.ResponseRecorder {
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

func (cellar testCellar) exec(t *testing.T, id, statement string) {
	t.Helper()
	conn, err := sql.Open("sqlite", filepath.Join(cellar.dir, id+".db"))
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Exec(statement)
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

func TestCreateForkDownloadDelete(t *testing.T) {
	cellar := newTestCellar(t)
	id, forkID := uuid.NewString(), uuid.NewString()

	rec := cellar.call("PUT", "/datasources/"+id, CreateRequest{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cellar.exec(t, id, "CREATE TABLE note (body TEXT); INSERT INTO note VALUES ('a'), ('b')")

	rec = cellar.call("PUT", "/datasources/"+id, CreateRequest{})
	require.Equal(t, http.StatusConflict, rec.Code, "an id that exists is never overwritten")

	rec = cellar.call("PUT", "/datasources/"+forkID, CreateRequest{SourceID: id})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var stored StoredDatabase
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&stored))
	require.Equal(t, forkID, stored.ID)
	require.Positive(t, stored.SizeBytes)
	cellar.exec(t, forkID, "INSERT INTO note VALUES ('c')")
	require.Equal(t, 2, cellar.countNotes(t, id), "a fork never touches its source")
	require.Equal(t, 3, cellar.countNotes(t, forkID))

	rec = cellar.call("GET", "/datasources/"+id+"/download", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	downloadPath := filepath.Join(t.TempDir(), "copy.db")
	require.NoError(t, os.WriteFile(downloadPath, rec.Body.Bytes(), 0o600))
	downloaded, err := sql.Open("sqlite", downloadPath)
	require.NoError(t, err)
	var count int
	require.NoError(t, downloaded.QueryRow("SELECT count(*) FROM note").Scan(&count))
	require.NoError(t, downloaded.Close())
	require.Equal(t, 2, count)

	rec = cellar.call("GET", "/datasources", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var inventory []StoredDatabase
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&inventory))
	require.ElementsMatch(t, []string{id, forkID}, []string{inventory[0].ID, inventory[1].ID}, "no temporary copy is listed")

	rec = cellar.call("DELETE", "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, rec.Code)
	entries, err := os.ReadDir(cellar.dir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), id), "%s left behind", entry.Name())
		require.False(t, strings.HasPrefix(entry.Name(), ".tmp-"), "%s left behind", entry.Name())
	}
	rec = cellar.call("DELETE", "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, "deleting twice is not an error")
}

func TestLifecycleRefusals(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()

	rec := cellar.call("PUT", "/datasources/"+id, CreateRequest{SourceID: uuid.NewString()})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.NoFileExists(t, filepath.Join(cellar.dir, id+".db"), "a failed fork leaves no file")

	rec = cellar.call("PUT", "/datasources/"+id, CreateRequest{SourceID: id, PointInTime: "2026-09-28T00:00:00Z"})
	require.Equal(t, http.StatusNotImplemented, rec.Code)
	require.Equal(t, CodeDisabled, errorCode(t, rec))

	rec = cellar.call("GET", "/datasources/"+id+"/download", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = cellar.call("PUT", "/datasources/..%2Fescape", CreateRequest{})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.NotContains(t, string(body), cellar.dir, "an error never names a path")
}
