package datasource_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"backend/e2e"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newManagedFixture is a workspace whose managed databases run on a test cellar.
func newManagedFixture(t *testing.T) e2e.Fixture {
	t.Helper()
	fixture := e2e.Setup(t)
	e2e.ServeCellar(t)
	return fixture
}

// callAsOwner sends a request to the backend as the workspace owner.
func callAsOwner(t *testing.T, fixture e2e.Fixture, method, path string, body map[string]any) (int, []byte) {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["workspace_id"] = fixture.Actor.WorkspaceID
	rec := e2e.Do(t, fixture.H, method, path, fixture.Actor.Token, body)
	return rec.Code, rec.Body.Bytes()
}

// createAsOwner posts to a create or fork route, granting the owner the new
// managed database, and returns its id.
func createAsOwner(t *testing.T, fixture e2e.Fixture, path string, body map[string]any) string {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["grant_to"] = map[string]any{"users": []string{fixture.Actor.UserID}}
	status, responseBody := callAsOwner(t, fixture, http.MethodPost, path, body)
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
	return created.ID
}

// createNotes is a managed database holding a note table with one row, kept.
func createNotes(t *testing.T, fixture e2e.Fixture) string {
	t.Helper()
	id := createAsOwner(t, fixture, "/datasources", map[string]any{"db_type": "sqlite", "name": "notes"})
	_, failure := e2e.Execute(t, fixture, id, "CREATE TABLE note (body TEXT); INSERT INTO note VALUES ('kept')")
	require.Nil(t, failure)
	return id
}

// requireDisabled checks a managed route answers 501 while CELLAR is unset.
func requireDisabled(t *testing.T, fixture e2e.Fixture, method, path string) {
	t.Helper()
	status, responseBody := callAsOwner(t, fixture, method, path, map[string]any{"db_type": "sqlite", "name": "off"})
	require.Equalf(t, http.StatusNotImplemented, status, "%s %s: %s", method, path, responseBody)
	require.Contains(t, string(responseBody), "not enabled")
}

// memberToken mints a token for a new workspace member whose one role allows
// action on datasourceID only.
func memberToken(t *testing.T, fixture e2e.Fixture, datasourceID, action string) string {
	t.Helper()
	workspaceID, memberID, roleID := fixture.Actor.WorkspaceID, uuid.NewString(), uuid.NewString()
	e2e.SeedUser(t, fixture.Conn, memberID)
	e2e.SeedMembership(t, fixture.Conn, workspaceID, memberID)
	e2e.SeedRole(t, fixture.Conn, roleID, workspaceID, action+" on one database")
	_, err := fixture.Conn.Exec(`INSERT INTO app.permission (id, role_id, workspace_id, datasource_id, action, effect)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, 'allow')`, uuid.NewString(), roleID, workspaceID, datasourceID, action)
	require.NoError(t, err)
	e2e.SeedUserRole(t, fixture.Conn, memberID, roleID, workspaceID)
	return e2e.MintJWT(t, memberID)
}
