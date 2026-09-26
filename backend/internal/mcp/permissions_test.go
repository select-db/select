package mcp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
	"backend/internal/auth"
	"backend/internal/mcp"
	"backend/internal/middlewares"
)

// A rule scoped to one datasource applies to MCP queries on it, as it does to
// the app's.
func TestMCPAppliesDatasourceScopedRules(t *testing.T) {
	f := e2e.Setup(t)
	datasourceID := uuid.NewString()
	// Before any request, which would cache the role's rules without these.
	for _, action := range []string{"select", "see"} {
		_, err := f.Conn.Exec(`INSERT INTO app.permission (role_id, workspace_id, db_instance_id, action, effect)
			VALUES ($1::uuid, $2::uuid, $3, $4, 'allow')`, f.Actor.RoleID, f.Actor.WorkspaceID, datasourceID, action)
		require.NoError(t, err)
	}

	rec := e2e.Do(t, f.H, http.MethodPut, "/datasources/"+datasourceID, f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"db_type":      "postgresql",
		"name":         "self",
		"dsn":          e2e.TargetDSN(t, f.Conn),
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "upsert failed: %s", rec.Body.String())

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_query","arguments":{"datasource_id":"` +
		datasourceID + `","statement":"SELECT name FROM app.workspace"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	roles := []auth.RoleRef{{ID: f.Actor.RoleID, Name: "Owner Role"}}
	ctx := middlewares.ContextWithAPIKeyPrincipal(req.Context(), uuid.NewString(), "mcp-key", f.Actor.WorkspaceID, roles)
	w := httptest.NewRecorder()
	mcp.Handler()(w, req.WithContext(ctx))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Falsef(t, resp.Result.IsError, "query refused: %s", w.Body.String())
	e2e.RequireEventStatus(t, f.Conn, "query", "executed", "success")
}
