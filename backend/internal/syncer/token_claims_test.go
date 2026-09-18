package syncer

import (
	"context"
	"database/sql"
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
// refresh already carries the change: the sync needs to do nothing to deliver
// it, and must not take the session away trying.
//
// Each test asserts both halves: the user can still refresh, and the token they
// would get next holds the roles the commit gave or took.

func seedRefreshToken(t *testing.T, conn *sql.DB, userID string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO auth.refresh_token (hashed_token, user_id, expires_at) VALUES ($1, $2::uuid, $3)`,
		newID(), userID, time.Now().Add(24*time.Hour),
	)
	require.NoError(t, err)
}

func countRefreshTokens(t *testing.T, conn *sql.DB, userID string) int {
	t.Helper()
	var n int
	require.NoError(t, conn.QueryRow(
		`SELECT count(*) FROM auth.refresh_token WHERE user_id = $1::uuid`, userID,
	).Scan(&n))
	return n
}

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
	return workspaceRoleIDs(claims, wsID)
}

func TestSync_UserToRoleInsert_ReachesMemberAndKeepsSession(t *testing.T) {
	localSigner(t)
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	memberID, utrID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, memberID, wsID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedRefreshToken(t, conn, memberID)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "INSERT", TableName: "user_to_role",
		ObjectID: utrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": utrID, "user_id": memberID, "role_id": roleID, "workspace_id": wsID},
	})

	require.Contains(t, nextTokenRoles(t, memberID, wsID), roleID,
		"the granted role must reach the member's next token")
	require.Equal(t, 1, countRefreshTokens(t, conn, memberID),
		"granting a role must not sign the member out")
}

func TestSync_UserToRoleDelete_ReachesMemberAndKeepsSession(t *testing.T) {
	localSigner(t)
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	memberID, utrID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, memberID, wsID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedUserToRole(t, conn, utrID, memberID, roleID, wsID)
	seedRefreshToken(t, conn, memberID)
	require.Contains(t, nextTokenRoles(t, memberID, wsID), roleID, "role held before the commit")

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "user_to_role",
		ObjectID: utrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": utrID, "workspace_id": wsID},
	})

	require.NotContains(t, nextTokenRoles(t, memberID, wsID), roleID,
		"the removed role must be gone from the member's next token")
	require.Equal(t, 1, countRefreshTokens(t, conn, memberID),
		"removing a role must not sign the member out")
}

func TestSync_UserToGroupInsert_ReachesMemberAndKeepsSession(t *testing.T) {
	localSigner(t)
	conn := newTestDB(t)
	ownerID, wsID, groupID, roleID := newID(), newID(), newID(), newID()
	memberID, utgID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, memberID, wsID)
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroupToRole(t, conn, newID(), groupID, roleID, wsID)
	seedRefreshToken(t, conn, memberID)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "INSERT", TableName: "user_to_group",
		ObjectID: utgID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": utgID, "user_id": memberID, "group_id": groupID, "workspace_id": wsID},
	})

	require.Contains(t, nextTokenRoles(t, memberID, wsID), roleID,
		"the group's role must reach the new member's next token")
	require.Equal(t, 1, countRefreshTokens(t, conn, memberID),
		"joining a group must not sign the member out")
}

// The fan-out case: a group's role set changes, so every current member's next
// token carries it, and every one of them keeps their session.
func TestSync_GroupToRoleInsert_ReachesAllMembersAndKeepsSessions(t *testing.T) {
	localSigner(t)
	conn := newTestDB(t)
	ownerID, wsID, roleID, groupID := newID(), newID(), newID(), newID()
	m1, m2, outsider, gtrID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedUser(t, conn, outsider, "Outsider")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, m1, wsID)
	seedMembership(t, conn, m2, wsID)
	seedMembership(t, conn, outsider, wsID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)
	seedRefreshToken(t, conn, m1)
	seedRefreshToken(t, conn, m2)
	seedRefreshToken(t, conn, outsider)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "INSERT", TableName: "group_to_role",
		ObjectID: gtrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": gtrID, "group_id": groupID, "role_id": roleID, "workspace_id": wsID},
	})

	require.Contains(t, nextTokenRoles(t, m1, wsID), roleID, "m1's next token carries the role")
	require.Contains(t, nextTokenRoles(t, m2, wsID), roleID, "m2's next token carries the role")
	require.NotContains(t, nextTokenRoles(t, outsider, wsID), roleID,
		"a non-member gains nothing from the group's role")
	require.Equal(t, 1, countRefreshTokens(t, conn, m1), "m1 keeps their session")
	require.Equal(t, 1, countRefreshTokens(t, conn, m2), "m2 keeps their session")
	require.Equal(t, 1, countRefreshTokens(t, conn, outsider), "the outsider keeps their session")
}

func TestSync_GroupToRoleDelete_ReachesAllMembersAndKeepsSessions(t *testing.T) {
	localSigner(t)
	conn := newTestDB(t)
	ownerID, wsID, roleID, groupID := newID(), newID(), newID(), newID()
	m1, m2, gtrID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, m1, wsID)
	seedMembership(t, conn, m2, wsID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedGroupToRole(t, conn, gtrID, groupID, roleID, wsID)
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)
	seedRefreshToken(t, conn, m1)
	seedRefreshToken(t, conn, m2)
	require.Contains(t, nextTokenRoles(t, m1, wsID), roleID, "role held before the commit")

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group_to_role",
		ObjectID: gtrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": gtrID, "workspace_id": wsID},
	})

	require.NotContains(t, nextTokenRoles(t, m1, wsID), roleID, "m1 loses the role")
	require.NotContains(t, nextTokenRoles(t, m2, wsID), roleID, "m2 loses the role")
	require.Equal(t, 1, countRefreshTokens(t, conn, m1), "m1 keeps their session")
	require.Equal(t, 1, countRefreshTokens(t, conn, m2), "m2 keeps their session")
}

func TestSync_GroupDelete_ReachesAllMembersAndKeepsSessions(t *testing.T) {
	localSigner(t)
	conn := newTestDB(t)
	ownerID, wsID, groupID, roleID := newID(), newID(), newID(), newID()
	m1, m2 := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedMembership(t, conn, m1, wsID)
	seedMembership(t, conn, m2, wsID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedGroupToRole(t, conn, newID(), groupID, roleID, wsID)
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)
	seedRefreshToken(t, conn, m1)
	seedRefreshToken(t, conn, m2)
	require.Contains(t, nextTokenRoles(t, m1, wsID), roleID, "role held before the commit")

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group",
		ObjectID: groupID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": groupID, "workspace_id": wsID},
	})

	require.NotContains(t, nextTokenRoles(t, m1, wsID), roleID, "m1 loses the group-derived role")
	require.NotContains(t, nextTokenRoles(t, m2, wsID), roleID, "m2 loses the group-derived role")
	require.Equal(t, 1, countRefreshTokens(t, conn, m1), "m1 keeps their session")
	require.Equal(t, 1, countRefreshTokens(t, conn, m2), "m2 keeps their session")
}
