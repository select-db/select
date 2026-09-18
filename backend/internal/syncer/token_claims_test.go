package syncer

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/auth"
	"backend/internal/syncer/types"
)

// The JWT bakes a user's effective roles in at issuance, so a role, group or
// membership change reaches the user only through the next token they are
// issued. CreateJWT reads roles from the database when it mints, so a plain
// refresh already carries the change.
//
// These cover which users a commit reaches. That their session survives it is
// proved by a real refresh in internal/auth/session_continuity_test.go, so
// nothing here counts refresh-token rows as a proxy for it.

// syncAsOwner runs one commit through Sync acting as the workspace owner (owner
// bypasses permission checks), returning after the mutation is applied.
func syncAsOwner(t *testing.T, ownerID, wsID string, c types.Commit) {
	t.Helper()
	_, _, err := Sync(context.Background(), ownerID, []string{wsID}, nil, []string{wsID}, &types.SyncRequest{
		PendingCommits: []types.Commit{c},
	})
	require.NoError(t, err)
}

// nextTokenRoles mints the token the user's next refresh would hand them and
// returns the role ids it grants in the workspace.
func nextTokenRoles(t *testing.T, userID, wsID string) map[string]string {
	t.Helper()
	tok, err := auth.CreateJWT(context.Background(), uuid.MustParse(userID))
	require.NoError(t, err)
	_, claims, err := auth.ValidateJWT(tok)
	require.NoError(t, err)
	return claims.RolesIn(wsID)
}

func TestSync_UserToGroupInsert_ReachesMember(t *testing.T) {
	localSigner(t)
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

	require.Contains(t, nextTokenRoles(t, memberID, wsID), roleID,
		"the group's role must reach the new member's next token")
}

// The fan-out case: a group's role set changes, so every current member's next
// token carries it, and every one of them keeps their session.
func TestSync_GroupToRoleInsert_ReachesAllMembers(t *testing.T) {
	localSigner(t)
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

	require.Contains(t, nextTokenRoles(t, m1, wsID), roleID, "m1's next token carries the role")
	require.Contains(t, nextTokenRoles(t, m2, wsID), roleID, "m2's next token carries the role")
	require.NotContains(t, nextTokenRoles(t, outsider, wsID), roleID,
		"a non-member gains nothing from the group's role")
}

func TestSync_GroupToRoleDelete_ReachesAllMembers(t *testing.T) {
	localSigner(t)
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
	require.Contains(t, nextTokenRoles(t, m1, wsID), roleID, "role held before the commit")

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group_to_role",
		ObjectID: gtrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": gtrID, "workspace_id": wsID},
	})

	require.NotContains(t, nextTokenRoles(t, m1, wsID), roleID, "m1 loses the role")
	require.NotContains(t, nextTokenRoles(t, m2, wsID), roleID, "m2 loses the role")
}

func TestSync_GroupDelete_ReachesAllMembers(t *testing.T) {
	localSigner(t)
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
	require.Contains(t, nextTokenRoles(t, m1, wsID), roleID, "role held before the commit")

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group",
		ObjectID: groupID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": groupID, "workspace_id": wsID},
	})

	require.NotContains(t, nextTokenRoles(t, m1, wsID), roleID, "m1 loses the group-derived role")
	require.NotContains(t, nextTokenRoles(t, m2, wsID), roleID, "m2 loses the group-derived role")
}
