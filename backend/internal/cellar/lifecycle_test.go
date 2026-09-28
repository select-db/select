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

// lifecycleCellar is a cellar over a temp dir and a way to call its routes as
// the backend would.
type lifecycleCellar struct {
	dir  string
	call func(method, path string, body any) *httptest.ResponseRecorder
}

func newLifecycleCellar(t *testing.T) lifecycleCellar {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	dir := t.TempDir()
	mux := http.NewServeMux()
	Register(mux, dir, &priv.PublicKey, "local")
	grant, err := Grant{WorkspaceID: uuid.NewString(), CellarID: "local", MaxBytes: 1 << 20, MaxInFlight: 4}.Encode()
	require.NoError(t, err)
	token := signWith(t, priv)
	return lifecycleCellar{dir: dir, call: func(method, path string, body any) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&b).Encode(body))
		}
		r := httptest.NewRequest(method, path, &b)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set(GrantHeader, grant)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}}
}

func (c lifecycleCellar) exec(t *testing.T, id, statement string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(c.dir, id+".db"))
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(statement)
	require.NoError(t, err)
}

func (c lifecycleCellar) count(t *testing.T, id string) int {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(c.dir, id+".db")+"?mode=ro")
	require.NoError(t, err)
	defer db.Close()
	var n int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM note").Scan(&n))
	return n
}

func codeOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e arrowstream.Error
	require.NoError(t, json.NewDecoder(w.Body).Decode(&e), w.Body.String())
	return e.Code
}

func TestCreateForkDownloadDelete(t *testing.T) {
	c := newLifecycleCellar(t)
	id, fork := uuid.NewString(), uuid.NewString()

	w := c.call("PUT", "/datasources/"+id, Create{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	c.exec(t, id, "CREATE TABLE note (body TEXT); INSERT INTO note VALUES ('a'), ('b')")

	w = c.call("PUT", "/datasources/"+id, Create{})
	require.Equal(t, http.StatusConflict, w.Code, "an id that exists is never overwritten")

	w = c.call("PUT", "/datasources/"+fork, Create{From: id})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var stored Stored
	require.NoError(t, json.NewDecoder(w.Body).Decode(&stored))
	require.Equal(t, fork, stored.ID)
	require.Positive(t, stored.SizeBytes)
	c.exec(t, fork, "INSERT INTO note VALUES ('c')")
	require.Equal(t, 2, c.count(t, id), "a fork never touches its source")
	require.Equal(t, 3, c.count(t, fork))

	w = c.call("GET", "/datasources/"+id+"/download", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	downloaded := filepath.Join(t.TempDir(), "copy.db")
	require.NoError(t, os.WriteFile(downloaded, w.Body.Bytes(), 0o600))
	db, err := sql.Open("sqlite", downloaded)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM note").Scan(&n))
	require.NoError(t, db.Close())
	require.Equal(t, 2, n)

	w = c.call("GET", "/datasources", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var inventory []Stored
	require.NoError(t, json.NewDecoder(w.Body).Decode(&inventory))
	require.ElementsMatch(t, []string{id, fork}, []string{inventory[0].ID, inventory[1].ID}, "no temporary copy is listed")

	w = c.call("DELETE", "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, w.Code)
	entries, err := os.ReadDir(c.dir)
	require.NoError(t, err)
	for _, e := range entries {
		require.False(t, strings.HasPrefix(e.Name(), id), "%s left behind", e.Name())
		require.False(t, strings.HasPrefix(e.Name(), ".tmp-"), "%s left behind", e.Name())
	}
	w = c.call("DELETE", "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, w.Code, "deleting twice is not an error")
}

func TestLifecycleRefusals(t *testing.T) {
	c := newLifecycleCellar(t)
	id := uuid.NewString()

	w := c.call("PUT", "/datasources/"+id, Create{From: uuid.NewString()})
	require.Equal(t, http.StatusNotFound, w.Code)
	require.NoFileExists(t, filepath.Join(c.dir, id+".db"), "a failed fork leaves no file")

	w = c.call("PUT", "/datasources/"+id, Create{From: id, At: "2026-09-28T00:00:00Z"})
	require.Equal(t, http.StatusNotImplemented, w.Code)
	require.Equal(t, CodeDisabled, codeOf(t, w))

	w = c.call("GET", "/datasources/"+id+"/download", nil)
	require.Equal(t, http.StatusNotFound, w.Code)

	w = c.call("PUT", "/datasources/..%2Fescape", Create{})
	require.Equal(t, http.StatusInternalServerError, w.Code)
	body, _ := io.ReadAll(w.Body)
	require.NotContains(t, string(body), c.dir, "an error never names a path")
}
