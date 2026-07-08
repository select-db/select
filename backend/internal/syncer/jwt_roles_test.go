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
	"testing"

	"github.com/stretchr/testify/require"

	"backend/db/db_types"
	"backend/internal/auth"

	"github.com/google/uuid"
)

// localSigner points auth.CreateJWT at an in-process RSA key so the proxified
// token can actually be issued and re-parsed inside the test.
func localSigner(t *testing.T) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(priv)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "jwt.pem")
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))
	t.Setenv("SELECTDB_KEK", "dev") // localMode -> in-process signer
	t.Setenv("PRIVATE_KEY_PATH", path)
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

// workspaceRoleIDs pulls the role ID set the token grants for one workspace.
func workspaceRoleIDs(claims *auth.CustomClaims, workspaceID string) map[string]string {
	out := map[string]string{}
	for _, ws := range claims.Workspaces {
		if ws.ID != workspaceID {
			continue
		}
		for _, r := range ws.Roles {
			out[r.ID] = r.Name
		}
	}
	return out
}

// TestCreateJWT_UnionsDirectAndGroupRoles is the proxified-path analogue of the
// client-side TestGroupRoleFlow_AppliesToLocalPermissions: a proxified backend
// derives a user's authority from the JWT, so the token must embed BOTH
// directly-assigned roles (user_to_role) and roles granted through a group
// (user_to_group -> group_to_role).
func TestCreateJWT_UnionsDirectAndGroupRoles(t *testing.T) {
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

	// membership so the workspace appears in the token
	_, err := conn.Exec(
		`INSERT INTO app.workspace_to_user (id, user_id, workspace_id) VALUES ($1::uuid,$2::uuid,$3::uuid)`,
		newID(), userID, wsID,
	)
	require.NoError(t, err)

	seedUserToRole(t, conn, newID(), userID, directRoleID, wsID)
	seedUserToGroup(t, conn, newID(), userID, groupID, wsID)
	seedGroupToRole(t, conn, newID(), groupID, groupRoleID, wsID)

	tok, err := auth.CreateJWT(context.Background(), db_types.NewJSONNullUUID(uuid.MustParse(userID)))
	require.NoError(t, err)

	_, claims, err := auth.ValidateJWT(tok)
	require.NoError(t, err)

	got := workspaceRoleIDs(claims, wsID)
	require.Contains(t, got, directRoleID, "direct role must be in the token")
	require.Contains(t, got, groupRoleID, "group-granted role must be in the token")
	require.Equal(t, "direct-role", got[directRoleID])
	require.Equal(t, "group-role", got[groupRoleID])
}

// TestCreateJWT_DedupesRoleGrantedBothWays proves the per-workspace dedup: a
// role assigned directly AND through a group appears exactly once in the claim.
func TestCreateJWT_DedupesRoleGrantedBothWays(t *testing.T) {
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

	_, err := conn.Exec(
		`INSERT INTO app.workspace_to_user (id, user_id, workspace_id) VALUES ($1::uuid,$2::uuid,$3::uuid)`,
		newID(), userID, wsID,
	)
	require.NoError(t, err)

	// same role, both paths
	seedUserToRole(t, conn, newID(), userID, roleID, wsID)
	seedUserToGroup(t, conn, newID(), userID, groupID, wsID)
	seedGroupToRole(t, conn, newID(), groupID, roleID, wsID)

	tok, err := auth.CreateJWT(context.Background(), db_types.NewJSONNullUUID(uuid.MustParse(userID)))
	require.NoError(t, err)

	_, claims, err := auth.ValidateJWT(tok)
	require.NoError(t, err)

	count := 0
	for _, ws := range claims.Workspaces {
		if ws.ID != wsID {
			continue
		}
		for _, r := range ws.Roles {
			if r.ID == roleID {
				count++
			}
		}
	}
	require.Equal(t, 1, count, "role granted both directly and via group must appear once")
}

// TestCreateJWT_SoftDeletedGroupGrantsNoRoles guards the same soft-delete bug on
// the token path that TestGroupRoles_SoftDeletedGroupGrantsNoRoles guards on the
// query path: deleting the group must drop its roles from freshly-issued tokens.
func TestCreateJWT_SoftDeletedGroupGrantsNoRoles(t *testing.T) {
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

	_, err := conn.Exec(
		`INSERT INTO app.workspace_to_user (id, user_id, workspace_id) VALUES ($1::uuid,$2::uuid,$3::uuid)`,
		newID(), userID, wsID,
	)
	require.NoError(t, err)

	seedUserToGroup(t, conn, newID(), userID, groupID, wsID)
	seedGroupToRole(t, conn, newID(), groupID, groupRoleID, wsID)

	// soft-delete the group
	_, err = conn.Exec(`UPDATE app."group" SET deleted_at = now() WHERE id = $1::uuid`, groupID)
	require.NoError(t, err)

	tok, err := auth.CreateJWT(context.Background(), db_types.NewJSONNullUUID(uuid.MustParse(userID)))
	require.NoError(t, err)

	_, claims, err := auth.ValidateJWT(tok)
	require.NoError(t, err)

	got := workspaceRoleIDs(claims, wsID)
	require.NotContains(t, got, groupRoleID, "soft-deleted group must not grant roles in the token")
}
