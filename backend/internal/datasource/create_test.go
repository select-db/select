package datasource_test

import (
	"net/http"
	"testing"

	"backend/e2e"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateManaged(t *testing.T) {
	fixture := newManagedFixture(t)

	id := createAsOwner(t, fixture, "/datasources", map[string]any{"db_type": "sqlite", "name": "notes"})
	_, failure := e2e.Execute(t, fixture, id, "CREATE TABLE note (body TEXT)")
	require.Nil(t, failure, "the dedicated role gives full access")
	e2e.RequireEvent(t, fixture.Conn, "datasource", "lifecycle.create")
}

func TestCreateManagedNeedsAName(t *testing.T) {
	fixture := newManagedFixture(t)

	status, responseBody := callAsOwner(t, fixture, http.MethodPost, "/datasources", map[string]any{"db_type": "sqlite"})
	require.Equal(t, http.StatusBadRequest, status, string(responseBody))
}

func TestCreateManagedQuota(t *testing.T) {
	fixture := newManagedFixture(t)
	// The solo plan allows 10 managed databases.
	for range 10 {
		_, err := fixture.Conn.Exec(`INSERT INTO app.datasource (id, workspace_id, name, db_type, cellar_id, state)
			VALUES ($1::uuid, $2::uuid, 'other', 'sqlite', 'local', 'hot')`, uuid.NewString(), fixture.Actor.WorkspaceID)
		require.NoError(t, err)
	}

	status, responseBody := callAsOwner(t, fixture, http.MethodPost, "/datasources", map[string]any{"db_type": "sqlite", "name": "eleventh"})
	require.Equal(t, http.StatusForbidden, status, string(responseBody))
	require.Contains(t, string(responseBody), "10 managed databases")
}

func TestCreateManagedWithoutCellar(t *testing.T) {
	fixture := newManagedFixture(t)
	cellarclient.URL = ""

	requireDisabled(t, fixture, http.MethodPost, "/datasources")
}

func TestCreateGrantToOthersNeedsTheRight(t *testing.T) {
	fixture := newManagedFixture(t)
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
	rec = e2e.Do(t, fixture.H, http.MethodPost, "/datasources", fixture.Actor.Token, createGrantingTo(uuid.NewString()))
	require.Equal(t, http.StatusBadRequest, rec.Code, "a user outside the workspace: %s", rec.Body.String())
}

func TestCreateAnyDatasourceTakesManageOnAll(t *testing.T) {
	fixture := newManagedFixture(t)
	creator := uuid.NewString()
	e2e.SeedUser(t, fixture.Conn, creator)
	e2e.SeedMembership(t, fixture.Conn, fixture.Actor.WorkspaceID, creator)
	creatorRole := e2e.SeedRoleWithPermission(t, fixture.Conn, fixture.Actor.WorkspaceID, "creator", "manage")
	e2e.SeedUserRole(t, fixture.Conn, creator, creatorRole, fixture.Actor.WorkspaceID)

	rec := e2e.Do(t, fixture.H, http.MethodPost, "/datasources", e2e.MintJWT(t, creator), map[string]any{
		"workspace_id": fixture.Actor.WorkspaceID, "db_type": "postgresql", "name": "remote", "dsn": e2e.TargetDSN(t, fixture.Conn)})
	require.Equal(t, http.StatusCreated, rec.Code, "managed or not, adding a datasource takes manage on *: %s", rec.Body.String())
}

func TestCreateNeedsManageOnAll(t *testing.T) {
	fixture := newManagedFixture(t)
	notesID := createNotes(t, fixture)
	// Manage on one database is not the right to add more.
	token := memberToken(t, fixture, notesID, "manage")

	for _, body := range []map[string]any{
		{"db_type": "sqlite", "name": "managed"},
		{"db_type": "postgresql", "name": "remote", "dsn": e2e.TargetDSN(t, fixture.Conn)},
	} {
		body["workspace_id"] = fixture.Actor.WorkspaceID
		rec := e2e.Do(t, fixture.H, http.MethodPost, "/datasources", token, body)
		require.Equal(t, http.StatusForbidden, rec.Code, "%s: %s", body["db_type"], rec.Body.String())
	}
	e2e.RequireEventStatus(t, fixture.Conn, "datasource", "lifecycle.create", "denied")
	var count int
	require.NoError(t, fixture.Conn.QueryRow(`SELECT count(*) FROM app.datasource WHERE workspace_id = $1::uuid`, fixture.Actor.WorkspaceID).Scan(&count))
	require.Equal(t, 1, count, "only the notes database")
}
