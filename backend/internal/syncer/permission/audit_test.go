package permission_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the permission entity: each operation must emit the audit
// event its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func TestAudit_PermissionUpserted(t *testing.T) {
	f := e2e.Setup(t)

	permID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "permission", permID, map[string]any{
		"id":      permID,
		"role_id": f.Actor.RoleID,
		"action":  "select",
		"effect":  "allow",
	})

	e2e.RequireEvent(t, f.Conn, "iam", "permission.upserted")
}

func TestAudit_PermissionDeleted(t *testing.T) {
	f := e2e.Setup(t)

	permID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "permission", permID, map[string]any{
		"id": permID, "role_id": f.Actor.RoleID, "action": "select", "effect": "allow",
	})
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "permission", permID, map[string]any{"id": permID})

	e2e.RequireEvent(t, f.Conn, "iam", "permission.deleted")
}
