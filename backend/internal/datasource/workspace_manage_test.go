package datasource_test

import (
	"net/http"
	"testing"

	"backend/e2e"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// workspace/datasources.manage administers every datasource of the workspace
// without a rule on any of them, except in the list, which stays per datasource.
func TestWorkspaceManageAdministersEveryDatasource(t *testing.T) {
	fixture := newManagedFixture(t)
	workspaceID := fixture.Actor.WorkspaceID
	manager := uuid.NewString()
	e2e.SeedUser(t, fixture.Conn, manager)
	e2e.SeedMembership(t, fixture.Conn, workspaceID, manager)
	managerRole := e2e.SeedRoleWithPermission(t, fixture.Conn, workspaceID, "connections", "workspace/datasources.manage")
	e2e.SeedUserRole(t, fixture.Conn, manager, managerRole, workspaceID)
	token := e2e.MintJWT(t, manager)
	notesID := createNotes(t, fixture)
	path := "/datasources/" + notesID
	body := map[string]any{"workspace_id": workspaceID}

	rec := e2e.Do(t, fixture.H, http.MethodGet, path, token, nil)
	require.Equal(t, http.StatusOK, rec.Code, "get: %s", rec.Body.String())
	rec = e2e.Do(t, fixture.H, http.MethodGet, path+"/download", token, nil)
	require.Equal(t, http.StatusOK, rec.Code, "download: %s", rec.Body.String())
	rec = e2e.Do(t, fixture.H, http.MethodPost, path+"/fork", token, body)
	require.Equal(t, http.StatusCreated, rec.Code, "fork: %s", rec.Body.String())
	rec = e2e.Do(t, fixture.H, http.MethodGet, "/datasources", token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, "[]", rec.Body.String(), "the list is per datasource")
	rec = e2e.Do(t, fixture.H, http.MethodDelete, path, token, body)
	require.Equal(t, http.StatusNoContent, rec.Code, "delete: %s", rec.Body.String())
}
