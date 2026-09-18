package workspace_to_user_test

import (
	"testing"

	"github.com/google/uuid"

	"backend/e2e"
)

// Audit coverage for the workspace_to_user entity (workspace membership): each
// operation must emit the audit event its catalog spec declares (see
// backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

// addMember seeds a fresh user and adds it to the actor's workspace, returning
// the new user id and the membership row id.
func addMember(t *testing.T, f e2e.Fixture) (userID, wtuID string) {
	t.Helper()
	userID = uuid.NewString()
	e2e.SeedUser(t, f.Conn, userID)
	wtuID = uuid.NewString()
	e2e.SyncCommit(t, f.H, f.Actor, "create", "workspace_to_user", wtuID, map[string]any{
		"id": wtuID, "user_id": userID,
	})
	return userID, wtuID
}

func TestAudit_WorkspaceUserAdded(t *testing.T) {
	f := e2e.Setup(t)
	addMember(t, f)
	e2e.RequireEvent(t, f.Conn, "iam", "workspace.user_membership.add")
}

func TestAudit_WorkspaceUserRemoved(t *testing.T) {
	f := e2e.Setup(t)
	_, wtuID := addMember(t, f)
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "workspace_to_user", wtuID, map[string]any{"id": wtuID})
	e2e.RequireEvent(t, f.Conn, "iam", "workspace.user_membership.remove")
}
