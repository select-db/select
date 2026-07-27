package role_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the role entity: each operation must emit the audit event
// its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func TestAudit_RoleCreated(t *testing.T) {
	f := e2e.Setup(t)

	roleID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "role", roleID, map[string]any{
		"id":   roleID,
		"name": "Analyst",
	})

	e2e.RequireEvent(t, f.Conn, "iam", "role.lifecycle.create")
}

func TestAudit_RoleUpdated(t *testing.T) {
	f := e2e.Setup(t)

	roleID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "role", roleID, map[string]any{"id": roleID, "name": "Analyst"})
	// A second upsert of the same id is an update, not a create.
	e2e.SyncCommit(t, f.H, f.Actor, "create", "role", roleID, map[string]any{"id": roleID, "name": "Senior Analyst"})

	e2e.RequireEvent(t, f.Conn, "iam", "role.lifecycle.update")
}

func TestAudit_RoleDeleted(t *testing.T) {
	f := e2e.Setup(t)

	roleID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "role", roleID, map[string]any{"id": roleID, "name": "Temp"})
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "role", roleID, map[string]any{"id": roleID})

	e2e.RequireEvent(t, f.Conn, "iam", "role.lifecycle.delete")
}
