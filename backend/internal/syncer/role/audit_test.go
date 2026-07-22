package role_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the role entity: each operation must emit the audit event
// its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func TestAudit_RoleUpserted(t *testing.T) {
	f := e2e.Setup(t)

	roleID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "role", roleID, map[string]any{
		"id":   roleID,
		"name": "Analyst",
	})

	e2e.RequireEvent(t, f.Conn, "iam", "role.upserted")
}

func TestAudit_RoleDeleted(t *testing.T) {
	f := e2e.Setup(t)

	roleID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "role", roleID, map[string]any{"id": roleID, "name": "Temp"})
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "role", roleID, map[string]any{"id": roleID})

	e2e.RequireEvent(t, f.Conn, "iam", "role.deleted")
}
