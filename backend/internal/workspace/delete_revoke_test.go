package workspace_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
)

// Deleting a workspace has to take its members' access with it. Every access
// token already minted -- the owner's and every other member's -- carries on
// being accepted unless membership itself stops resolving for a deleted
// workspace, which is what does the work here. Nobody is signed out for it:
// see TestDeleteWorkspace_DoesNotSignTheOwnerOut.
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

// A workspace that is gone has no use for credentials into somebody's
// database, and the encrypted DSN and SSH config sit on the datasource row
// until something clears them.
func TestDeleteWorkspace_ClearsStoredCredentials(t *testing.T) {
	f := e2e.Setup(t)

	id := uuid.NewString()
	rec := e2e.Do(t, f.H, http.MethodPut, "/datasources/"+id, f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"db_type":      "postgresql",
		"name":         "warehouse",
		"dsn":          e2e.TargetDSN(t, f.Conn),
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "upsert datasource: %s", rec.Body.String())

	var stored int
	require.NoError(t, f.Conn.QueryRow(
		`SELECT count(*) FROM app.datasource WHERE id = $1::uuid AND encrypted_dsn IS NOT NULL`, id,
	).Scan(&stored))
	require.Equal(t, 1, stored, "the datasource was stored without its encrypted DSN")

	rec = e2e.Do(t, f.H, http.MethodDelete, "/workspaces/"+f.Actor.WorkspaceID, f.Actor.Token,
		map[string]any{"workspace_id": f.Actor.WorkspaceID})
	require.Equalf(t, http.StatusNoContent, rec.Code, "delete: %s", rec.Body.String())

	var remaining int
	require.NoError(t, f.Conn.QueryRow(
		`SELECT count(*) FROM app.datasource
		 WHERE workspace_id = $1::uuid AND (encrypted_dsn IS NOT NULL OR encrypted_ssh IS NOT NULL)`,
		f.Actor.WorkspaceID,
	).Scan(&remaining))
	require.Zerof(t, remaining, "%d datasource rows still hold credentials into a deleted workspace", remaining)

	var rows int
	require.NoError(t, f.Conn.QueryRow(
		`SELECT count(*) FROM app.datasource WHERE workspace_id = $1::uuid`, f.Actor.WorkspaceID,
	).Scan(&rows))
	require.Equal(t, 1, rows, "the datasource row itself should survive, only its secrets go")
}

// Removing somebody from a workspace is a deleted_at on the membership row, and
// membership was read without looking at it: the person kept the access their
// token was minted with.
//
// Removed the way the app removes them, through a sync commit, so this covers
// the path a person actually takes rather than the row it ends at.
func TestRemovedMember_LosesAccess(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	membershipID := e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	memberToken := e2e.MintJWT(t, memberID)

	rec := e2e.Do(t, f.H, http.MethodGet, "/datasources", memberToken, nil)
	require.Equalf(t, http.StatusOK, rec.Code, "before the removal: %s", rec.Body.String())

	e2e.SyncCommit(t, f.H, f.Actor, "delete", "workspace_to_user", membershipID, map[string]any{
		"id":           membershipID,
		"workspace_id": f.Actor.WorkspaceID,
		"user_id":      memberID,
	})

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

	rec := e2e.CreateAPIKey(t, f.H, f.Actor.Token, f.Actor.WorkspaceID, f.Actor.RoleID, "ci")
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

// Permissions are read from the database on each request, behind a cache the
// sync invalidates, so editing what a role may do reaches the people holding it
// at once, without waiting for any token. Taking the role itself away is the
// other half, in TestRoleRemoval_ReachesMemberAtOnceWithoutSigningThemOut.
func TestPermissionRemoved_TakesEffectOnTheTokenAlreadyHeld(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	roleID := e2e.SeedRoleWithPermission(t, f.Conn, f.Actor.WorkspaceID, "Key Manager", "workspace/api-keys.manage")
	e2e.SeedUserRole(t, f.Conn, memberID, roleID, f.Actor.WorkspaceID)

	held := e2e.MintJWT(t, memberID)
	require.Equal(t, http.StatusOK,
		e2e.CreateAPIKey(t, f.H, held, f.Actor.WorkspaceID, roleID, "before").Code,
		"the role grants this before the manager touches it")

	var permID string
	require.NoError(t, f.Conn.QueryRow(
		`SELECT id FROM app.permission WHERE role_id = $1::uuid`, roleID).Scan(&permID))
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "permission", permID, map[string]any{
		"id": permID, "workspace_id": f.Actor.WorkspaceID,
	})

	require.NotEqual(t, http.StatusOK,
		e2e.CreateAPIKey(t, f.H, held, f.Actor.WorkspaceID, roleID, "after").Code,
		"a permission the manager removed is still granted to a token already issued")
}

// Nothing validates that a user_to_role names somebody who belongs to the
// workspace, so a manager can write a grant for any user id at all. Membership
// is what keeps it from being enforced, and it is the only thing that does.
func TestGrantWithoutMembership_ReachesNothing(t *testing.T) {
	f := e2e.Setup(t)

	outsiderID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, outsiderID)
	roleID := e2e.SeedRoleWithPermission(t, f.Conn, f.Actor.WorkspaceID, "Key Manager", "workspace/api-keys.manage")

	// The grant, with no workspace_to_user row to go with it.
	e2e.SeedUserRole(t, f.Conn, outsiderID, roleID, f.Actor.WorkspaceID)

	rec := e2e.CreateAPIKey(t, f.H, e2e.MintJWT(t, outsiderID), f.Actor.WorkspaceID, roleID, "outsider")
	require.NotEqualf(t, http.StatusOK, rec.Code,
		"a role granted to a non-member let them act in the workspace: %s", rec.Body.String())
}
