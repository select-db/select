package user_to_group_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the user_to_group entity: each operation must emit the audit
// event its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func addToGroup(t *testing.T, f e2e.Fixture) (groupID, utgID string) {
	t.Helper()
	groupID = uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "group", groupID, map[string]any{"id": groupID, "name": "Eng"})
	utgID = uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "user_to_group", utgID, map[string]any{
		"id": utgID, "user_id": f.Actor.UserID, "group_id": groupID,
	})
	return groupID, utgID
}

func TestAudit_GroupMemberAdded(t *testing.T) {
	f := e2e.Setup(t)
	addToGroup(t, f)
	e2e.RequireEvent(t, f.Conn, "iam", "group.member_added")
}

func TestAudit_GroupMemberRemoved(t *testing.T) {
	f := e2e.Setup(t)
	_, utgID := addToGroup(t, f)
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "user_to_group", utgID, map[string]any{"id": utgID})
	e2e.RequireEvent(t, f.Conn, "iam", "group.member_removed")
}
