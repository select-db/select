package e2e

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/auth"
)

// Actor is a crafted account: a user, a workspace they own and belong to, a role
// assigned to them, and a freshly minted access token. It is the standard
// authenticated identity for e2e requests and is reused across the suite.
type Actor struct {
	UserID      string
	WorkspaceID string
	RoleID      string
	Token       string
}

// NewAccount crafts a fully wired owner identity and mints its JWT. Ownership +
// the assigned role make the token pass Authenticated/Membership and the authz
// gates the handlers apply, without going through the GitHub device-code flow.
func NewAccount(t *testing.T, conn *sql.DB) Actor {
	t.Helper()
	userID := uuid.NewString()
	workspaceID := uuid.NewString()
	roleID := uuid.NewString()

	SeedUser(t, conn, userID)
	SeedWorkspace(t, conn, workspaceID, userID)
	SeedMembership(t, conn, workspaceID, userID)
	SeedRole(t, conn, roleID, workspaceID, "Owner Role")
	SeedUserRole(t, conn, userID, roleID, workspaceID)

	return Actor{
		UserID:      userID,
		WorkspaceID: workspaceID,
		RoleID:      roleID,
		Token:       MintJWT(t, userID),
	}
}

// MintJWT issues an access token naming userID. It carries identity only:
// workspaces, ownership and roles are derived per request, so when a token was
// minted relative to a seed makes no difference to what it reaches.
func MintJWT(t *testing.T, userID string) string {
	t.Helper()
	uid, err := uuid.Parse(userID)
	require.NoError(t, err)
	token, err := auth.CreateJWT(context.Background(), uid)
	require.NoError(t, err)
	return token
}

func SeedUser(t *testing.T, conn *sql.DB, id string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO app."user" (id, name, email) VALUES ($1::uuid, $2, $3)`,
		id, "user-"+id[:8], id+"@test.local")
	require.NoError(t, err)
}

func SeedWorkspace(t *testing.T, conn *sql.DB, id, ownerID string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO app.workspace (id, name, owner_id) VALUES ($1::uuid, $2, $3::uuid)`,
		id, "ws-"+id[:8], ownerID)
	require.NoError(t, err)
}

// SeedMembership adds a user to a workspace and returns the membership's id,
// which is what a sync commit addressing that membership names.
func SeedMembership(t *testing.T, conn *sql.DB, workspaceID, userID string) string {
	t.Helper()
	id := uuid.NewString()
	_, err := conn.Exec(`INSERT INTO app.workspace_to_user (id, workspace_id, user_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		id, workspaceID, userID)
	require.NoError(t, err)
	return id
}

func SeedRole(t *testing.T, conn *sql.DB, id, workspaceID, name string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO app.role (id, workspace_id, name) VALUES ($1::uuid, $2::uuid, $3)`,
		id, workspaceID, name)
	require.NoError(t, err)
}

// SeedPermission grants a role one workspace-level action.
func SeedPermission(t *testing.T, conn *sql.DB, roleID, workspaceID, action string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO app.permission (id, role_id, workspace_id, action, effect) VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'allow')`,
		uuid.NewString(), roleID, workspaceID, action)
	require.NoError(t, err)
}

// SeedRoleWithPermission returns a new role in the workspace carrying one
// action, for tests that ask whether somebody may act rather than what they hold.
func SeedRoleWithPermission(t *testing.T, conn *sql.DB, workspaceID, name, action string) string {
	t.Helper()
	roleID := uuid.NewString()
	SeedRole(t, conn, roleID, workspaceID, name)
	SeedPermission(t, conn, roleID, workspaceID, action)
	return roleID
}

func SeedUserRole(t *testing.T, conn *sql.DB, userID, roleID, workspaceID string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO app.user_to_role (id, user_id, role_id, workspace_id) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)`,
		uuid.NewString(), userID, roleID, workspaceID)
	require.NoError(t, err)
}
