package workspace_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
)

// Deleting a workspace has to take its members' access with it. The owner's
// refresh tokens are dropped by the handler, but every access token already
// minted -- the owner's and every other member's -- carries on being accepted
// unless membership itself stops resolving for a deleted workspace.
func TestDeleteWorkspace_RevokesMemberAccess(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	memberToken := e2e.MintJWT(t, memberID)

	rec := e2e.Do(t, f.H, http.MethodGet, "/datasources", memberToken, nil)
	require.Equalf(t, http.StatusOK, rec.Code, "before the delete: %s", rec.Body.String())

	rec = e2e.Do(t, f.H, http.MethodDelete, "/workspaces/"+f.Actor.WorkspaceID, f.Actor.Token,
		map[string]any{"workspace_id": f.Actor.WorkspaceID})
	require.Equalf(t, http.StatusNoContent, rec.Code, "delete: %s", rec.Body.String())

	rec = e2e.Do(t, f.H, http.MethodGet, "/datasources", memberToken, nil)
	require.NotEqualf(t, http.StatusOK, rec.Code,
		"a member still reaches the datasources of a deleted workspace: %s", rec.Body.String())

	rec = e2e.Do(t, f.H, http.MethodGet, "/datasources", f.Actor.Token, nil)
	require.NotEqualf(t, http.StatusOK, rec.Code,
		"the owner still reaches the datasources of the workspace they deleted: %s", rec.Body.String())
}

// Removing somebody from a workspace is a deleted_at on the membership row, and
// membership was read without looking at it: the person kept the access their
// token was minted with.
func TestRemovedMember_LosesAccess(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	memberToken := e2e.MintJWT(t, memberID)

	rec := e2e.Do(t, f.H, http.MethodGet, "/datasources", memberToken, nil)
	require.Equalf(t, http.StatusOK, rec.Code, "before the removal: %s", rec.Body.String())

	_, err := f.Conn.Exec(
		`UPDATE app.workspace_to_user SET deleted_at = NOW() WHERE user_id = $1::uuid AND workspace_id = $2::uuid`,
		memberID, f.Actor.WorkspaceID,
	)
	require.NoError(t, err)

	rec = e2e.Do(t, f.H, http.MethodGet, "/datasources", memberToken, nil)
	require.NotEqualf(t, http.StatusOK, rec.Code,
		"a removed member still reaches the workspace: %s", rec.Body.String())
}

// An API key carries its workspace on the key row, so the membership query the
// tests above cover is never consulted for it: the workspace's standing has to
// be checked where the key is resolved, or a key outlives the workspace it was
// issued in and keeps reaching every route in it.
func TestDeleteWorkspace_RevokesAPIKeys(t *testing.T) {
	f := e2e.Setup(t)

	rec := e2e.Do(t, f.H, http.MethodPost, "/apikeys", f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"name":         "ci",
		"role_ids":     []string{f.Actor.RoleID},
	})
	require.Equalf(t, http.StatusOK, rec.Code, "create key: %s", rec.Body.String())

	var created struct {
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Key)

	rec = e2e.Do(t, f.H, http.MethodGet, "/datasources", created.Key, nil)
	require.Equalf(t, http.StatusOK, rec.Code, "before the delete: %s", rec.Body.String())

	rec = e2e.Do(t, f.H, http.MethodDelete, "/workspaces/"+f.Actor.WorkspaceID, f.Actor.Token,
		map[string]any{"workspace_id": f.Actor.WorkspaceID})
	require.Equalf(t, http.StatusNoContent, rec.Code, "delete: %s", rec.Body.String())

	rec = e2e.Do(t, f.H, http.MethodGet, "/datasources", created.Key, nil)
	require.NotEqualf(t, http.StatusOK, rec.Code,
		"an API key still reaches the datasources of a deleted workspace: %s", rec.Body.String())
}
