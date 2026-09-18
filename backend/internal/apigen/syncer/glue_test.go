package syncer

import (
	"backend/internal/apigen/schema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EmitSyncGlue produces valid, formatted Go (format.Source succeeds) and wires
// the uniform workspace-scoped by-id (the Params form). Byte-level pinning
// happens via the committed role/*_gen.go once regenerated locally; here we
// assert the shape offline, independent of the committed files.
func TestEmitSyncGlueRole(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{schema.RoleTable()}})
	if len(errs) != 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}
	files, err := EmitSyncGlue(ents[0], testScoped)
	if err != nil {
		t.Fatalf("emit (also runs format.Source): %v", err)
	}

	byName := map[string]string{}
	for _, f := range files {
		byName[f.Name] = f.Content
	}
	const scopedFetch = "generated.GetRoleByIDParams{ID: idUUID, WorkspaceID: workspaceUUID}"
	for _, name := range []string{"apply.go", "fetch_current.go"} {
		if !strings.Contains(byName[name], scopedFetch) {
			t.Fatalf("%s does not use the workspace-scoped by-id fetch:\n%s", name, byName[name])
		}
	}
}

// The sync wire struct types.RoleRow is generated too; pin it to the committed
// file.
func TestEmitSyncRowTypeMatchesRole(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{schema.RoleTable()}})
	if len(errs) != 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}
	f, err := EmitSyncRowType(ents[0])
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "syncer", "types", f.Name))
	if err != nil {
		t.Fatalf("read generated %s: %v", f.Name, err)
	}
	if f.Content != string(want) {
		t.Fatalf("%s drifted from committed generated file", f.Name)
	}
}
