package mcp_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"backend/e2e"

	"github.com/stretchr/testify/require"
)

// An agent's key creates and forks through MCP, and can use what it made
// without anyone granting it.
func TestCreateAndForkThroughMCP(t *testing.T) {
	fixture := e2e.Setup(t)
	e2e.ServeCellar(t)
	agentRole := e2e.SeedRoleWithPermission(t, fixture.Conn, fixture.Actor.WorkspaceID, "agent", "manage")
	rec := e2e.CreateAPIKey(t, fixture.H, fixture.Actor.Token, fixture.Actor.WorkspaceID, agentRole, "agent")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var apiKey struct {
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &apiKey))

	callTool := func(tool string, args map[string]any) map[string]any {
		t.Helper()
		rec := e2e.Do(t, fixture.H, http.MethodPost, "/mcp", apiKey.Key, map[string]any{"jsonrpc": "2.0", "id": 1,
			"method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var rpcResponse struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rpcResponse))
		require.False(t, rpcResponse.Result.IsError, "%s: %s", tool, rec.Body.String())
		toolResult := rpcResponse.Result.Content[0].Text
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(toolResult), &result))
		require.Nil(t, result["error"], "%s: %s", tool, toolResult)
		return result
	}

	scratchID, _ := callTool("create_datasource", map[string]any{"name": "scratch"})["id"].(string)
	require.NotEmpty(t, scratchID)
	callTool("execute_statement", map[string]any{"datasource_id": scratchID, "statement": "CREATE TABLE t (x INTEGER)"})
	forkID, _ := callTool("fork_datasource", map[string]any{"datasource_id": scratchID})["id"].(string)
	require.NotEmpty(t, forkID)
	callTool("execute_statement", map[string]any{"datasource_id": forkID, "statement": "INSERT INTO t VALUES (1)"})
}

// A key without manage on "*" cannot create, and one without manage on the
// source cannot fork it, as over REST.
func TestCreateAndForkThroughMCPNeedManage(t *testing.T) {
	fixture := e2e.Setup(t)
	e2e.ServeCellar(t)
	readerRole := e2e.SeedRoleWithPermission(t, fixture.Conn, fixture.Actor.WorkspaceID, "reader", "select")
	rec := e2e.CreateAPIKey(t, fixture.H, fixture.Actor.Token, fixture.Actor.WorkspaceID, readerRole, "reader")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var apiKey struct {
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &apiKey))
	rec = e2e.Do(t, fixture.H, http.MethodPost, "/datasources", fixture.Actor.Token,
		map[string]any{"workspace_id": fixture.Actor.WorkspaceID, "db_type": "sqlite", "name": "source"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var source struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &source))

	for tool, args := range map[string]map[string]any{
		"create_datasource": {"name": "scratch"},
		"fork_datasource":   {"datasource_id": source.ID},
	} {
		rec := e2e.Do(t, fixture.H, http.MethodPost, "/mcp", apiKey.Key, map[string]any{"jsonrpc": "2.0", "id": 1,
			"method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `\"code\":\"forbidden\"`, "%s: %s", tool, rec.Body.String())
	}
	e2e.RequireEventStatus(t, fixture.Conn, "datasource", "lifecycle.create", "denied")
}
