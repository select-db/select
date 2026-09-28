package cellarclient_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"backend/e2e"
	"backend/internal/datasource/cellarclient"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// call sends a request to the backend as the workspace owner.
func (database managedDB) call(t *testing.T, method, path string, body map[string]any) (int, []byte) {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["workspace_id"] = database.fixture.Actor.WorkspaceID
	rec := e2e.Do(t, database.fixture.H, method, path, database.fixture.Actor.Token, body)
	return rec.Code, rec.Body.Bytes()
}

// create posts to a create or fork route, granting the owner the new database,
// and returns the new database.
func (database managedDB) create(t *testing.T, path string, body map[string]any) managedDB {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["grant_to"] = map[string]any{"users": []string{database.fixture.Actor.UserID}}
	status, responseBody := database.call(t, http.MethodPost, path, body)
	require.Equal(t, http.StatusCreated, status, string(responseBody))
	var created struct {
		ID     string `json:"id"`
		Config struct {
			ID        string `json:"id"`
			DBType    string `json:"db_type"`
			Proxified bool   `json:"proxified"`
		} `json:"config"`
	}
	require.NoError(t, json.Unmarshal(responseBody, &created))
	require.Equal(t, created.ID, created.Config.ID)
	require.Equal(t, "sqlite", created.Config.DBType)
	require.True(t, created.Config.Proxified)
	database.id = created.ID
	return database
}

func TestManagedLifecycle(t *testing.T) {
	seeded := newManagedDB(t)
	conn := seeded.fixture.Conn
	notes := seeded.create(t, "/datasources", map[string]any{"db_type": "sqlite", "name": "notes"})

	_, failure := notes.run(t, "CREATE TABLE note (body TEXT)")
	require.Nil(t, failure, "the dedicated role gives full access")
	_, failure = notes.run(t, "INSERT INTO note VALUES ('kept')")
	require.Nil(t, failure)
	e2e.RequireEvent(t, conn, "datasource", "lifecycle.create")

	fork := notes.create(t, "/datasources/"+notes.id+"/fork", nil)
	rows, failure := fork.run(t, "SELECT body FROM note")
	require.Nil(t, failure)
	require.Equal(t, [][]any{{"kept"}}, rows, "a fork holds the source's data")
	var name string
	require.NoError(t, conn.QueryRow(`SELECT name FROM app.datasource WHERE id = $1`, fork.id).Scan(&name))
	require.Equal(t, "notes (fork)", name)

	anHourAgo := time.Now().Add(-time.Hour).Format(time.RFC3339)
	status, responseBody := notes.call(t, http.MethodPost, "/datasources/"+notes.id+"/fork", map[string]any{"at": anHourAgo})
	require.Equal(t, http.StatusNotImplemented, status, "a point-in-time fork waits for the bucket: %s", responseBody)
	aMonthAgo := time.Now().AddDate(0, 0, -30).Format(time.RFC3339)
	status, _ = notes.call(t, http.MethodPost, "/datasources/"+notes.id+"/fork", map[string]any{"at": aMonthAgo})
	require.Equal(t, http.StatusBadRequest, status, "outside the plan's window")

	status, responseBody = notes.call(t, http.MethodGet, "/datasources/"+notes.id+"/download", nil)
	require.Equal(t, http.StatusOK, status, string(responseBody))
	require.True(t, strings.HasPrefix(string(responseBody), "SQLite format 3"))

	status, responseBody = notes.call(t, http.MethodPut, "/datasources/"+notes.id,
		map[string]any{"name": "renamed", "db_type": "postgresql", "dsn": "postgres://elsewhere/db"})
	require.Equal(t, http.StatusNoContent, status, string(responseBody))
	var dbType string
	require.NoError(t, conn.QueryRow(`SELECT name, db_type FROM app.datasource WHERE id = $1`, notes.id).Scan(&name, &dbType))
	require.Equal(t, []string{"renamed", "sqlite"}, []string{name, dbType}, "a managed database only takes a new name")

	status, responseBody = notes.call(t, http.MethodDelete, "/datasources/"+notes.id, nil)
	require.Equal(t, http.StatusNoContent, status, string(responseBody))
	var state string
	require.NoError(t, conn.QueryRow(`SELECT state FROM app.datasource WHERE id = $1`, notes.id).Scan(&state))
	require.Equal(t, "deleting", state, "the reconciler purges the file; the row only stops serving")
	status, _ = notes.call(t, http.MethodPost, "/datasources/"+notes.id+"/execute", map[string]any{"sql": "SELECT 1"})
	require.Equal(t, http.StatusNotFound, status, "a deleted database stops serving at once")
	var liveRoles int
	require.NoError(t, conn.QueryRow(`SELECT count(*) FROM app.role r WHERE r.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM app.permission p WHERE p.role_id = r.id AND p.datasource_id = $1)`, notes.id).Scan(&liveRoles))
	require.Zero(t, liveRoles, "the database's dedicated role goes with it")
	_, failure = fork.run(t, "SELECT body FROM note")
	require.Nil(t, failure, "deleting the source leaves its fork")
}

func TestManagedQuota(t *testing.T) {
	seeded := newManagedDB(t)
	// With the seeded one, the solo plan's 10 databases are taken.
	for range 9 {
		_, err := seeded.fixture.Conn.Exec(`INSERT INTO app.datasource (id, workspace_id, name, db_type, cellar_id, state)
			VALUES ($1::uuid, $2::uuid, 'other', 'sqlite', 'local', 'hot')`, uuid.NewString(), seeded.fixture.Actor.WorkspaceID)
		require.NoError(t, err)
	}
	status, responseBody := seeded.call(t, http.MethodPost, "/datasources", map[string]any{"db_type": "sqlite", "name": "eleventh"})
	require.Equal(t, http.StatusForbidden, status, string(responseBody))
	require.Contains(t, string(responseBody), "10 managed databases")
}

func TestManagedRoutesWithoutCellar(t *testing.T) {
	seeded := newManagedDB(t)
	cellarclient.URL = ""
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/datasources"},
		{http.MethodPost, "/datasources/" + seeded.id + "/fork"},
		{http.MethodGet, "/datasources/" + seeded.id + "/download"},
		{http.MethodDelete, "/datasources/" + seeded.id},
	} {
		status, responseBody := seeded.call(t, route.method, route.path, map[string]any{"db_type": "sqlite", "name": "off"})
		require.Equalf(t, http.StatusNotImplemented, status, "%s %s: %s", route.method, route.path, responseBody)
		require.Contains(t, string(responseBody), "not enabled")
	}
}

func TestManagedGrantToOthersNeedsTheRight(t *testing.T) {
	fixture := newManagedDB(t).fixture
	workspaceID := fixture.Actor.WorkspaceID
	member := uuid.NewString()
	e2e.SeedUser(t, fixture.Conn, member)
	e2e.SeedMembership(t, fixture.Conn, workspaceID, member)
	creator := uuid.NewString()
	e2e.SeedUser(t, fixture.Conn, creator)
	e2e.SeedMembership(t, fixture.Conn, workspaceID, creator)
	creatorRole := e2e.SeedRoleWithPermission(t, fixture.Conn, workspaceID, "creator", "manage")
	e2e.SeedUserRole(t, fixture.Conn, creator, creatorRole, workspaceID)
	creatorToken := e2e.MintJWT(t, creator)

	createGrantingTo := func(users ...string) map[string]any {
		return map[string]any{"workspace_id": workspaceID, "db_type": "sqlite", "name": "mine",
			"grant_to": map[string]any{"users": users}}
	}
	rec := e2e.Do(t, fixture.H, http.MethodPost, "/datasources", creatorToken, createGrantingTo(member))
	require.Equal(t, http.StatusForbidden, rec.Code, "granting another member needs users.manage: %s", rec.Body.String())
	rec = e2e.Do(t, fixture.H, http.MethodPost, "/datasources", creatorToken, createGrantingTo(creator))
	require.Equal(t, http.StatusCreated, rec.Code, "the caller can always grant itself: %s", rec.Body.String())
	rec = e2e.Do(t, fixture.H, http.MethodPost, "/datasources", creatorToken, map[string]any{"workspace_id": workspaceID,
		"db_type": "postgresql", "name": "remote", "dsn": e2e.TargetDSN(t, fixture.Conn)})
	require.Equal(t, http.StatusCreated, rec.Code, "managed or not, adding a datasource takes manage on *: %s", rec.Body.String())
	rec = e2e.Do(t, fixture.H, http.MethodPost, "/datasources", fixture.Actor.Token, createGrantingTo(uuid.NewString()))
	require.Equal(t, http.StatusBadRequest, rec.Code, "a user outside the workspace: %s", rec.Body.String())
}

// An agent's key creates and forks through MCP, and can use what it made
// without anyone granting it.
func TestManagedThroughMCP(t *testing.T) {
	fixture := newManagedDB(t).fixture
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
