package datasource_test

import (
	"net/http"
	"strings"
	"testing"

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
