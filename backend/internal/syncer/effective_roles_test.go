package syncer

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/db"
	"backend/internal/auth"

	"github.com/google/uuid"
)

// grantsIn returns the roles the standing query hands back for this workspace,
// in the order it aggregates them, so a test can see whether a role granted two
// ways arrives once or twice. Empty when the user is not a member: the query
// only emits rows for workspaces they belong to.
func grantsIn(t *testing.T, userID, wsID string) []auth.RoleRef {
	t.Helper()
	rows, err := db.Queries.GetStandingByUserID(context.Background(), uuid.MustParse(userID))
	require.NoError(t, err)
	for _, row := range rows {
		if row.WorkspaceID.String() != wsID {
			continue
		}
		var roles []auth.RoleRef
		require.NoError(t, json.Unmarshal(row.Roles, &roles))
		return roles
	}
	return nil
}

// effectiveRoles is what grantsIn returns keyed by role id. It reads the query
// standing is derived from, not buildAuthContext itself: the e2e tests in
// internal/auth and internal/workspace are what cover the assembly.
func effectiveRoles(t *testing.T, userID, wsID string) map[string]string {
	t.Helper()
	roles := map[string]string{}
	for _, g := range grantsIn(t, userID, wsID) {
		roles[g.ID] = g.Name
	}
	return roles
}

func seedUserToRole(t *testing.T, conn *sql.DB, id, userID, roleID, workspaceID string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO app.user_to_role (id, user_id, role_id, workspace_id) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid)`,
		id, userID, roleID, workspaceID,
	)
	require.NoError(t, err)
}

func seedUserToGroup(t *testing.T, conn *sql.DB, id, userID, groupID, workspaceID string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO app.user_to_group (id, user_id, group_id, workspace_id) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid)`,
		id, userID, groupID, workspaceID,
	)
	require.NoError(t, err)
}

func seedGroupToRole(t *testing.T, conn *sql.DB, id, groupID, roleID, workspaceID string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO app.group_to_role (id, group_id, role_id, workspace_id) VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid)`,
		id, groupID, roleID, workspaceID,
	)
	require.NoError(t, err)
}

// A user's effective roles are the union of the directly-assigned ones
// (user_to_role) and those granted through a group (user_to_group ->
// group_to_role). The union is what a request is given, so both paths have to
// reach it.
func TestStanding_UnionsDirectAndGroupRoles(t *testing.T) {
	conn := newTestDB(t)

	userID := newID()
	wsID := newID()
	directRoleID := newID()
	groupRoleID := newID()
	groupID := newID()

	ownerID := newID()
	seedUser(t, conn, userID, "member")
	seedUser(t, conn, ownerID, "owner")
	seedWorkspace(t, conn, wsID, "ws", ownerID) // owned by someone else
	seedRole(t, conn, directRoleID, wsID, "direct-role")
	seedRole(t, conn, groupRoleID, wsID, "group-role")
	seedGroup(t, conn, groupID, wsID, "engineering")

	// membership so the workspace appears at all
	seedMembership(t, conn, wsID, userID)

	seedUserToRole(t, conn, newID(), userID, directRoleID, wsID)
	seedUserToGroup(t, conn, newID(), userID, groupID, wsID)
	seedGroupToRole(t, conn, newID(), groupID, groupRoleID, wsID)

	got := effectiveRoles(t, userID, wsID)
	require.Contains(t, got, directRoleID, "direct role must be in the standing")
	require.Contains(t, got, groupRoleID, "group-granted role must be in the standing")
	require.Equal(t, "direct-role", got[directRoleID])
	require.Equal(t, "group-role", got[groupRoleID])
}

// A role assigned directly AND through a group is one grant, not two.
func TestStanding_DedupesRoleGrantedBothWays(t *testing.T) {
	conn := newTestDB(t)

	userID := newID()
	wsID := newID()
	roleID := newID()
	groupID := newID()

	ownerID := newID()
	seedUser(t, conn, userID, "member")
	seedUser(t, conn, ownerID, "owner")
	seedWorkspace(t, conn, wsID, "ws", ownerID)
	seedRole(t, conn, roleID, wsID, "shared-role")
	seedGroup(t, conn, groupID, wsID, "engineering")

	seedMembership(t, conn, wsID, userID)

	// same role, both paths
	seedUserToRole(t, conn, newID(), userID, roleID, wsID)
	seedUserToGroup(t, conn, newID(), userID, groupID, wsID)
	seedGroupToRole(t, conn, newID(), groupID, roleID, wsID)

	count := 0
	for _, g := range grantsIn(t, userID, wsID) {
		if g.ID == roleID {
			count++
		}
	}
	require.Equal(t, 1, count, "role granted both directly and via group must be counted once")
}

// Deleting a group is a soft delete, and the FK cascade only fires on hard
// deletes, so its still-live membership rows must not keep granting its roles.
func TestStanding_SoftDeletedGroupGrantsNoRoles(t *testing.T) {
	conn := newTestDB(t)

	userID := newID()
	wsID := newID()
	groupRoleID := newID()
	groupID := newID()

	ownerID := newID()
	seedUser(t, conn, userID, "member")
	seedUser(t, conn, ownerID, "owner")
	seedWorkspace(t, conn, wsID, "ws", ownerID)
	seedRole(t, conn, groupRoleID, wsID, "group-role")
	seedGroup(t, conn, groupID, wsID, "engineering")

	seedMembership(t, conn, wsID, userID)

	seedUserToGroup(t, conn, newID(), userID, groupID, wsID)
	seedGroupToRole(t, conn, newID(), groupID, groupRoleID, wsID)

	// soft-delete the group
	_, err := conn.Exec(`UPDATE app."group" SET deleted_at = now() WHERE id = $1::uuid`, groupID)
	require.NoError(t, err)

	got := effectiveRoles(t, userID, wsID)
	require.NotContains(t, got, groupRoleID, "soft-deleted group must grant no roles")
}

// A grant row names both a role and a workspace, and the two can disagree. The
// syncer refuses to write such a row, so this seeds one directly: standing is
// derived from this query alone, and it must not hand somebody a role from a
// tenant they were never granted anything in.
func TestStanding_IgnoresGrantPointingAtAnotherWorkspacesRole(t *testing.T) {
	conn := newTestDB(t)

	userID, ownerID := newID(), newID()
	mine, theirs := newID(), newID()
	theirRoleID := newID()

	seedUser(t, conn, userID, "member")
	seedUser(t, conn, ownerID, "owner")
	seedWorkspace(t, conn, mine, "mine", userID)
	seedWorkspace(t, conn, theirs, "theirs", ownerID)
	seedRole(t, conn, theirRoleID, theirs, "their-admin")

	// A member of both, so the other workspace's standing is built at all.
	seedMembership(t, conn, mine, userID)
	seedMembership(t, conn, theirs, userID)

	seedUserToRole(t, conn, newID(), userID, theirRoleID, mine)

	require.NotContains(t, effectiveRoles(t, userID, theirs), theirRoleID,
		"a grant written in one workspace granted a role in another")
	require.NotContains(t, effectiveRoles(t, userID, mine), theirRoleID,
		"a grant naming another workspace's role granted it locally")
}

// Nothing validates that a user_to_role names somebody who belongs to the
// workspace, so an owner can write a grant for any user id at all. Membership
// is what keeps it from being enforced, and it is the only thing that does.
func TestStanding_GrantToNonMemberReachesNothing(t *testing.T) {
	conn := newTestDB(t)

	outsiderID, ownerID, wsID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, outsiderID, "outsider")
	seedUser(t, conn, ownerID, "owner")
	seedWorkspace(t, conn, wsID, "ws", ownerID)
	seedRole(t, conn, roleID, wsID, "admin")

	// The grant, with no workspace_to_user row to go with it.
	seedUserToRole(t, conn, newID(), outsiderID, roleID, wsID)

	require.NotContains(t, effectiveRoles(t, outsiderID, wsID), roleID,
		"a role granted to somebody who does not belong to the workspace still reaches them")
}

// A role is soft-deleted too, and the direct branch carries its own predicate
// for it. TestStanding_SoftDeletedGroupGrantsNoRoles covers the group's.
func TestStanding_SoftDeletedRoleGrantsNothing(t *testing.T) {
	conn := newTestDB(t)

	userID, ownerID, wsID, roleID := newID(), newID(), newID(), newID()
	seedUser(t, conn, userID, "member")
	seedUser(t, conn, ownerID, "owner")
	seedWorkspace(t, conn, wsID, "ws", ownerID)
	seedRole(t, conn, roleID, wsID, "direct-role")
	seedMembership(t, conn, wsID, userID)
	seedUserToRole(t, conn, newID(), userID, roleID, wsID)
	require.Contains(t, effectiveRoles(t, userID, wsID), roleID, "role held before the delete")

	_, err := conn.Exec(`UPDATE app.role SET deleted_at = now() WHERE id = $1::uuid`, roleID)
	require.NoError(t, err)

	require.NotContains(t, effectiveRoles(t, userID, wsID), roleID,
		"a soft-deleted role is still granted")
}
