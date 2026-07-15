package apikey_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/e2e"
)

// Audit coverage for the API key management handlers: each operation must emit
// the audit event its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

// createAPIKey creates a key as the owner and returns its id.
func createAPIKey(t *testing.T, f e2e.Fixture) string {
	t.Helper()
	rec := e2e.Do(t, f.H, http.MethodPost, "/apikey/create", f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"name":         "ci",
		"role_ids":     []string{f.Actor.RoleID},
	})
	require.Equalf(t, http.StatusOK, rec.Code, "create failed: %s", rec.Body.String())
	var resp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.ID)
	return resp.ID
}

func TestAudit_APIKeyCreated(t *testing.T) {
	f := e2e.Setup(t)
	createAPIKey(t, f)
	e2e.RequireEvent(t, f.Conn, "iam", "api_key.created")
}

func TestAudit_APIKeyRotated(t *testing.T) {
	f := e2e.Setup(t)
	id := createAPIKey(t, f)
	rec := e2e.Do(t, f.H, http.MethodPost, "/apikey/rotate", f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID, "id": id,
	})
	require.Equalf(t, http.StatusOK, rec.Code, "rotate failed: %s", rec.Body.String())
	e2e.RequireEvent(t, f.Conn, "iam", "api_key.rotated")
}

func TestAudit_APIKeyRevoked(t *testing.T) {
	f := e2e.Setup(t)
	id := createAPIKey(t, f)
	rec := e2e.Do(t, f.H, http.MethodPost, "/apikey/revoke", f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID, "id": id,
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "revoke failed: %s", rec.Body.String())
	e2e.RequireEvent(t, f.Conn, "iam", "api_key.revoked")
}

func TestAudit_APIKeySetRoles(t *testing.T) {
	f := e2e.Setup(t)
	id := createAPIKey(t, f)
	rec := e2e.Do(t, f.H, http.MethodPost, "/apikey/set-roles", f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID, "id": id, "role_ids": []string{f.Actor.RoleID},
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "set-roles failed: %s", rec.Body.String())
	e2e.RequireEvent(t, f.Conn, "iam", "api_key.set_roles")
}
