package datasource_test

import (
	"net/http"
	"strings"
	"testing"

	"backend/e2e"
	"backend/internal/datasource/cellarclient"

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

// The download is named after the database, cut to characters every file
// system accepts.
func TestDownloadManagedFileName(t *testing.T) {
	fixture := newManagedFixture(t)
	for name, want := range map[string]string{
		"notes":               "notes.db",
		`a/b\c:d*e?f"g<h>i|j`: "a_b_c_d_e_f_g_h_i_j.db",
		"tab\there":           "tab_here.db",
		"  padded  ":          "padded.db",
		"   ":                 "database.db",
	} {
		id := createAsOwner(t, fixture, "/datasources", map[string]any{"db_type": "sqlite", "name": name})
		rec := e2e.Do(t, fixture.H, http.MethodGet, "/datasources/"+id+"/download", fixture.Actor.Token,
			map[string]any{"workspace_id": fixture.Actor.WorkspaceID})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equalf(t, `attachment; filename="`+want+`"`, rec.Header().Get("Content-Disposition"), "name %q", name)
	}
}
