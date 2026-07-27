package workspace_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
)

// Audit coverage for the dedicated workspace/member handlers: each operation must
// emit the audit event its catalog spec declares (see
// backend/internal/audit/catalog.go). workspace.user_membership.add and
// workspace.lifecycle.delete are also reachable via the sync path; both are
// audited (emit-both, by design).

func TestMain(m *testing.M) { e2e.Run(m) }

// nonOwnerToken mints a token for a plain workspace member (no owner rights, no
// manage permission).
func nonOwnerToken(t *testing.T, f e2e.Fixture) string {
	t.Helper()
	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	return e2e.MintJWT(t, memberID)
}

func TestAudit_WorkspaceCreated(t *testing.T) {
	f := e2e.Setup(t)
	rec := e2e.Do(t, f.H, http.MethodPost, "/workspace/create", f.Actor.Token, map[string]any{"name": "New WS"})
	require.Equalf(t, http.StatusOK, rec.Code, "create: %s", rec.Body.String())
	e2e.RequireEvent(t, f.Conn, "iam", "workspace.lifecycle.create")
}

func TestAudit_WorkspaceDeleted(t *testing.T) {
	f := e2e.Setup(t)
	rec := e2e.Do(t, f.H, http.MethodPost, "/workspace/delete", f.Actor.Token, map[string]any{
		"id": f.Actor.WorkspaceID, "workspace_id": f.Actor.WorkspaceID,
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "delete: %s", rec.Body.String())
	e2e.RequireEvent(t, f.Conn, "iam", "workspace.lifecycle.delete")
}

func TestAudit_WorkspaceDeleteDenied(t *testing.T) {
	f := e2e.Setup(t)
	token := nonOwnerToken(t, f)
	rec := e2e.Do(t, f.H, http.MethodPost, "/workspace/delete", token, map[string]any{
		"id": f.Actor.WorkspaceID, "workspace_id": f.Actor.WorkspaceID,
	})
	require.Equalf(t, http.StatusForbidden, rec.Code, "want 403: %s", rec.Body.String())
	e2e.RequireEventStatus(t, f.Conn, "iam", "workspace.lifecycle.delete", "denied")
}

func TestAudit_WorkspaceUserAdded(t *testing.T) {
	f := e2e.Setup(t)
	email := "member-" + uuid.NewString()[:8] + "@test.local"
	rec := e2e.Do(t, f.H, http.MethodPost, "/user/add", f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID, "email": email,
	})
	require.Equalf(t, http.StatusOK, rec.Code, "add: %s", rec.Body.String())
	e2e.RequireEvent(t, f.Conn, "iam", "workspace.user_membership.add")
}

func TestAudit_MemberAddDenied(t *testing.T) {
	f := e2e.Setup(t)
	token := nonOwnerToken(t, f)
	email := "member-" + uuid.NewString()[:8] + "@test.local"
	rec := e2e.Do(t, f.H, http.MethodPost, "/user/add", token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID, "email": email,
	})
	require.Equalf(t, http.StatusForbidden, rec.Code, "want 403: %s", rec.Body.String())
	e2e.RequireEventStatus(t, f.Conn, "iam", "workspace.user_membership.add", "denied")
}
