package syncer

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/auth"
)

func seedUserPlaceholder(t *testing.T, conn *sql.DB, email string) string {
	t.Helper()
	id, err := db.Queries.InsertUserPlaceholder(context.Background(), generated.InsertUserPlaceholderParams{
		Email: db_types.NewJSONNullString(email),
		Name:  db_types.NewJSONNullString(email),
	})
	require.NoError(t, err)
	return id.String()
}

func seedWorkspaceToUser(t *testing.T, conn *sql.DB, workspaceID, userID string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO app.workspace_to_user (id, workspace_id, user_id, updated_at) VALUES (gen_random_uuid(), $1::uuid, $2::uuid, NOW())`,
		workspaceID, userID,
	)
	require.NoError(t, err)
}

// TestPlaceholderResolution_FoundAndLinked verifies that a placeholder user
// (created via invite, email only, no identity) is found by
// GetUserByEmailNoIdentity and can be updated with GitHub profile data.
func TestPlaceholderResolution_FoundAndLinked(t *testing.T) {
	conn := newTestDB(t)

	email := "invited@example.com"
	placeholderID := seedUserPlaceholder(t, conn, email)

	// Placeholder should have no identity
	row, err := db.Queries.GetUserByEmailNoIdentity(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, placeholderID, row.ID.String())
	assert.Equal(t, email, row.Email.String)

	// Simulate what pollForAccessToken does when resolving a placeholder
	err = db.Queries.UpdateUserForLogin(context.Background(), generated.UpdateUserForLoginParams{
		ID:        row.ID,
		Name:      db_types.NewJSONNullString("Invited User"),
		GithubID:  db_types.NewJSONNullInt64(12345),
		Email:     db_types.NewJSONNullString(email),
		AvatarUrl: db_types.NewJSONNullString("https://avatars.example.com/12345"),
	})
	require.NoError(t, err)

	err = db.Queries.CreateUserIdentity(context.Background(), generated.CreateUserIdentityParams{
		UserID:         row.ID,
		Provider:       db_types.NewJSONNullString("github"),
		ProviderUserID: db_types.NewJSONNullString("12345"),
		Email:          db_types.NewJSONNullString(email),
	})
	require.NoError(t, err)

	// After resolution, GetUserByEmailNoIdentity should NOT find this user anymore
	_, err = db.Queries.GetUserByEmailNoIdentity(context.Background(), email)
	assert.ErrorIs(t, err, sql.ErrNoRows)

	// Verify updated fields
	var name, avatarURL string
	var githubID int64
	err = conn.QueryRow(
		`SELECT name, github_id, avatar_url FROM app."user" WHERE id = $1::uuid`,
		placeholderID,
	).Scan(&name, &githubID, &avatarURL)
	require.NoError(t, err)
	assert.Equal(t, "Invited User", name)
	assert.Equal(t, int64(12345), githubID)
	assert.Equal(t, "https://avatars.example.com/12345", avatarURL)
}

// TestPlaceholderResolution_NotFoundForRealUser verifies that a user who
// already has an identity is NOT returned by GetUserByEmailNoIdentity.
func TestPlaceholderResolution_NotFoundForRealUser(t *testing.T) {
	conn := newTestDB(t)

	userID := newID()
	seedUser(t, conn, userID, "Real User")

	// Create identity for this user
	userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
	require.NoError(t, err)
	err = db.Queries.CreateUserIdentity(context.Background(), generated.CreateUserIdentityParams{
		UserID:         userUUID,
		Provider:       db_types.NewJSONNullString("github"),
		ProviderUserID: db_types.NewJSONNullString("99999"),
		Email:          db_types.NewJSONNullString(userID + "@test.local"),
	})
	require.NoError(t, err)

	_, err = db.Queries.GetUserByEmailNoIdentity(context.Background(), userID+"@test.local")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// TestPlaceholderResolution_PreservesWorkspaceMembership verifies that when a
// placeholder user is resolved, their existing workspace memberships are preserved.
func TestPlaceholderResolution_PreservesWorkspaceMembership(t *testing.T) {
	conn := newTestDB(t)

	// Create an owner and workspace
	ownerID := newID()
	wsID := newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "Team WS", ownerID)

	// Invite a placeholder user and add them to the workspace
	email := "newguy@example.com"
	placeholderID := seedUserPlaceholder(t, conn, email)
	seedWorkspaceToUser(t, conn, wsID, placeholderID)

	// Resolve the placeholder (simulating login)
	row, err := db.Queries.GetUserByEmailNoIdentity(context.Background(), email)
	require.NoError(t, err)

	err = db.Queries.UpdateUserForLogin(context.Background(), generated.UpdateUserForLoginParams{
		ID:        row.ID,
		Name:      db_types.NewJSONNullString("New Guy"),
		GithubID:  db_types.NewJSONNullInt64(77777),
		Email:     db_types.NewJSONNullString(email),
		AvatarUrl: db_types.NewJSONNullString("https://avatars.example.com/77777"),
	})
	require.NoError(t, err)

	err = db.Queries.CreateUserIdentity(context.Background(), generated.CreateUserIdentityParams{
		UserID:         row.ID,
		Provider:       db_types.NewJSONNullString("github"),
		ProviderUserID: db_types.NewJSONNullString("77777"),
		Email:          db_types.NewJSONNullString(email),
	})
	require.NoError(t, err)

	// EnsureDefaultWorkspaceForUser should NOT create a new workspace
	// because the user already has one via the invite
	err = auth.EnsureDefaultWorkspaceForUser(context.Background(), placeholderID)
	require.NoError(t, err)

	userUUID, err := db_types.NewJSONNullUUIDFromString(placeholderID)
	require.NoError(t, err)
	count, err := db.Queries.CountWorkspaceToUserByUserID(context.Background(), userUUID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "should still have exactly one workspace membership (the invited one)")
}

// TestReactivateWorkspaceToUser verifies that a soft-deleted workspace_to_user
// record can be reactivated (deleted_at set back to NULL).
func TestReactivateWorkspaceToUser(t *testing.T) {
	conn := newTestDB(t)

	ownerID, wsID := newID(), newID()
	memberID := newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedWorkspaceToUser(t, conn, wsID, memberID)

	wsUUID, err := db_types.NewJSONNullUUIDFromString(wsID)
	require.NoError(t, err)
	memberUUID, err := db_types.NewJSONNullUUIDFromString(memberID)
	require.NoError(t, err)

	// Soft-delete the membership
	_, err = conn.Exec(
		`UPDATE app.workspace_to_user SET deleted_at = NOW() WHERE workspace_id = $1::uuid AND user_id = $2::uuid`,
		wsID, memberID,
	)
	require.NoError(t, err)

	// Verify it's soft-deleted
	var deletedAt *string
	err = conn.QueryRow(
		`SELECT deleted_at::text FROM app.workspace_to_user WHERE workspace_id = $1::uuid AND user_id = $2::uuid`,
		wsID, memberID,
	).Scan(&deletedAt)
	require.NoError(t, err)
	require.NotNil(t, deletedAt)

	// Reactivate
	wtuID, err := db.Queries.ReactivateWorkspaceToUser(context.Background(), generated.ReactivateWorkspaceToUserParams{
		WorkspaceID: wsUUID,
		UserID:      memberUUID,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, wtuID.String())

	// Verify deleted_at is NULL again
	var restoredDeletedAt *string
	err = conn.QueryRow(
		`SELECT deleted_at::text FROM app.workspace_to_user WHERE workspace_id = $1::uuid AND user_id = $2::uuid`,
		wsID, memberID,
	).Scan(&restoredDeletedAt)
	require.NoError(t, err)
	assert.Nil(t, restoredDeletedAt, "deleted_at should be NULL after reactivation")
}

// TestReactivateWorkspaceToUser_NoSoftDeletedRow verifies that reactivation
// returns sql.ErrNoRows when there's no soft-deleted record to restore.
func TestReactivateWorkspaceToUser_NoSoftDeletedRow(t *testing.T) {
	conn := newTestDB(t)

	ownerID, wsID := newID(), newID()
	memberID := newID()
	seedUser(t, conn, ownerID, "Owner")
	seedUser(t, conn, memberID, "Member")
	seedWorkspace(t, conn, wsID, "WS", ownerID)
	seedWorkspaceToUser(t, conn, wsID, memberID)

	wsUUID, err := db_types.NewJSONNullUUIDFromString(wsID)
	require.NoError(t, err)
	memberUUID, err := db_types.NewJSONNullUUIDFromString(memberID)
	require.NoError(t, err)

	// Reactivate on an active record: should return ErrNoRows (deleted_at IS NOT NULL doesn't match)
	_, err = db.Queries.ReactivateWorkspaceToUser(context.Background(), generated.ReactivateWorkspaceToUserParams{
		WorkspaceID: wsUUID,
		UserID:      memberUUID,
	})
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// TestAddMember_FullFlow verifies the full add-member flow: search finds no user,
// create placeholder, insert workspace_to_user, then reactivate after soft-delete.
func TestAddMember_FullFlow(t *testing.T) {
	conn := newTestDB(t)

	ownerID, wsID := newID(), newID()
	seedUser(t, conn, ownerID, "Owner")
	seedWorkspace(t, conn, wsID, "WS", ownerID)

	wsUUID, err := db_types.NewJSONNullUUIDFromString(wsID)
	require.NoError(t, err)

	email := "newmember@example.com"

	// 1. Search: user not found
	_, err = db.Queries.GetUserByEmail(context.Background(), generated.GetUserByEmailParams{
		Lower:       email,
		WorkspaceID: wsUUID,
	})
	assert.ErrorIs(t, err, sql.ErrNoRows)

	// 2. Create placeholder
	userID, err := db.Queries.InsertUserPlaceholder(context.Background(), generated.InsertUserPlaceholderParams{
		Email: db_types.NewJSONNullString(email),
		Name:  db_types.NewJSONNullString(email),
	})
	require.NoError(t, err)

	// 3. Add to workspace
	wtuID, err := db.Queries.InsertWorkspaceToUserForAdd(context.Background(), generated.InsertWorkspaceToUserForAddParams{
		WorkspaceID: wsUUID,
		UserID:      userID,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, wtuID.String())

	// 4. Verify search now returns found + is_member
	row, err := db.Queries.GetUserByEmail(context.Background(), generated.GetUserByEmailParams{
		Lower:       email,
		WorkspaceID: wsUUID,
	})
	require.NoError(t, err)
	assert.True(t, row.IsMember)
	assert.Equal(t, userID.String(), row.ID.String())

	// 5. Soft-delete
	_, err = conn.Exec(
		`UPDATE app.workspace_to_user SET deleted_at = NOW() WHERE id = $1::uuid`, wtuID.String(),
	)
	require.NoError(t, err)

	// 6. Search again: found but not a member
	row, err = db.Queries.GetUserByEmail(context.Background(), generated.GetUserByEmailParams{
		Lower:       email,
		WorkspaceID: wsUUID,
	})
	require.NoError(t, err)
	assert.False(t, row.IsMember)

	// 7. Reactivate
	reactivatedID, err := db.Queries.ReactivateWorkspaceToUser(context.Background(), generated.ReactivateWorkspaceToUserParams{
		WorkspaceID: wsUUID,
		UserID:      userID,
	})
	require.NoError(t, err)
	assert.Equal(t, wtuID.String(), reactivatedID.String())

	// 8. Verify member again
	row, err = db.Queries.GetUserByEmail(context.Background(), generated.GetUserByEmailParams{
		Lower:       email,
		WorkspaceID: wsUUID,
	})
	require.NoError(t, err)
	assert.True(t, row.IsMember)
}
