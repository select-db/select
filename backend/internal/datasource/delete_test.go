package datasource_test

import (
	"net/http"
	"testing"

	"backend/e2e"
	"backend/internal/datasource/cellarclient"

	"github.com/google/uuid"
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

// A datasource that is not managed goes the same way: its rules leave every
// role, and a role that had rules on it alone goes with them.
func TestDeleteUnmanagedRemovesItsRules(t *testing.T) {
	fixture := e2e.Setup(t)
	id := uuid.NewString()
	status, responseBody := callAsOwner(t, fixture, http.MethodPut, "/datasources/"+id,
		map[string]any{"db_type": "postgresql", "name": "remote", "dsn": e2e.TargetDSN(t, fixture.Conn)})
	require.Equal(t, http.StatusNoContent, status, string(responseBody))

	onlyThisRole := e2e.SeedRoleWithPermission(t, fixture.Conn, fixture.Actor.WorkspaceID, "only this", "see")
	_, err := fixture.Conn.Exec(`UPDATE app.permission SET datasource_id = $1 WHERE role_id = $2`, id, onlyThisRole)
	require.NoError(t, err)
	sharedRole := e2e.SeedRoleWithPermission(t, fixture.Conn, fixture.Actor.WorkspaceID, "shared", "see")
	_, err = fixture.Conn.Exec(`INSERT INTO app.permission (role_id, workspace_id, datasource_id, action, effect)
		VALUES ($1::uuid, $2::uuid, $3, 'select', 'allow')`, sharedRole, fixture.Actor.WorkspaceID, id)
	require.NoError(t, err)

	status, responseBody = callAsOwner(t, fixture, http.MethodDelete, "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, status, string(responseBody))

	var rows int
	require.NoError(t, fixture.Conn.QueryRow(`SELECT count(*) FROM app.datasource WHERE id = $1`, id).Scan(&rows))
	require.Zero(t, rows, "a datasource with no file to purge is removed at once")
	var liveRules int
	require.NoError(t, fixture.Conn.QueryRow(`SELECT count(*) FROM app.permission WHERE datasource_id = $1 AND deleted_at IS NULL`, id).Scan(&liveRules))
	require.Zero(t, liveRules, "no rule outlives its datasource")
	var onlyThisDeleted, sharedDeleted bool
	require.NoError(t, fixture.Conn.QueryRow(`SELECT deleted_at IS NOT NULL FROM app.role WHERE id = $1`, onlyThisRole).Scan(&onlyThisDeleted))
	require.NoError(t, fixture.Conn.QueryRow(`SELECT deleted_at IS NOT NULL FROM app.role WHERE id = $1`, sharedRole).Scan(&sharedDeleted))
	require.True(t, onlyThisDeleted, "a role with rules on this datasource alone goes with it")
	require.False(t, sharedDeleted, "a role with a rule anywhere else stays")
}
