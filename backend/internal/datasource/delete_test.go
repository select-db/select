package datasource_test

import (
	"net/http"
	"testing"

	"backend/e2e"
	"backend/internal/datasource/cellarclient"

	"github.com/stretchr/testify/require"
)

func TestDeleteManaged(t *testing.T) {
	fixture := newManagedFixture(t)
	id := createNotes(t, fixture)
	forkID := createAsOwner(t, fixture, "/datasources/"+id+"/fork", nil)

	status, responseBody := callAsOwner(t, fixture, http.MethodDelete, "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, status, string(responseBody))

	var state string
	require.NoError(t, fixture.Conn.QueryRow(`SELECT state FROM app.datasource WHERE id = $1`, id).Scan(&state))
	require.Equal(t, "deleting", state, "the reconciler purges the file; the row only stops serving")
	status, _ = callAsOwner(t, fixture, http.MethodPost, "/datasources/"+id+"/execute", map[string]any{"sql": "SELECT 1"})
	require.Equal(t, http.StatusNotFound, status, "a deleted database stops serving at once")

	var liveRoles int
	require.NoError(t, fixture.Conn.QueryRow(`SELECT count(*) FROM app.role r WHERE r.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM app.permission p WHERE p.role_id = r.id AND p.datasource_id = $1)`, id).Scan(&liveRoles))
	require.Zero(t, liveRoles, "the database's dedicated role goes with it")

	_, failure := e2e.Execute(t, fixture, forkID, "SELECT body FROM note")
	require.Nil(t, failure, "deleting the source leaves its fork")
}

func TestDeleteManagedWithoutCellar(t *testing.T) {
	fixture := newManagedFixture(t)
	id := createNotes(t, fixture)
	cellarclient.URL = ""

	requireDisabled(t, fixture, http.MethodDelete, "/datasources/"+id)
}
