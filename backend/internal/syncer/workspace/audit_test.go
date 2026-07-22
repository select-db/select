package workspace_test

import (
	"testing"

	"backend/e2e"
)

// Audit coverage for the workspace entity via the sync path. Creation isn't
// reachable here — a caller must already belong to the workspace it commits to,
// so workspace.created lives on the dedicated /workspace/create handler. Deletion
// is owner-only and reachable, so it is covered (see
// backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

func TestAudit_WorkspaceDeleted(t *testing.T) {
	f := e2e.Setup(t)
	e2e.SyncCommit(t, f.H, f.Actor, "delete", "workspace", f.Actor.WorkspaceID, map[string]any{"id": f.Actor.WorkspaceID})
	e2e.RequireEvent(t, f.Conn, "iam", "workspace.deleted")
}
