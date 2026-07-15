package datasource_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/e2e"
)

// Audit coverage for the datasource management handlers: each operation must emit
// the audit event its catalog spec declares (see backend/internal/audit/catalog.go).

func TestMain(m *testing.M) { e2e.Run(m) }

// upsertDatasource creates a datasource as the owner and returns its id.
func upsertDatasource(t *testing.T, f e2e.Fixture) string {
	t.Helper()
	id := uuid.NewString()
	rec := e2e.Do(t, f.H, http.MethodPost, "/datasource/upsert", f.Actor.Token, map[string]any{
		"id":           id,
		"workspace_id": f.Actor.WorkspaceID,
		"db_type":      "postgresql",
		"name":         "prod",
		"dsn":          "postgres://u:p@db:5432/app",
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "upsert failed: %s", rec.Body.String())
	return id
}

func TestAudit_DatasourceUpserted(t *testing.T) {
	f := e2e.Setup(t)
	upsertDatasource(t, f)
	e2e.RequireEvent(t, f.Conn, "datasource", "upserted")
}

func TestAudit_DatasourceDeleted(t *testing.T) {
	f := e2e.Setup(t)
	id := upsertDatasource(t, f)

	rec := e2e.Do(t, f.H, http.MethodPost, "/datasource/delete", f.Actor.Token, map[string]any{
		"id":           id,
		"workspace_id": f.Actor.WorkspaceID,
	})
	require.Equalf(t, http.StatusNoContent, rec.Code, "delete failed: %s", rec.Body.String())

	e2e.RequireEvent(t, f.Conn, "datasource", "deleted")
}

// nonManagerToken mints a token for a workspace member with no owner rights and
// no manage permission, so datasource management is forbidden for them.
func nonManagerToken(t *testing.T, f e2e.Fixture) string {
	t.Helper()
	memberID := uuid.NewString()
	e2e.SeedUser(t, f.Conn, memberID)
	e2e.SeedMembership(t, f.Conn, f.Actor.WorkspaceID, memberID)
	return e2e.MintJWT(t, memberID)
}

func TestAudit_DatasourceUpsertDenied(t *testing.T) {
	f := e2e.Setup(t)
	token := nonManagerToken(t, f)

	rec := e2e.Do(t, f.H, http.MethodPost, "/datasource/upsert", token, map[string]any{
		"id":           uuid.NewString(),
		"workspace_id": f.Actor.WorkspaceID,
		"db_type":      "postgresql",
		"name":         "prod",
		"dsn":          "postgres://u:p@db:5432/app",
	})
	require.Equalf(t, http.StatusForbidden, rec.Code, "expected 403, got %d: %s", rec.Code, rec.Body.String())

	e2e.RequireEventStatus(t, f.Conn, "datasource", "upserted", "denied")
}

func TestAudit_DatasourceDeleteDenied(t *testing.T) {
	f := e2e.Setup(t)
	token := nonManagerToken(t, f)

	rec := e2e.Do(t, f.H, http.MethodPost, "/datasource/delete", token, map[string]any{
		"id":           uuid.NewString(),
		"workspace_id": f.Actor.WorkspaceID,
	})
	require.Equalf(t, http.StatusForbidden, rec.Code, "expected 403, got %d: %s", rec.Code, rec.Body.String())

	e2e.RequireEventStatus(t, f.Conn, "datasource", "deleted", "denied")
}
