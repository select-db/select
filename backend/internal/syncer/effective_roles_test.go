package syncer

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/db"

	"github.com/google/uuid"
)

// localSigner points auth.CreateJWT at an in-process RSA key. Done once per
// process: auth caches its signer behind a sync.Once, so a second key would be
// generated and then ignored.
var signerOnce sync.Once

func localSigner(t *testing.T) {
	t.Helper()
	signerOnce.Do(func() { generateLocalSigner(t) })
}

func generateLocalSigner(t *testing.T) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(priv)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	// The key and the env outlive the test that generated them, so neither
	// t.TempDir nor t.Setenv will do: the first would be swept while later tests
	// still need the path, the second restored while they still need the value.
	dir, err := os.MkdirTemp("", "jwt-signer-*")
	require.NoError(t, err)
	path := filepath.Join(dir, "jwt.pem")
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))
	require.NoError(t, os.Setenv("SELECTDB_KEK", "dev")) // localMode -> in-process signer
	require.NoError(t, os.Setenv("PRIVATE_KEY_PATH", path))
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
	localSigner(t)
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
	localSigner(t)
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
	rows, err := db.Queries.GetWorkspaceStandingByUserID(context.Background(), uuid.MustParse(userID))
	require.NoError(t, err)
	for _, row := range rows {
		if row.WorkspaceID.String() == wsID && row.RoleID.Valid && row.RoleID.UUID.String() == roleID {
			count++
		}
	}
	require.Equal(t, 1, count, "role granted both directly and via group must be counted once")
}

// Deleting a group is a soft delete, and the FK cascade only fires on hard
// deletes, so its still-live membership rows must not keep granting its roles.
func TestStanding_SoftDeletedGroupGrantsNoRoles(t *testing.T) {
	localSigner(t)
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
