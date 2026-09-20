package syncer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/syncer/types"
)

// Which users a role, group or membership commit reaches. Standing is derived
// per request, so a commit reaches everyone it names on their next request;
// these assert who that is.
//
// That the change costs nobody their session is proved by a real refresh in
// internal/auth/session_continuity_test.go.

// syncAsOwner runs one commit through Sync acting as the workspace owner (owner
// bypasses permission checks), returning after the mutation is applied.
func syncAsOwner(t *testing.T, ownerID, wsID string, c types.Commit) {
	t.Helper()
	_, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{c},
	})
	require.NoError(t, err)
}

func TestSync_UserToGroupInsert_ReachesMember(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, groupID, roleID := newID(), newID(), newID(), newID()
	memberID, utgID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, wsID, memberID)
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroupToRole(t, conn, newID(), groupID, roleID, wsID)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "INSERT", TableName: "user_to_group",
		ObjectID: utgID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": utgID, "user_id": memberID, "group_id": groupID, "workspace_id": wsID},
	})

	require.Contains(t, effectiveRoles(t, memberID, wsID), roleID,
		"the group's role must reach the new member")
}

// The fan-out case: a group's role set changes, so every current member's
// standing changes with it on their next request.
func TestSync_GroupToRoleInsert_ReachesAllMembers(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID, groupID := newID(), newID(), newID(), newID()
	m1, m2, outsider, gtrID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedUser(t, conn, outsider, "Outsider")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, wsID, m1)
	seedMembership(t, conn, wsID, m2)
	seedMembership(t, conn, wsID, outsider)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "INSERT", TableName: "group_to_role",
		ObjectID: gtrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": gtrID, "group_id": groupID, "role_id": roleID, "workspace_id": wsID},
	})

	require.Contains(t, effectiveRoles(t, m1, wsID), roleID, "m1's standing has the role")
	require.Contains(t, effectiveRoles(t, m2, wsID), roleID, "m2's standing has the role")
	require.NotContains(t, effectiveRoles(t, outsider, wsID), roleID,
		"a non-member gains nothing from the group's role")
}

func TestSync_GroupToRoleDelete_ReachesAllMembers(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID, groupID := newID(), newID(), newID(), newID()
	m1, m2, gtrID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, wsID, m1)
	seedMembership(t, conn, wsID, m2)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedGroupToRole(t, conn, gtrID, groupID, roleID, wsID)
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)
	require.Contains(t, effectiveRoles(t, m1, wsID), roleID, "role held before the commit")

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group_to_role",
		ObjectID: gtrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": gtrID, "workspace_id": wsID},
	})

	require.NotContains(t, effectiveRoles(t, m1, wsID), roleID, "m1 loses the role")
	require.NotContains(t, effectiveRoles(t, m2, wsID), roleID, "m2 loses the role")
}

func TestSync_GroupDelete_ReachesAllMembers(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, groupID, roleID := newID(), newID(), newID(), newID()
	m1, m2 := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, wsID, m1)
	seedMembership(t, conn, wsID, m2)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedGroupToRole(t, conn, newID(), groupID, roleID, wsID)
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)
	require.Contains(t, effectiveRoles(t, m1, wsID), roleID, "role held before the commit")

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group",
		ObjectID: groupID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": groupID, "workspace_id": wsID},
	})

	require.NotContains(t, effectiveRoles(t, m1, wsID), roleID, "m1 loses the group-derived role")
	require.NotContains(t, effectiveRoles(t, m2, wsID), roleID, "m2 loses the group-derived role")
}
