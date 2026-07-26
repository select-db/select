package syncer

import (
	"backend/internal/apigen/schema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EmitSyncChanges renders one aggregation line and one filter line per
// @app.sync table, keyed by the pascal-cased struct field, and skips
// non-synced tables.
func TestEmitSyncChanges(t *testing.T) {
	ents := []schema.Entity{
		{Table: "role", Sync: true},
		{Table: "workspace"}, // special: not @app.sync, excluded
		{Table: "user_to_role", Sync: true},
	}
	f, err := EmitSyncChanges(ents)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"changes.Role, err = role.GetChangesSince(ctx, userID, since)",
		"changes.UserToRole, err = user_to_role.GetChangesSince(ctx, userID, since)",
		"changes.Role = FilterByID(changes.Role, applied, func(r types.RoleRow) string { return r.ID })",
	} {
		if !strings.Contains(f.Content, want) {
			t.Fatalf("missing %q in:\n%s", want, f.Content)
		}
	}
	if strings.Contains(f.Content, "workspace.GetChangesSince") || strings.Contains(f.Content, "changes.Workspaces") {
		t.Fatalf("workspace special must not appear in the generated aggregation:\n%s", f.Content)
	}
}

// Pin the aggregation to the committed changes.go so drift is caught.
func TestEmitSyncChangesMatchesCommitted(t *testing.T) {
	tables := []string{"role", "user_to_role", "permission", "group", "user_to_group", "group_to_role", "workspace_to_user"}
	var ents []schema.Entity
	for _, tbl := range tables {
		ents = append(ents, schema.Entity{Table: tbl, Sync: true})
	}
	f, err := EmitSyncChanges(ents)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "syncer", "gen", f.Name))
	if err != nil {
		t.Fatalf("read %s: %v", f.Name, err)
	}
	if f.Content != string(want) {
		t.Fatalf("%s drifted from committed generated file", f.Name)
	}
}
