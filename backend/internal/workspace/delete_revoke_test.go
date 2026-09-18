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

// What a manager taking access away actually reaches, and when. Three things
// can be taken, and they do not take effect at the same moment, so each is
// pinned here against the token the person is already holding.

// seedManagedRole gives the member a role carrying one workspace permission,
// and returns the role id and the user_to_role row id.
func seedManagedRole(t *testing.T, f e2e.Fixture, memberID, action string) (roleID, grantID string) {
	t.Helper()
	roleID = uuid.NewString()
	e2e.SeedRole(t, f.Conn, roleID, f.Actor.WorkspaceID, "Key Manager")
	_, err := f.Conn.Exec(
		`INSERT INTO app.permission (id, role_id, workspace_id, action, effect)
		 VALUES ($1::uuid,$2::uuid,$3::uuid,$4,'allow')`,
		uuid.NewString(), roleID, f.Actor.WorkspaceID, action)
	require.NoError(t, err)
	e2e.SeedUserRole(t, f.Conn, memberID, roleID, f.Actor.WorkspaceID)
	require.NoError(t, f.Conn.QueryRow(
		`SELECT id FROM app.user_to_role WHERE user_id=$1::uuid AND role_id=$2::uuid`,
		memberID, roleID).Scan(&grantID))
	return roleID, grantID
}

// createsAPIKey reports whether the token may mint an API key, a route gated on
// the workspace/api-keys.manage permission.
func createsAPIKey(t *testing.T, f e2e.Fixture, token, roleID, name string) int {
	t.Helper()
	return e2e.Do(t, f.H, http.MethodPost, "/apikeys", token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"name":         name,
		"role_ids":     []string{roleID},
	}).Code
}

// Permissions are read from the database on each request, behind a cache the
// sync invalidates, so editing what a role may do reaches the people holding it
// at once, without waiting for any token.
func TestPermissionRemoved_TakesEffectOnTheTokenAlreadyHeld(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	roleID, _ := seedManagedRole(t, f, memberID, "workspace/api-keys.manage")

	held := e2e.MintJWT(t, memberID)
	require.Equal(t, http.StatusOK, createsAPIKey(t, f, held, roleID, "before"),
		"the role grants this before the manager touches it")

	var permID string
	require.NoError(t, f.Conn.QueryRow(
		`SELECT id FROM app.permission WHERE role_id = $1::uuid`, roleID).Scan(&permID))
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "permission", permID, map[string]any{
		"id": permID, "workspace_id": f.Actor.WorkspaceID,
	})

	require.NotEqual(t, http.StatusOK, createsAPIKey(t, f, held, roleID, "after"),
		"a permission the manager removed is still granted to a token already issued")
}

// Which roles a person holds is baked into their access token, so taking a role
// away reaches them when that token turns over and not before. The pair below
// is the contract: enforced on the next token, and not on the one in hand.
//
// The window is accessTokenTTL, five minutes. It is not what revoking their
// refresh token would have shortened: an unexpired access token is accepted on
// its signature without the refresh token being read at all.
func TestRoleRemoved_IsEnforcedOnTheNextTokenAndNotBefore(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	roleID, grantID := seedManagedRole(t, f, memberID, "workspace/api-keys.manage")

	held := e2e.MintJWT(t, memberID)
	require.Equal(t, http.StatusOK, createsAPIKey(t, f, held, roleID, "before"),
		"the role grants this before the manager takes it away")

	e2e.SyncCommit(t, f.H, f.Actor, "delete", "user_to_role", grantID, map[string]any{
		"id": grantID, "workspace_id": f.Actor.WorkspaceID,
	})

	require.Equal(t, http.StatusOK, createsAPIKey(t, f, held, roleID, "during-the-window"),
		"the window closed early, which is a better guarantee than this pins: widen it deliberately")

	require.NotEqual(t, http.StatusOK, createsAPIKey(t, f, e2e.MintJWT(t, memberID), roleID, "after"),
		"the removed role is still granted by a token minted after the removal")
}
