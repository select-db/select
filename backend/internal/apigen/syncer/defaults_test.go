package syncer

import (
	"backend/internal/apigen/schema"
	"strings"
	"testing"
)

// A NOT NULL column with a literal DB default must coalesce to that default in
// the merge (PatchStrDefault) so a client-omitted value never writes NULL. A
// nullable column keeps PatchNullStr; a NOT NULL column without a default keeps
// PatchStr.
func TestEmitGlueCoalescesNotNullDefault(t *testing.T) {
	tbl := schema.RawTable{
		Schema: "app", Name: "group", Comment: "@app.sync",
		Columns: []schema.RawColumn{
			{Name: "id", DataType: "uuid", NotNull: true},
			{Name: "workspace_id", DataType: "uuid", NotNull: true},
			{Name: "name", DataType: "text", NotNull: true},                             // NOT NULL, no default
			{Name: "source", DataType: "text", NotNull: true, Default: "'local'::text"}, // NOT NULL + default
			{Name: "note", DataType: "text"},                                            // nullable
			{Name: "updated_at", DataType: "timestamptz", NotNull: true},
			{Name: "deleted_at", DataType: "timestamptz"},
		},
		PrimaryKey:  []string{"id"},
		ForeignKeys: []schema.RawFK{{Column: "workspace_id", RefSchema: "app", RefTable: "workspace", RefColumn: "id"}},
	}
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{tbl}})
	if len(errs) != 0 {
		t.Fatalf("lint: %v", errs)
	}
	files, err := EmitSyncGlue(ents[0], testScoped)
	if err != nil {
		t.Fatal(err)
	}
	var apply string
	for _, f := range files {
		if f.Name == "apply.go" {
			apply = f.Content
		}
	}
	if !strings.Contains(apply, `utils.PatchStrDefault(payload, "source", existing.Source, "local")`) {
		t.Errorf("source should coalesce to its default:\n%s", apply)
	}
	if !strings.Contains(apply, `utils.PatchStr(payload, "name", existing.Name)`) {
		t.Errorf("name (NOT NULL, no default) should use PatchStr")
	}
	if !strings.Contains(apply, `utils.PatchNullStr(payload, "note", existing.Note)`) {
		t.Errorf("note (nullable) should use PatchNullStr")
	}
}
