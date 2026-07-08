package syncer

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/syncer/types"
)

// The JWT bakes a user's effective roles in at issuance (short TTL). A role,
// group, or membership change only reaches the user once they obtain a fresh
// token, so every sync mutation that shifts effective roles must revoke the
// affected users' refresh tokens. These tests drive the real Sync entrypoint as
// the workspace owner and assert exactly whose tokens get revoked.

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

func TestSync_UserToRoleInsert_RevokesMemberTokens(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	memberID, utrID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedRefreshToken(t, conn, memberID)
	require.Equal(t, 1, countRefreshTokens(t, conn, memberID))

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "INSERT", TableName: "user_to_role",
		ObjectID: utrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": utrID, "user_id": memberID, "role_id": roleID, "workspace_id": wsID},
	})

	require.Equal(t, 0, countRefreshTokens(t, conn, memberID),
		"granting a role must revoke the member's refresh tokens")
}

func TestSync_UserToRoleDelete_RevokesMemberTokens(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID := newID(), newID(), newID()
	memberID, utrID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedUserToRole(t, conn, utrID, memberID, roleID, wsID)
	seedRefreshToken(t, conn, memberID)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "user_to_role",
		ObjectID: utrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": utrID, "workspace_id": wsID},
	})

	require.Equal(t, 0, countRefreshTokens(t, conn, memberID),
		"removing a role must revoke the member's refresh tokens")
}

func TestSync_UserToGroupInsert_RevokesMemberTokens(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, groupID := newID(), newID(), newID()
	memberID, utgID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedRefreshToken(t, conn, memberID)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "INSERT", TableName: "user_to_group",
		ObjectID: utgID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": utgID, "user_id": memberID, "group_id": groupID, "workspace_id": wsID},
	})

	require.Equal(t, 0, countRefreshTokens(t, conn, memberID),
		"adding a user to a group must revoke their refresh tokens")
}

// TestSync_GroupToRoleInsert_RevokesAllMembersTokens is the fan-out case: a
// group's role set changes, so EVERY current member's token must be revoked,
// and only members' — an unrelated workspace user's token must survive.
func TestSync_GroupToRoleInsert_RevokesAllMembersTokens(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID, groupID := newID(), newID(), newID(), newID()
	m1, m2, outsider, gtrID := newID(), newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedUser(t, conn, outsider, "Outsider")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
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

	require.Equal(t, 0, countRefreshTokens(t, conn, m1), "member m1 must be revoked")
	require.Equal(t, 0, countRefreshTokens(t, conn, m2), "member m2 must be revoked")
	require.Equal(t, 1, countRefreshTokens(t, conn, outsider),
		"a non-member's token must not be revoked")
}

func TestSync_GroupToRoleDelete_RevokesAllMembersTokens(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, roleID, groupID := newID(), newID(), newID(), newID()
	m1, m2, gtrID := newID(), newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedRole(t, conn, roleID, wsID, "Admins")
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedGroupToRole(t, conn, gtrID, groupID, roleID, wsID)
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)
	seedRefreshToken(t, conn, m1)
	seedRefreshToken(t, conn, m2)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group_to_role",
		ObjectID: gtrID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": gtrID, "workspace_id": wsID},
	})

	require.Equal(t, 0, countRefreshTokens(t, conn, m1), "member m1 must be revoked")
	require.Equal(t, 0, countRefreshTokens(t, conn, m2), "member m2 must be revoked")
}

func TestSync_GroupDelete_RevokesAllMembersTokens(t *testing.T) {
	conn := newTestDB(t)
	ownerID, wsID, groupID := newID(), newID(), newID()
	m1, m2 := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, m1, "M1")
	seedUser(t, conn, m2, "M2")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedGroup(t, conn, groupID, wsID, "Engineering")
	seedUserToGroup(t, conn, newID(), m1, groupID, wsID)
	seedUserToGroup(t, conn, newID(), m2, groupID, wsID)
	seedRefreshToken(t, conn, m1)
	seedRefreshToken(t, conn, m2)

	syncAsOwner(t, ownerID, wsID, types.Commit{
		ID: newID(), Operation: "delete", TableName: "group",
		ObjectID: groupID, WorkspaceID: wsID, UserID: ownerID, CreatedAt: time.Now(),
		Payload: map[string]any{"id": groupID, "workspace_id": wsID},
	})

	require.Equal(t, 0, countRefreshTokens(t, conn, m1), "member m1 must be revoked on group delete")
	require.Equal(t, 0, countRefreshTokens(t, conn, m2), "member m2 must be revoked on group delete")
}
