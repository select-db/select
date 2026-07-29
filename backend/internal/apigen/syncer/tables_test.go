package syncer

import (
	"backend/internal/apigen/schema"
	"strings"
	"testing"
)

// testScoped is the set of workspace-scoped FK-target tables the emitter guards
// (role and group carry workspace_id); the single-table test builds don't
// include those targets, so the guard set is supplied explicitly here.
var testScoped = map[string]bool{"role": true, "group": true}

func glueByName(t *testing.T, tbl schema.RawTable) map[string]string {
	t.Helper()
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{tbl}})
	if len(errs) != 0 {
		t.Fatalf("lint: %v", errs)
	}
	files, err := EmitSyncGlue(ents[0], testScoped) // also runs format.Source (syntax check)
	if err != nil {
		t.Fatalf("emit glue: %v", err)
	}
	m := map[string]string{}
	for _, f := range files {
		m[f.Name] = f.Content
	}
	return m
}

func sqlByName(t *testing.T, tbl schema.RawTable) map[string]string {
	t.Helper()
	ents, _ := schema.Build(schema.RawSchema{Tables: []schema.RawTable{tbl}})
	m := map[string]string{}
	for _, f := range EmitSyncSQL(ents[0]) {
		m[f.Name] = f.Content
	}
	return m
}

func has(t *testing.T, hay, needle string) {
	t.Helper()
	if !strings.Contains(hay, needle) {
		t.Fatalf("missing %q in:\n%s", needle, hay)
	}
}
func hasNot(t *testing.T, hay, needle string) {
	t.Helper()
	if strings.Contains(hay, needle) {
		t.Fatalf("unexpected %q in:\n%s", needle, hay)
	}
}

// permission: FK role_id passed through (not updated); nullable text →
// PatchNullStr, NOT NULL text → PatchStr; every non-FK column is updatable.
func TestEmitPermissionUniform(t *testing.T) {
	apply := glueByName(t, schema.PermissionTable())["apply.go"]
	// The FK is parsed + scope-checked only when present, so a partial update that
	// omits role_id keeps the existing value instead of failing.
	has(t, apply, `if _, present := payload["role_id"]; present {`)
	has(t, apply, `roleUUID, err = db_types.NewJSONNullUUIDFromString(utils.MapGetString(payload, "role_id"))`)
	has(t, apply, `scope.RoleInWorkspace(ctx, roleUUID, workspaceUUID)`) // cross-workspace guard
	// FK-out-of-workspace yields a typed FieldError so the REST layer can surface a
	// precise, safe 422 (never revealing cross-workspace existence).
	has(t, apply, `&types.FieldError{Field: "role_id", Message: "does not reference a role in this workspace"}`)
	has(t, apply, `utils.PatchUUID(payload, "role_id", existing.RoleID, roleUUID)`)
	has(t, apply, `utils.PatchNullStr(payload, "db_instance_id", existing.DbInstanceID)`)
	has(t, apply, `utils.PatchStr(payload, "action", existing.Action)`)

	sql := sqlByName(t, schema.PermissionTable())
	has(t, sql["get_permission_by_id_query.sql"], "WHERE id = $1 AND workspace_id = $2;")
	up := sql["upsert_permission_statement.sql"]
	has(t, up, "INSERT INTO app.permission (id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect, updated_at)")
	has(t, up, "effect = EXCLUDED.effect,")
	hasNot(t, up, "role_id = EXCLUDED.role_id") // FK identity, never updated
}

// user_to_role: link table — both FKs passed through, no patchables, so the
// upsert updates nothing on conflict (just un-deletes). Uniform plural naming.
func TestEmitUserToRoleUniform(t *testing.T) {
	apply := glueByName(t, schema.UserToRoleTable())["apply.go"]
	has(t, apply, `scope.RoleInWorkspace(ctx, roleUUID, workspaceUUID)`) // role must be in-workspace
	has(t, apply, `utils.PatchUUID(payload, "user_id", existing.UserID, userUUID)`)
	has(t, apply, `utils.PatchUUID(payload, "role_id", existing.RoleID, roleUUID)`)
	hasNot(t, apply, "utils.PatchStr")
	hasNot(t, apply, "utils.PatchNullStr")

	sql := sqlByName(t, schema.UserToRoleTable())
	if _, ok := sql["get_user_to_roles_for_user_since_query.sql"]; !ok {
		t.Fatal("expected uniform plural file name get_user_to_roles_for_user_since_query.sql")
	}
	up := sql["upsert_user_to_role_statement.sql"]
	has(t, up, "INSERT INTO app.user_to_role (id, user_id, role_id, workspace_id, updated_at)")
	hasNot(t, up, "= EXCLUDED.") // nothing patchable
}
