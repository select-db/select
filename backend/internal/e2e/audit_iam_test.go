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
	conn := support.NewDB(t)
	support.StartAudit(t, conn)
	actor := support.NewAccount(t, conn)
	h := support.NewHandler()

	permID := uuid.NewString()
	support.SyncCommit(t, h, actor, "create", "permission", permID, map[string]any{
		"id":      permID,
		"role_id": actor.RoleID,
		"action":  "select",
		"effect":  "allow",
	})

	support.RequireEvent(t, conn, "iam", "permission.upserted")
}

func TestAudit_Group_Upserted_IsWired(t *testing.T) {
	conn := support.NewDB(t)
	support.StartAudit(t, conn)
	actor := support.NewAccount(t, conn)
	h := support.NewHandler()

	groupID := uuid.NewString()
	support.SyncCommit(t, h, actor, "create", "group", groupID, map[string]any{
		"id":   groupID,
		"name": "Engineering",
	})

	support.RequireEvent(t, conn, "iam", "group.upserted")
}

func TestAudit_Role_Upserted_NotWiredYet(t *testing.T) {
	conn := support.NewDB(t)
	support.StartAudit(t, conn)
	actor := support.NewAccount(t, conn)
	h := support.NewHandler()

	roleID := uuid.NewString()
	support.SyncCommit(t, h, actor, "create", "role", roleID, map[string]any{
		"id":   roleID,
		"name": "Analyst",
	})

	support.RequireEvent(t, conn, "iam", "role.upserted")
}

func TestAudit_Permission_Deleted_NotWiredYet(t *testing.T) {
	conn := support.NewDB(t)
	support.StartAudit(t, conn)
	actor := support.NewAccount(t, conn)
	h := support.NewHandler()

	permID := uuid.NewString()
	support.SyncCommit(t, h, actor, "create", "permission", permID, map[string]any{
		"id": permID, "role_id": actor.RoleID, "action": "select", "effect": "allow",
	})
	support.SyncCommit(t, h, actor, "delete", "permission", permID, map[string]any{"id": permID})

	support.RequireEvent(t, conn, "iam", "permission.deleted")
}

func TestAudit_Group_Deleted_NotWiredYet(t *testing.T) {
	conn := support.NewDB(t)
	support.StartAudit(t, conn)
	actor := support.NewAccount(t, conn)
	h := support.NewHandler()

	groupID := uuid.NewString()
	support.SyncCommit(t, h, actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Temp"})
	support.SyncCommit(t, h, actor, "delete", "group", groupID, map[string]any{"id": groupID})

	support.RequireEvent(t, conn, "iam", "group.deleted")
}
