package user_to_role_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the user_to_role entity (a role granted directly to a user):
// each operation must emit the audit event its catalog spec declares (see
// backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

// grantUserRole seeds a fresh user and grants it the actor's role, returning the
// new user id and the user_to_role row id.
func grantUserRole(t *testing.T, f e2e.Fixture) (userID, utrID string) {
	t.Helper()
	userID = uuid.NewString()
	e2e.SeedUser(t, f.Conn, userID)
	utrID = uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "user_to_role", utrID, map[string]any{
		"id": utrID, "user_id": userID, "role_id": f.Actor.RoleID,
	})
	return userID, utrID
}

func TestAudit_UserRoleGranted(t *testing.T) {
	f := e2e.Setup(t)
	grantUserRole(t, f)
	e2e.RequireEvent(t, f.Conn, "iam", "user.role.grant")
}

func TestAudit_UserRoleRevoked(t *testing.T) {
	f := e2e.Setup(t)
	_, utrID := grantUserRole(t, f)
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "user_to_role", utrID, map[string]any{"id": utrID})
	e2e.RequireEvent(t, f.Conn, "iam", "user.role.revoke")
}
