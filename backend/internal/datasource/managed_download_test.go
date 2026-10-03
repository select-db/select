package datasource_test

import (
	"net/http"
	"strings"
	"testing"

	"backend/e2e"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/stretchr/testify/require"
)

func TestDownloadManaged(t *testing.T) {
	fixture := newManagedFixture(t)
	id := createNotes(t, fixture)

	status, responseBody := callAsOwner(t, fixture, http.MethodGet, "/datasources/"+id+"/download", nil)
	require.Equal(t, http.StatusOK, status, string(responseBody))
	require.True(t, strings.HasPrefix(string(responseBody), "SQLite format 3"))
}

func TestDownloadManagedWithoutCellar(t *testing.T) {
	fixture := newManagedFixture(t)
	id := createNotes(t, fixture)
	cellarclient.URL = ""

	requireDisabled(t, fixture, http.MethodGet, "/datasources/"+id+"/download")
}

// The download is named after the database, in characters every file system
// accepts.
func TestDownloadManagedFileName(t *testing.T) {
	fixture := newManagedFixture(t)
	id := createAsOwner(t, fixture, "/datasources", map[string]any{"db_type": "sqlite", "name": "q3/q4: sales"})

	rec := e2e.Do(t, fixture.H, http.MethodGet, "/datasources/"+id+"/download", fixture.Actor.Token,
		map[string]any{"workspace_id": fixture.Actor.WorkspaceID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, `attachment; filename="q3_q4_ sales.db"`, rec.Header().Get("Content-Disposition"))
}
