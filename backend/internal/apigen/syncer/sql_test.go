package syncer

import (
	"backend/internal/apigen/schema"
	"strings"
	"testing"
)

// The emitter's uniform output for role. By-id is workspace-scoped (id AND
// workspace_id); the rest is unchanged. Committed role/sql/* are regenerated
// from this by `go run ./cmd/apigen generate`.
func TestEmitCRUDRole(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{schema.RoleTable()}})
	if len(errs) != 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}
	want := map[string]string{
		"get_role_by_id_query.sql": "" +
			"-- name: GetRoleByID :one\n" +
			"SELECT id, workspace_id, name, updated_at, deleted_at\n" +
			"FROM app.role\n" +
			"WHERE id = $1 AND workspace_id = $2;\n",
		"get_roles_for_user_since_query.sql": "" +
			"-- name: GetRolesForUserSince :many\n" +
			"SELECT r.id, r.workspace_id, r.name, r.updated_at, r.deleted_at\n" +
			"FROM app.role r\n" +
			"INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = r.workspace_id AND wtu.user_id = $1\n" +
			"WHERE r.updated_at > $2;\n",
		"set_role_deleted_at_statement.sql": "" +
			"-- name: SetRoleDeletedAt :exec\n" +
			"UPDATE app.role\n" +
			"SET deleted_at = now(), updated_at = now()\n" +
			"WHERE id = $1 AND workspace_id = $2;\n",
		"upsert_role_statement.sql": "" +
			"-- name: UpsertRole :exec\n" +
			"INSERT INTO app.role (id, workspace_id, name, updated_at)\n" +
			"VALUES ($1, $2, $3, now())\n" +
			"ON CONFLICT (id) DO UPDATE SET\n" +
			"  name = EXCLUDED.name,\n" +
			"  updated_at = now(),\n" +
			"  deleted_at = NULL;\n",
	}
	for _, f := range EmitSyncSQL(ents[0]) {
		exp, ok := want[f.Name]
		if !ok {
			t.Fatalf("unexpected file %s", f.Name)
		}
		if strings.TrimSpace(f.Content) != strings.TrimSpace(exp) {
			t.Fatalf("%s mismatch:\n--- got ---\n%s\n--- want ---\n%s", f.Name, f.Content, exp)
		}
	}
}
