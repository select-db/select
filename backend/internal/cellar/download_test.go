package cellar

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDownload(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)

	rec := cellar.call("GET", "/datasources/"+id+"/download", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/vnd.sqlite3", rec.Header().Get("Content-Type"))
	downloadPath := filepath.Join(t.TempDir(), "copy.db")
	require.NoError(t, os.WriteFile(downloadPath, rec.Body.Bytes(), 0o600))
	downloaded, err := sql.Open("sqlite", downloadPath)
	require.NoError(t, err)
	defer downloaded.Close()
	var count int
	require.NoError(t, downloaded.QueryRow("SELECT count(*) FROM note").Scan(&count))
	require.Equal(t, 2, count, "the copy holds the database's rows")

	entries, err := os.ReadDir(cellar.dir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotRegexp(t, `^\.tmp-`, entry.Name(), "the copy is removed once sent")
	}
}

func TestDownloadMissing(t *testing.T) {
	cellar := newTestCellar(t)

	rec := cellar.call("GET", "/datasources/"+uuid.NewString()+"/download", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
