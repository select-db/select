package syncer

import (
	"strings"
	"testing"

	"backend/internal/apigen/schema"
)

// scopeEntities is a minimal entity set exercising both a scoped FK target (role,
// group — carry workspace_id) and a global one (user — does not), so the emitter
// generates a guard only for the workspace-scoped targets.
func scopeEntities() ([]schema.Entity, map[string]bool) {
	fk := func(col, target string) schema.Field {
		return schema.Field{Name: col, Column: col, Kind: schema.KindUUID, FK: &schema.FKRef{Schema: "app", Table: target}}
	}
	ents := []schema.Entity{
		{Name: "permission", Table: "permission", Sync: true, Fields: []schema.Field{fk("role_id", "role")}},
		{Name: "user_to_group", Table: "user_to_group", Sync: true, Fields: []schema.Field{
			fk("user_id", "user"), fk("group_id", "group"),
		}},
		{Name: "group_to_role", Table: "group_to_role", Sync: true, Fields: []schema.Field{
			fk("group_id", "group"), fk("role_id", "role"), // role repeated → deduped
		}},
		// A read-only (unsynced) entity whose scoped FK must NOT produce a guard —
		// its writes never go through Apply, so nothing would call it.
		{Name: "log", Table: "event", Sync: false, Fields: []schema.Field{fk("snapshot_id", "principal_snapshot")}},
	}
	scoped := map[string]bool{"role": true, "group": true, "principal_snapshot": true} // user is global
	return ents, scoped
}

func TestEmitScope(t *testing.T) {
	ents, scoped := scopeEntities()
	f, err := EmitScope(ents, scoped)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if f.Name != "scope.go" {
		t.Fatalf("name: got %q", f.Name)
	}
	c := f.Content

	// A guard per distinct workspace-scoped target, using the workspace-scoped
	// sqlc by-id query so an out-of-workspace row reads as ErrNoRows.
	for _, want := range []string{
		"package scope",
		"func GroupInWorkspace(ctx context.Context, groupID, workspaceID db_types.JSONNullUUID) (bool, error)",
		"func RoleInWorkspace(ctx context.Context, roleID, workspaceID db_types.JSONNullUUID) (bool, error)",
		"db.Queries.GetRoleByID(ctx, generated.GetRoleByIDParams{ID: roleID, WorkspaceID: workspaceID})",
		"errors.Is(err, sql.ErrNoRows)",
	} {
		if !strings.Contains(c, want) {
			t.Fatalf("scope.go missing:\n%s\n---\n%s", want, c)
		}
	}
	// The global target (user, no workspace_id) gets no guard.
	if strings.Contains(c, "UserInWorkspace") {
		t.Fatalf("global FK target must not be guarded:\n%s", c)
	}
	// A scoped FK target referenced only by a read-only (unsynced) entity gets no
	// guard — nothing writes it through Apply, and it may lack a by-id query.
	if strings.Contains(c, "PrincipalSnapshotInWorkspace") {
		t.Fatalf("FK target of an unsynced entity must not be guarded:\n%s", c)
	}
	// Deduped: role appears in two entities but is emitted once.
	if n := strings.Count(c, "func RoleInWorkspace"); n != 1 {
		t.Fatalf("RoleInWorkspace emitted %d times, want 1", n)
	}
}
