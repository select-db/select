package cellar_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"backend/e2e"
	"backend/internal/datasource/cellar"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// lifecycle calls the backend's routes for managed databases as the owner.
type lifecycle struct {
	m managedDB
}

func newLifecycle(t *testing.T) lifecycle {
	t.Helper()
	m := newManagedDB(t)
	cellar.ID = "local"
	t.Cleanup(func() { cellar.ID = "" })
	return lifecycle{m: m}
}

func (l lifecycle) call(t *testing.T, method, path string, body map[string]any) (int, []byte) {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["workspace_id"] = l.m.f.Actor.WorkspaceID
	rec := e2e.Do(t, l.m.f.H, method, path, l.m.f.Actor.Token, body)
	return rec.Code, rec.Body.Bytes()
}

// create makes a managed database granted to the owner and returns its id.
func (l lifecycle) create(t *testing.T, path string, body map[string]any) string {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["grant_to"] = map[string]any{"users": []string{l.m.f.Actor.UserID}}
	code, out := l.call(t, http.MethodPost, path, body)
	require.Equal(t, http.StatusCreated, code, string(out))
	var created struct {
		ID     string `json:"id"`
		Config struct {
			ID        string `json:"id"`
			DBType    string `json:"db_type"`
			Proxified bool   `json:"proxified"`
		} `json:"config"`
	}
	require.NoError(t, json.Unmarshal(out, &created))
	require.Equal(t, created.ID, created.Config.ID)
	require.Equal(t, "sqlite", created.Config.DBType)
	require.True(t, created.Config.Proxified)
	return created.ID
}

func (l lifecycle) on(id string) managedDB {
	m := l.m
	m.id = id
	return m
}

func TestManagedLifecycle(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t, "/datasources", map[string]any{"db_type": "sqlite", "name": "notes"})

	db := l.on(id)
	_, failure := db.run(t, "CREATE TABLE note (body TEXT)")
	require.Nil(t, failure, "the dedicated role gives full access")
	_, failure = db.run(t, "INSERT INTO note VALUES ('kept')")
	require.Nil(t, failure)
	e2e.RequireEvent(t, l.m.f.Conn, "datasource", "lifecycle.create")

	fork := l.create(t, "/datasources/"+id+"/fork", nil)
	rows, failure := l.on(fork).run(t, "SELECT body FROM note")
	require.Nil(t, failure)
	require.Equal(t, [][]any{{"kept"}}, rows, "a fork holds the source's data")
	var name string
	require.NoError(t, l.m.f.Conn.QueryRow(`SELECT name FROM app.datasource WHERE id = $1`, fork).Scan(&name))
	require.Equal(t, "notes (fork)", name)

	code, out := l.call(t, http.MethodPost, "/datasources/"+id+"/fork", map[string]any{"at": time.Now().Add(-time.Hour).Format(time.RFC3339)})
	require.Equal(t, http.StatusNotImplemented, code, "a point-in-time fork waits for the bucket: %s", out)
	code, _ = l.call(t, http.MethodPost, "/datasources/"+id+"/fork", map[string]any{"at": time.Now().AddDate(0, 0, -30).Format(time.RFC3339)})
	require.Equal(t, http.StatusBadRequest, code, "outside the plan's window")

	code, out = l.call(t, http.MethodGet, "/datasources/"+id+"/download", nil)
	require.Equal(t, http.StatusOK, code, string(out))
	require.True(t, strings.HasPrefix(string(out), "SQLite format 3"))

	code, out = l.call(t, http.MethodPut, "/datasources/"+id, map[string]any{"name": "renamed", "db_type": "postgresql", "dsn": "postgres://elsewhere/db"})
	require.Equal(t, http.StatusNoContent, code, string(out))
	var dbType string
	require.NoError(t, l.m.f.Conn.QueryRow(`SELECT name, db_type FROM app.datasource WHERE id = $1`, id).Scan(&name, &dbType))
	require.Equal(t, []string{"renamed", "sqlite"}, []string{name, dbType}, "a managed database only takes a new name")

	code, out = l.call(t, http.MethodDelete, "/datasources/"+id, nil)
	require.Equal(t, http.StatusNoContent, code, string(out))
	var state string
	require.NoError(t, l.m.f.Conn.QueryRow(`SELECT state FROM app.datasource WHERE id = $1`, id).Scan(&state))
	require.Equal(t, "deleting", state, "the reconciler purges the file; the row only stops serving")
	rec := e2e.Do(t, l.m.f.H, http.MethodPost, "/datasources/"+id+"/execute", l.m.f.Actor.Token,
		map[string]any{"workspace_id": l.m.f.Actor.WorkspaceID, "sql": "SELECT 1"})
	require.Equal(t, http.StatusNotFound, rec.Code, "a deleted database stops serving at once")
	var liveRoles int
	require.NoError(t, l.m.f.Conn.QueryRow(`SELECT count(*) FROM app.role r WHERE r.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM app.permission p WHERE p.role_id = r.id AND p.datasource_id = $1)`, id).Scan(&liveRoles))
	require.Zero(t, liveRoles, "the database's dedicated role goes with it")
	_, failure = l.on(fork).run(t, "SELECT body FROM note")
	require.Nil(t, failure, "deleting the source leaves its fork")
}

func TestManagedQuota(t *testing.T) {
	l := newLifecycle(t)
	for range 9 {
		_, err := l.m.f.Conn.Exec(`INSERT INTO app.datasource (id, workspace_id, name, db_type, cellar_id, state)
			VALUES ($1::uuid, $2::uuid, 'other', 'sqlite', 'local', 'hot')`, uuid.NewString(), l.m.f.Actor.WorkspaceID)
		require.NoError(t, err)
	}
	code, out := l.call(t, http.MethodPost, "/datasources", map[string]any{"db_type": "sqlite", "name": "eleventh"})
	require.Equal(t, http.StatusForbidden, code, string(out))
	require.Contains(t, string(out), "10 managed databases")
}

func TestManagedRoutesWithoutCellar(t *testing.T) {
	l := newLifecycle(t)
	cellar.URL = ""
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/datasources"},
		{http.MethodPost, "/datasources/" + l.m.id + "/fork"},
		{http.MethodGet, "/datasources/" + l.m.id + "/download"},
		{http.MethodDelete, "/datasources/" + l.m.id},
	} {
		code, out := l.call(t, c.method, c.path, map[string]any{"db_type": "sqlite", "name": "off"})
		require.Equalf(t, http.StatusNotImplemented, code, "%s %s: %s", c.method, c.path, out)
		require.Contains(t, string(out), "not enabled")
	}
}

func TestManagedGrantToOthersNeedsTheRight(t *testing.T) {
	l := newLifecycle(t)
	f := l.m.f
	member := uuid.NewString()
	e2e.SeedUser(t, f.Conn, member)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, member)
	creator := uuid.NewString()
	e2e.SeedUser(t, f.Conn, creator)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, creator)
	role := e2e.SeedRoleWithPermission(t, f.Conn, f.Actor.WorkspaceID, "creator", "manage")
	e2e.SeedUserRole(t, f.Conn, creator, role, f.Actor.WorkspaceID)
	token := e2e.MintJWT(t, creator)

	body := func(users ...string) map[string]any {
		return map[string]any{"workspace_id": f.Actor.WorkspaceID, "db_type": "sqlite", "name": "mine",
			"grant_to": map[string]any{"users": users}}
	}
	rec := e2e.Do(t, f.H, http.MethodPost, "/datasources", token, body(member))
	require.Equal(t, http.StatusForbidden, rec.Code, "granting another member needs users.manage: %s", rec.Body.String())
	rec = e2e.Do(t, f.H, http.MethodPost, "/datasources", token, body(creator))
	require.Equal(t, http.StatusCreated, rec.Code, "the caller can always grant itself: %s", rec.Body.String())
	rec = e2e.Do(t, f.H, http.MethodPost, "/datasources", token, body())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	var held bool
	require.NoError(t, f.Conn.QueryRow(`SELECT EXISTS (SELECT 1 FROM app.user_to_role ur
		JOIN app.permission p ON p.role_id = ur.role_id WHERE ur.user_id = $1 AND p.datasource_id = $2)`, creator, created.ID).Scan(&held))
	require.True(t, held, "the creator is granted the database without naming itself")
	rec = e2e.Do(t, f.H, http.MethodPost, "/datasources", f.Actor.Token, body(uuid.NewString()))
	require.Equal(t, http.StatusBadRequest, rec.Code, "a user outside the workspace: %s", rec.Body.String())
}

// An agent's key creates and forks through MCP, and can use what it made
// without anyone granting it.
func TestManagedThroughMCP(t *testing.T) {
	l := newLifecycle(t)
	f := l.m.f
	role := e2e.SeedRoleWithPermission(t, f.Conn, f.Actor.WorkspaceID, "agent", "manage")
	rec := e2e.CreateAPIKey(t, f.H, f.Actor.Token, f.Actor.WorkspaceID, role, "agent")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var key struct {
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &key))

	call := func(tool string, args map[string]any) map[string]any {
		t.Helper()
		rec := e2e.Do(t, f.H, http.MethodPost, "/mcp", key.Key, map[string]any{"jsonrpc": "2.0", "id": 1,
			"method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var resp struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.False(t, resp.Result.IsError, "%s: %s", tool, rec.Body.String())
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(resp.Result.Content[0].Text), &out))
		require.Nil(t, out["error"], "%s: %s", tool, resp.Result.Content[0].Text)
		return out
	}

	id, _ := call("create_datasource", map[string]any{"name": "scratch"})["id"].(string)
	require.NotEmpty(t, id)
	call("execute_statement", map[string]any{"datasource_id": id, "statement": "CREATE TABLE t (x INTEGER)"})
	fork, _ := call("fork_datasource", map[string]any{"datasource_id": id})["id"].(string)
	require.NotEmpty(t, fork)
	call("execute_statement", map[string]any{"datasource_id": fork, "statement": "INSERT INTO t VALUES (1)"})
}
