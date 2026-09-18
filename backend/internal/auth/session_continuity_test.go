package auth_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
)

// What a session has to survive. Each test drives the real refresh the
// middleware runs, so "still signed in" means the refresh actually succeeds,
// not that a row happens to be present.

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

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	member := e2e.SignIn(t, memberID, "laptop")

	before, err := member.Refresh(t)
	require.NoError(t, err)
	require.NotContains(t, e2e.RolesInWorkspace(t, before.AccessToken, f.Actor.WorkspaceID), f.Actor.RoleID,
		"the member starts without the role")

	utrID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "user_to_role", utrID, map[string]any{
		"id":           utrID,
		"user_id":      memberID,
		"role_id":      f.Actor.RoleID,
		"workspace_id": f.Actor.WorkspaceID,
	})

	after, err := member.Refresh(t)
	require.NoError(t, err, "granting a role signed the member out")
	require.Contains(t, e2e.RolesInWorkspace(t, after.AccessToken, f.Actor.WorkspaceID), f.Actor.RoleID,
		"the granted role did not reach the member's next token")
}

// And taking one away reaches them the same way.
func TestRoleRemoval_ReachesMemberWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	utrID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "user_to_role", utrID, map[string]any{
		"id":           utrID,
		"user_id":      memberID,
		"role_id":      f.Actor.RoleID,
		"workspace_id": f.Actor.WorkspaceID,
	})
	member := e2e.SignIn(t, memberID, "laptop")

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

// A permission change alters what a role may do rather than who holds it, so it
// touches no token at all. It must not touch anyone's session either.
func TestPermissionChange_DoesNotSignMembersOut(t *testing.T) {
	f := e2e.Setup(t)

	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	member := e2e.SignIn(t, memberID, "laptop")

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
