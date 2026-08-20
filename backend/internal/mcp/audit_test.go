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
	"backend/internal/mcp"
	"backend/internal/middlewares"
)

func TestMain(m *testing.M) { e2e.Run(m) }

// A query run through the MCP execute_query tool lands in the data-plane log as
// query.executed, tagged channel=mcp and attributed to the calling API key.
func TestAudit_MCPQueryExecuted(t *testing.T) {
	f := e2e.Setup(t)

	// A datasource the engine can actually connect to: this test's own database.
	dsID := uuid.NewString()
	rec := e2e.Do(t, f.H, http.MethodPut, "/datasources/"+dsID, f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"db_type":      "postgresql",
		"name":         "self",
		"dsn":          e2e.TargetDSN(t, f.Conn),
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "upsert failed: %s", rec.Body.String())

	// Drive the real MCP tool as an API-key principal in the datasource's
	// workspace (MCP requires an API key; Authenticated() would overwrite an
	// injected context, so call the handler directly like the protocol tests).
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_query","arguments":{"datasource_id":"` +
		dsID + `","statement":"SELECT 1 AS n"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middlewares.ContextWithAPIKeyPrincipal(req.Context(), uuid.NewString(), "mcp-key", f.Actor.WorkspaceID, nil)
	w := httptest.NewRecorder()
	mcp.Handler()(w, req.WithContext(ctx))
	require.Equalf(t, http.StatusOK, w.Code, "tools/call failed: %s", w.Body.String())

	// jsonrpc envelope is 200 even for a tool-level error; assert the query ran.
	var resp struct {
		Error *json.RawMessage `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Nil(t, resp.Error, "jsonrpc error: %s", w.Body.String())

	e2e.RequireEventStatus(t, f.Conn, "query", "executed", "success")

	var channel string
	require.NoError(t, f.Conn.QueryRow(
		`SELECT payload->>'channel'
		   FROM audit.event
		  WHERE domain = 'query' AND action = 'executed'
		  ORDER BY occurred_at DESC LIMIT 1`,
	).Scan(&channel))
	require.Equal(t, "mcp", channel)
}

// A query the permission engine blocks is not a separate event nor a system
// fault: it rides query.executed with status=denied (MCP is default-deny, so a
// key with no grants querying a real table is refused).
func TestAudit_MCPQueryDenied(t *testing.T) {
	f := e2e.Setup(t)

	dsID := uuid.NewString()
	rec := e2e.Do(t, f.H, http.MethodPut, "/datasources/"+dsID, f.Actor.Token, map[string]any{
		"workspace_id": f.Actor.WorkspaceID,
		"db_type":      "postgresql",
		"name":         "self",
		"dsn":          e2e.TargetDSN(t, f.Conn),
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "upsert failed: %s", rec.Body.String())

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_query","arguments":{"datasource_id":"` +
		dsID + `","statement":"SELECT * FROM app.workspace"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middlewares.ContextWithAPIKeyPrincipal(req.Context(), uuid.NewString(), "mcp-key", f.Actor.WorkspaceID, nil)
	w := httptest.NewRecorder()
	mcp.Handler()(w, req.WithContext(ctx))
	require.Equal(t, http.StatusOK, w.Code)

	e2e.RequireEventStatus(t, f.Conn, "query", "executed", "denied")
}
