package auth_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
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

// grantRole gives the member a role through the real sync endpoint, and returns
// the commit's object id so a later test can take it back.
func grantRole(t *testing.T, f e2e.Fixture, memberID, roleID string) string {
	t.Helper()
	utrID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "user_to_role", utrID, map[string]any{
		"id":           utrID,
		"user_id":      memberID,
		"role_id":      roleID,
		"workspace_id": f.Actor.WorkspaceID,
	})
	return utrID
}

// seedKeyManagerRole returns a role carrying one workspace permission, so a test
// can ask whether a person may act rather than what their token says.
func seedKeyManagerRole(t *testing.T, f e2e.Fixture) string {
	t.Helper()
	return e2e.SeedRoleWithPermission(t, f.Conn, f.Actor.WorkspaceID, "Key Manager", "workspace/api-keys.manage")
}

// mayManageKeys reports whether this token reaches a route gated on the
// workspace/api-keys.manage permission.
func mayManageKeys(t *testing.T, f e2e.Fixture, token, roleID, name string) bool {
	t.Helper()
	return e2e.CreateAPIKey(t, f.H, token, f.Actor.WorkspaceID, roleID, name).Code == http.StatusOK
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

// Granting somebody a role reaches them on the request after the commit, on the
// token they are already holding, because standing is derived per request. And
// it costs them nothing: their session is not what changed.
func TestRoleGrant_ReachesMemberAtOnceWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	memberID, member := newMemberDevice(t, f, "laptop")
	roleID := seedKeyManagerRole(t, f)
	held := e2e.MintJWT(t, memberID)
	require.False(t, mayManageKeys(t, f, held, roleID, "before"),
		"the member starts without the role")

	grantRole(t, f, memberID, roleID)

	require.True(t, mayManageKeys(t, f, held, roleID, "after"),
		"the granted role did not reach the token the member is holding")
	_, err := member.Refresh(t)
	require.NoError(t, err, "granting a role signed the member out")
}

// And taking one away stops granting on the same request, rather than when the
// token that named it happens to expire.
func TestRoleRemoval_ReachesMemberAtOnceWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	memberID, member := newMemberDevice(t, f, "laptop")
	roleID := seedKeyManagerRole(t, f)
	grantID := grantRole(t, f, memberID, roleID)
	held := e2e.MintJWT(t, memberID)
	require.True(t, mayManageKeys(t, f, held, roleID, "before"),
		"the member starts with the role")

	e2e.SyncCommit(t, f.H, f.Actor, "delete", "user_to_role", grantID, map[string]any{
		"id":           grantID,
		"workspace_id": f.Actor.WorkspaceID,
	})

	require.False(t, mayManageKeys(t, f, held, roleID, "after"),
		"the removed role is still granted to the token the member is holding")
	_, err := member.Refresh(t)
	require.NoError(t, err, "removing a role signed the member out")
}

// A group's role set changes every current member's standing at once, so this
// is the widest a single commit reaches. All of them gain it, none of them lose
// their session for it.
func TestGroupRoleGrant_ReachesEveryMemberAtOnceWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	roleID := seedKeyManagerRole(t, f)

	type member struct {
		device *e2e.Device
		id     string
		held   string
	}
	var members []member
	for _, deviceID := range []string{"laptop", "desktop"} {
		id, device := newMemberDevice(t, f, deviceID)
		// Held from before the commit: the point is that it reaches the token
		// they already have, which one minted afterwards would not prove.
		members = append(members, member{device: device, id: id, held: e2e.MintJWT(t, id)})
	}

	groupID := uuid.NewString()
	_, err := f.Conn.Exec(
		`INSERT INTO app."group" (id, workspace_id, name) VALUES ($1::uuid,$2::uuid,$3)`,
		groupID, f.Actor.WorkspaceID, "Engineering")
	require.NoError(t, err)
	for _, m := range members {
		utgID := uuid.NewString()
		e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "user_to_group", utgID, map[string]any{
			"id":           utgID,
			"user_id":      m.id,
			"group_id":     groupID,
			"workspace_id": f.Actor.WorkspaceID,
		})
	}

	gtrID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "group_to_role", gtrID, map[string]any{
		"id":           gtrID,
		"group_id":     groupID,
		"role_id":      roleID,
		"workspace_id": f.Actor.WorkspaceID,
	})

	for _, m := range members {
		require.Truef(t, mayManageKeys(t, f, m.held, roleID, "after-"+m.device.DeviceID),
			"the group's role did not reach the token the %s member is holding", m.device.DeviceID)
		_, err := m.device.Refresh(t)
		require.NoErrorf(t, err, "giving the group a role signed the %s member out", m.device.DeviceID)
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

// The token the author holds predates the workspace they just made, and it
// still reaches it as owner on the next request, with nothing re-minted and
// nobody sent back to the login screen.
func TestCreateWorkspace_MakesTheAuthorItsOwnerWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	device := e2e.SignIn(t, f.Actor.UserID, "laptop")
	held := f.Actor.Token

	rec := e2e.Do(t, f.H, http.MethodPost, "/workspaces", held, map[string]any{"name": "Second"})
	require.Equalf(t, http.StatusOK, rec.Code, "create workspace: %s", rec.Body.String())

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	// A route gated on IsOwner or CanManage: with no roles in the new workspace
	// yet, only ownership passes it.
	rec = e2e.Do(t, f.H, http.MethodPut, "/datasources/"+uuid.NewString(), held, map[string]any{
		"workspace_id": created.ID,
		"db_type":      "postgresql",
		"name":         "warehouse",
		"dsn":          e2e.TargetDSN(t, f.Conn),
	})
	require.Equalf(t, http.StatusNoContent, rec.Code,
		"the author cannot use the workspace they just made: %s", rec.Body.String())

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

// Taking somebody out of a group is the everyday way a manager withdraws access
// granted through a team, and it is the one grant path whose removal the rest
// of these do not exercise.
func TestGroupMembershipRemoved_ReachesMemberAtOnceWithoutSigningThemOut(t *testing.T) {
	f := e2e.Setup(t)

	memberID, member := newMemberDevice(t, f, "laptop")
	roleID := seedKeyManagerRole(t, f)

	groupID := uuid.NewString()
	_, err := f.Conn.Exec(
		`INSERT INTO app."group" (id, workspace_id, name) VALUES ($1::uuid,$2::uuid,$3)`,
		groupID, f.Actor.WorkspaceID, "Engineering")
	require.NoError(t, err)

	utgID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "user_to_group", utgID, map[string]any{
		"id": utgID, "user_id": memberID, "group_id": groupID, "workspace_id": f.Actor.WorkspaceID,
	})
	gtrID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "INSERT", "group_to_role", gtrID, map[string]any{
		"id": gtrID, "group_id": groupID, "role_id": roleID, "workspace_id": f.Actor.WorkspaceID,
	})

	held := e2e.MintJWT(t, memberID)
	require.True(t, mayManageKeys(t, f, held, roleID, "before"),
		"the group grants this before the manager touches it")

	e2e.SyncCommit(t, f.H, f.Actor, "delete", "user_to_group", utgID, map[string]any{
		"id": utgID, "workspace_id": f.Actor.WorkspaceID,
	})

	require.False(t, mayManageKeys(t, f, held, roleID, "after"),
		"the group's role still reaches somebody the manager took out of it")
	_, err = member.Refresh(t)
	require.NoError(t, err, "leaving a group signed the member out")
}
