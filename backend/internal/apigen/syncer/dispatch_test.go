package syncer

import (
	"backend/internal/apigen/schema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EmitSyncDispatch lists only @app.sync tables, sorted, one registry entry each,
// and skips non-synced tables. It must also produce valid, formatted Go.
func TestEmitSyncDispatch(t *testing.T) {
	ents := []schema.Entity{
		{Table: "role", Sync: true},
		{Table: "workspace"}, // not synced: hand-written special, excluded
		{Table: "group", Sync: true},
	}
	f, err := EmitSyncDispatch(ents)
	if err != nil {
		t.Fatalf("emit (also runs format.Source): %v", err)
	}
	if !strings.Contains(f.Content, `"group":`) || !strings.Contains(f.Content, `"role":`) {
		t.Fatalf("missing synced table entry:\n%s", f.Content)
	}
	if strings.Contains(f.Content, `"workspace":`) {
		t.Fatalf("non-synced workspace must not appear in the generated registry:\n%s", f.Content)
	}
	// group sorts before role.
	if strings.Index(f.Content, `"group":`) > strings.Index(f.Content, `"role":`) {
		t.Fatalf("entries not sorted:\n%s", f.Content)
	}
}

// Pin the registry to the committed dispatch.go so drift is caught.
func TestEmitSyncDispatchMatchesCommitted(t *testing.T) {
	tables := []string{
		"role", "user_to_role", "permission", "group",
		"user_to_group", "group_to_role", "workspace_to_user",
	}
	var ents []schema.Entity
	for _, tbl := range tables {
		ents = append(ents, schema.Entity{Table: tbl, Sync: true})
	}
	f, err := EmitSyncDispatch(ents)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "syncer", "gen", f.Name))
	if err != nil {
		t.Fatalf("read generated %s: %v", f.Name, err)
	}
	if f.Content != string(want) {
		t.Fatalf("%s drifted from committed generated file", f.Name)
	}
}
