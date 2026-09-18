package auth_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
	"backend/internal/auth"
)

// What a session has to survive. Each test drives the real refresh the
// middleware runs, so "still signed in" means the refresh actually succeeds,
// not that a row happens to be present.

// newMemberDevice adds a member to the fixture's workspace and signs them in on
// one device.
func newMemberDevice(t *testing.T, f e2e.Fixture, deviceID string) (string, *e2e.Device) {
	t.Helper()
	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	return memberID, e2e.SignIn(t, memberID, deviceID)
}

// grantRole gives the member the fixture's role through the real sync endpoint,
// and returns the commit's object id so a later test can take it back.
func grantRole(t *testing.T, f e2e.Fixture, memberID string) string {
	t.Helper()
	utrID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "user_to_role", utrID, map[string]any{
		"id":           utrID,
		"user_id":      memberID,
		"role_id":      f.Actor.RoleID,
		"workspace_id": f.Actor.WorkspaceID,
	})
	return utrID
}

// A person signed in on a laptop and a desktop holds one refresh token per
// device, and neither device's rotation may take the other's away.
func TestTwoDevices_BothStaySignedIn(t *testing.T) {
	f := e2e.Setup(t)

	laptop := e2e.SignIn(t, f.Actor.UserID, "laptop")
	desktop := e2e.SignIn(t, f.Actor.UserID, "desktop")

	// Signing in on the desktop must not have cost the laptop its session.
	_, err := laptop.Refresh(t)
	require.NoError(t, err, "the laptop lost its session when the desktop signed in")

	// Nor may the laptop's rotation, which happens every five minutes, cost the
	// desktop its own.
	_, err = desktop.Refresh(t)
	require.NoError(t, err, "the desktop lost its session when the laptop refreshed")

	// And both keep going, rather than taking turns signing each other out.
	_, err = laptop.Refresh(t)
	require.NoError(t, err, "the laptop lost its session when the desktop refreshed")
	_, err = desktop.Refresh(t)
	require.NoError(t, err, "the desktop lost its session on the next round")
}

// A refresh token belongs to the device it was issued for: presenting it with
// another device id must not work.
func TestRefreshToken_IsBoundToItsDevice(t *testing.T) {
	f := e2e.Setup(t)

	laptop := e2e.SignIn(t, f.Actor.UserID, "laptop")
	impostor := &e2e.Device{UserID: laptop.UserID, DeviceID: "somewhere-else", Token: laptop.Token}

	_, err := impostor.Refresh(t)
	require.Error(t, err, "a refresh token must not work from a different device")
}

// Granting somebody a role has to reach them, and the way it reaches them is
// the next token they are issued. Their session is not the thing that changed.
func TestRoleGrant_ReachesMemberWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	memberID, member := newMemberDevice(t, f, "laptop")

	before, err := member.Refresh(t)
	require.NoError(t, err)
	require.NotContains(t, e2e.RolesInWorkspace(t, before.AccessToken, f.Actor.WorkspaceID), f.Actor.RoleID,
		"the member starts without the role")

	grantRole(t, f, memberID)

	after, err := member.Refresh(t)
	require.NoError(t, err, "granting a role signed the member out")
	require.Contains(t, e2e.RolesInWorkspace(t, after.AccessToken, f.Actor.WorkspaceID), f.Actor.RoleID,
		"the granted role did not reach the member's next token")
}

// And taking one away reaches them the same way.
func TestRoleRemoval_ReachesMemberWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	memberID, member := newMemberDevice(t, f, "laptop")
	utrID := grantRole(t, f, memberID)

	before, err := member.Refresh(t)
	require.NoError(t, err)
	require.Contains(t, e2e.RolesInWorkspace(t, before.AccessToken, f.Actor.WorkspaceID), f.Actor.RoleID,
		"the member starts with the role")

	e2e.SyncCommit(t, f.H, f.Actor, "delete", "user_to_role", utrID, map[string]any{
		"id":           utrID,
		"workspace_id": f.Actor.WorkspaceID,
	})

	after, err := member.Refresh(t)
	require.NoError(t, err, "removing a role signed the member out")
	require.NotContains(t, e2e.RolesInWorkspace(t, after.AccessToken, f.Actor.WorkspaceID), f.Actor.RoleID,
		"the removed role is still in the member's next token")
}

// A group's role set changes every current member's standing at once, so this is
// the widest a single commit reaches. None of them may lose their session for it.
func TestGroupRoleGrant_ReachesEveryMemberWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	firstID, first := newMemberDevice(t, f, "laptop")
	secondID, second := newMemberDevice(t, f, "desktop")

	groupID := uuid.NewString()
	_, err := f.Conn.Exec(
		`INSERT INTO app."group" (id, workspace_id, name) VALUES ($1::uuid,$2::uuid,$3)`,
		groupID, f.Actor.WorkspaceID, "Engineering")
	require.NoError(t, err)
	for _, memberID := range []string{firstID, secondID} {
		utgID := uuid.NewString()
		e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "user_to_group", utgID, map[string]any{
			"id":           utgID,
			"user_id":      memberID,
			"group_id":     groupID,
			"workspace_id": f.Actor.WorkspaceID,
		})
	}

	gtrID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "group_to_role", gtrID, map[string]any{
		"id":           gtrID,
		"group_id":     groupID,
		"role_id":      f.Actor.RoleID,
		"workspace_id": f.Actor.WorkspaceID,
	})

	for name, device := range map[string]*e2e.Device{"first": first, "second": second} {
		tokens, err := device.Refresh(t)
		require.NoErrorf(t, err, "giving the group a role signed the %s member out", name)
		require.Containsf(t, e2e.RolesInWorkspace(t, tokens.AccessToken, f.Actor.WorkspaceID), f.Actor.RoleID,
			"the group's role did not reach the %s member's next token", name)
	}
}

// A permission change alters what a role may do rather than who holds it, so it
// touches no token at all. It must not touch anyone's session either.
func TestPermissionChange_DoesNotSignMembersOut(t *testing.T) {
	f := e2e.Setup(t)

	_, member := newMemberDevice(t, f, "laptop")

	permID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "permission", permID, map[string]any{
		"id":           permID,
		"role_id":      f.Actor.RoleID,
		"workspace_id": f.Actor.WorkspaceID,
		"action":       "datasource:read",
		"effect":       "allow",
	})

	_, err := member.Refresh(t)
	require.NoError(t, err, "a permission change signed a member out")
	_, err = e2e.SignIn(t, f.Actor.UserID, "owner-device").Refresh(t)
	require.NoError(t, err, "a permission change signed its author out")
}

// Making a workspace is not a reason to be sent back to the login screen.
func TestCreateWorkspace_DoesNotSignTheAuthorOut(t *testing.T) {
	f := e2e.Setup(t)

	device := e2e.SignIn(t, f.Actor.UserID, "laptop")

	rec := e2e.Do(t, f.H, http.MethodPost, "/workspaces", f.Actor.Token, map[string]any{"name": "Second"})
	require.Equalf(t, http.StatusOK, rec.Code, "create workspace: %s", rec.Body.String())

	_, err := device.Refresh(t)
	require.NoError(t, err, "creating a workspace signed its author out")
}

// Creating a workspace makes you its owner, and the claim the caller is holding
// predates it, so the handler has to hand back a token that says so. Without it
// the creator is a member of their new workspace with no roles and no ownership
// until their access token expires, and every owner-gated route in it refuses
// them for five minutes.
func TestCreateWorkspace_MakesTheAuthorItsOwnerAtOnce(t *testing.T) {
	f := e2e.Setup(t)

	rec := e2e.Do(t, f.H, http.MethodPost, "/workspaces", f.Actor.Token, map[string]any{"name": "Second"})
	require.Equalf(t, http.StatusOK, rec.Code, "create workspace: %s", rec.Body.String())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	token := rec.Header().Get("X-New-Access-Token")
	require.NotEmpty(t, token, "creating a workspace must hand back a token that knows about it")

	_, claims, err := auth.ValidateJWT(token)
	require.NoError(t, err)
	var owned bool
	for _, ws := range claims.Workspaces {
		if ws.ID == created.ID {
			owned = ws.IsOwner
		}
	}
	require.True(t, owned, "the new token does not make the author owner of the workspace they just made")

	// And it works on a route the new workspace grants no other way: with no
	// roles in it yet, only ownership passes this gate.
	rec = e2e.Do(t, f.H, http.MethodPut, "/datasources/"+uuid.NewString(), token, map[string]any{
		"workspace_id": created.ID,
		"db_type":      "postgresql",
		"name":         "warehouse",
		"dsn":          e2e.TargetDSN(t, f.Conn),
	})
	require.Equalf(t, http.StatusNoContent, rec.Code,
		"the author cannot use their own new workspace: %s", rec.Body.String())
}

// Nor is deleting one. Access to the deleted workspace stops because membership
// stops resolving for it, which TestDeleteWorkspace_RevokesMemberAccess covers;
// the person's session is a separate thing and stays.
func TestDeleteWorkspace_DoesNotSignTheOwnerOut(t *testing.T) {
	f := e2e.Setup(t)

	device := e2e.SignIn(t, f.Actor.UserID, "laptop")

	rec := e2e.Do(t, f.H, http.MethodDelete, "/workspaces/"+f.Actor.WorkspaceID, f.Actor.Token,
		map[string]any{"workspace_id": f.Actor.WorkspaceID})
	require.Equalf(t, http.StatusNoContent, rec.Code, "delete workspace: %s", rec.Body.String())

	_, err := device.Refresh(t)
	require.NoError(t, err, "deleting a workspace signed the owner out")
}
