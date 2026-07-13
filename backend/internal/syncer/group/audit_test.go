package group_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the group entity's emit sites. A _IsWired test must stay
// green; a _NotWiredYet test is red until the emit site lands (see
// backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func TestAudit_GroupUpserted_IsWired(t *testing.T) {
	f := e2e.Setup(t)

	groupID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{
		"id":   groupID,
		"name": "Engineering",
	})

	e2e.RequireEvent(t, f.Conn, "iam", "group.upserted")
}

func TestAudit_GroupDeleted_NotWiredYet(t *testing.T) {
	f := e2e.Setup(t)

	groupID := uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Temp"})
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "group", groupID, map[string]any{"id": groupID})

	e2e.RequireEvent(t, f.Conn, "iam", "group.deleted")
}
