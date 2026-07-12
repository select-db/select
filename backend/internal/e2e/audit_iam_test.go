package e2e

import (
	"testing"

	"github.com/google/uuid"

	"backend/internal/testsupport"
)

// --- IAM via the sync/patch path -------------------------------------------
//
// Wired today (expect green): iam.permission.upserted, iam.group.upserted.
// Not wired yet (expect red): role.upserted, permission.deleted, group.deleted,
// member.added — the patch delete path has no audit hook and several upserts
// have no AuditConfig.

func TestAudit_Permission_Upserted_IsWired(t *testing.T) {
	conn := testsupport.NewDB(t)
	testsupport.StartAudit(t, conn)
	actor := testsupport.NewAccount(t, conn)
	h := testsupport.NewHandler()

	permID := uuid.NewString()
	testsupport.SyncCommit(t, h, actor, "create", "permission", permID, map[string]any{
		"id":      permID,
		"role_id": actor.RoleID,
		"action":  "select",
		"effect":  "allow",
	})

	testsupport.RequireEvent(t, conn, "iam", "permission.upserted")
}

func TestAudit_Group_Upserted_IsWired(t *testing.T) {
	conn := testsupport.NewDB(t)
	testsupport.StartAudit(t, conn)
	actor := testsupport.NewAccount(t, conn)
	h := testsupport.NewHandler()

	groupID := uuid.NewString()
	testsupport.SyncCommit(t, h, actor, "create", "group", groupID, map[string]any{
		"id":   groupID,
		"name": "Engineering",
	})

	testsupport.RequireEvent(t, conn, "iam", "group.upserted")
}

func TestAudit_Role_Upserted_NotWiredYet(t *testing.T) {
	conn := testsupport.NewDB(t)
	testsupport.StartAudit(t, conn)
	actor := testsupport.NewAccount(t, conn)
	h := testsupport.NewHandler()

	roleID := uuid.NewString()
	testsupport.SyncCommit(t, h, actor, "create", "role", roleID, map[string]any{
		"id":   roleID,
		"name": "Analyst",
	})

	testsupport.RequireEvent(t, conn, "iam", "role.upserted")
}

func TestAudit_Permission_Deleted_NotWiredYet(t *testing.T) {
	conn := testsupport.NewDB(t)
	testsupport.StartAudit(t, conn)
	actor := testsupport.NewAccount(t, conn)
	h := testsupport.NewHandler()

	permID := uuid.NewString()
	testsupport.SyncCommit(t, h, actor, "create", "permission", permID, map[string]any{
		"id": permID, "role_id": actor.RoleID, "action": "select", "effect": "allow",
	})
	testsupport.SyncCommit(t, h, actor, "delete", "permission", permID, map[string]any{"id": permID})

	testsupport.RequireEvent(t, conn, "iam", "permission.deleted")
}

func TestAudit_Group_Deleted_NotWiredYet(t *testing.T) {
	conn := testsupport.NewDB(t)
	testsupport.StartAudit(t, conn)
	actor := testsupport.NewAccount(t, conn)
	h := testsupport.NewHandler()

	groupID := uuid.NewString()
	testsupport.SyncCommit(t, h, actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Temp"})
	testsupport.SyncCommit(t, h, actor, "delete", "group", groupID, map[string]any{"id": groupID})

	testsupport.RequireEvent(t, conn, "iam", "group.deleted")
}
