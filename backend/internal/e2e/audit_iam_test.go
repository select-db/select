package e2e

import (
	"testing"

	"github.com/google/uuid"

	"backend/internal/test/support"
)

// --- IAM via the sync/patch path -------------------------------------------
//
// Wired today (expect green): iam.permission.upserted, iam.group.upserted.
// Not wired yet (expect red): role.upserted, permission.deleted, group.deleted,
// member.added — the patch delete path has no audit hook and several upserts
// have no AuditConfig.

func TestAudit_Permission_Upserted_IsWired(t *testing.T) {
	f := support.Setup(t)

	permID := uuid.NewString()
	support.SyncCommit(t, f.H, f.Actor, "create", "permission", permID, map[string]any{
		"id":      permID,
		"role_id": f.Actor.RoleID,
		"action":  "select",
		"effect":  "allow",
	})

	support.RequireEvent(t, f.Conn, "iam", "permission.upserted")
}

func TestAudit_Group_Upserted_IsWired(t *testing.T) {
	f := support.Setup(t)

	groupID := uuid.NewString()
	support.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{
		"id":   groupID,
		"name": "Engineering",
	})

	support.RequireEvent(t, f.Conn, "iam", "group.upserted")
}

func TestAudit_Role_Upserted_NotWiredYet(t *testing.T) {
	f := support.Setup(t)

	roleID := uuid.NewString()
	support.SyncCommit(t, f.H, f.Actor, "create", "role", roleID, map[string]any{
		"id":   roleID,
		"name": "Analyst",
	})

	support.RequireEvent(t, f.Conn, "iam", "role.upserted")
}

func TestAudit_Permission_Deleted_NotWiredYet(t *testing.T) {
	f := support.Setup(t)

	permID := uuid.NewString()
	support.SyncCommit(t, f.H, f.Actor, "create", "permission", permID, map[string]any{
		"id": permID, "role_id": f.Actor.RoleID, "action": "select", "effect": "allow",
	})
	support.SyncCommit(t, f.H, f.Actor, "delete", "permission", permID, map[string]any{"id": permID})

	support.RequireEvent(t, f.Conn, "iam", "permission.deleted")
}

func TestAudit_Group_Deleted_NotWiredYet(t *testing.T) {
	f := support.Setup(t)

	groupID := uuid.NewString()
	support.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Temp"})
	support.SyncCommit(t, f.H, f.Actor, "delete", "group", groupID, map[string]any{"id": groupID})

	support.RequireEvent(t, f.Conn, "iam", "group.deleted")
}
