package group_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the group entity: each operation must emit the audit event
// its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func TestAudit_GroupCreated(t *testing.T) {
	f := e2e.Setup(t)

	groupID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{
		"id":   groupID,
		"name": "Engineering",
	})

	e2e.RequireEvent(t, f.Conn, "iam", "group.lifecycle.create")
}

func TestAudit_GroupUpdated(t *testing.T) {
	f := e2e.Setup(t)

	groupID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Engineering"})
	// A second upsert of the same id is an update, not a create.
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Platform"})

	e2e.RequireEvent(t, f.Conn, "iam", "group.lifecycle.update")
}

func TestAudit_GroupDeleted(t *testing.T) {
	f := e2e.Setup(t)

	groupID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Temp"})
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "group", groupID, map[string]any{"id": groupID})

	e2e.RequireEvent(t, f.Conn, "iam", "group.lifecycle.delete")
}
