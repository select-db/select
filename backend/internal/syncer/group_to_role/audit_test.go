package group_to_role_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the group_to_role entity: each operation must emit the audit
// event its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func attachRole(t *testing.T, f e2e.Fixture) (groupID, gtrID string) {
	t.Helper()
	groupID = uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Eng"})
	gtrID = uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group_to_role", gtrID, map[string]any{
		"id": gtrID, "group_id": groupID, "role_id": f.Actor.RoleID,
	})
	return groupID, gtrID
}

func TestAudit_GroupRoleAttached(t *testing.T) {
	f := e2e.Setup(t)
	attachRole(t, f)
	e2e.RequireEvent(t, f.Conn, "iam", "group.role_attached")
}

func TestAudit_GroupRoleDetached(t *testing.T) {
	f := e2e.Setup(t)
	_, gtrID := attachRole(t, f)
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "group_to_role", gtrID, map[string]any{"id": gtrID})
	e2e.RequireEvent(t, f.Conn, "iam", "group.role_detached")
}
