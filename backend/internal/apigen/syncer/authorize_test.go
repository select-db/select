package syncer

import (
	"backend/internal/apigen/schema"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syncTbl builds a minimal @app.sync table carrying the given table comment.
// authorize derivation only reads the @app.api tags, so the columns just need
// to satisfy the sync convention (pk + workspace + cursor + soft-delete).
func syncTbl(name, comment string) schema.RawTable {
	return schema.RawTable{
		Schema: "app", Name: name, Comment: comment,
		Columns: []schema.RawColumn{
			{Name: "id", DataType: "uuid", NotNull: true},
			{Name: "workspace_id", DataType: "uuid", NotNull: true},
			{Name: "updated_at", DataType: "timestamptz", NotNull: true},
			{Name: "deleted_at", DataType: "timestamptz"},
		},
		PrimaryKey:  []string{"id"},
		ForeignKeys: []schema.RawFK{{Column: "workspace_id", RefSchema: "app", RefTable: "workspace", RefColumn: "id"}},
	}
}

// EmitAuthorize derives each synced table's required-action AND-list from the
// @app.api `requires` clause, including the compound cases, and maps values to
// core.Action constants.
func TestEmitAuthorize(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{
		syncTbl("role", "@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage"),
		syncTbl("group_to_role", "@app.sync @app.api.create|update|delete requires groups.manage, roles.manage"),
	}})
	if len(errs) != 0 {
		t.Fatalf("lint: %v", errs)
	}
	f, err := EmitAuthorize(ents)
	if err != nil {
		t.Fatal(err)
	}
	// Compound AND-list preserves tag order and maps to the core constants.
	// (Key/value are asserted apart so gofmt's column alignment doesn't matter.)
	if !strings.Contains(f.Content, `"group_to_role":`) ||
		!strings.Contains(f.Content, `{core.ActionWorkspaceGroupsManage, core.ActionWorkspaceRolesManage},`) {
		t.Fatalf("group_to_role compound gate missing:\n%s", f.Content)
	}
	if !strings.Contains(f.Content, `"role":`) ||
		!strings.Contains(f.Content, `{core.ActionWorkspaceRolesManage},`) {
		t.Fatalf("role gate missing:\n%s", f.Content)
	}
}

// An unknown `requires` value must fail generation rather than silently emit a
// table with no gate — a dropped check is exactly the security regression this
// guards against.
func TestEmitAuthorizeUnknownActionFails(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{
		syncTbl("role", "@app.sync @app.api.create requires bogus.action"),
	}})
	if len(errs) != 0 {
		t.Fatalf("lint: %v", errs)
	}
	if _, err := EmitAuthorize(ents); err == nil {
		t.Fatal("expected error for unknown requires action, got nil")
	}
}

// Read-only tables (only list/get, no requires) get no entry — the shared
// preamble is their only gate.
func TestEmitAuthorizeReadOnlyOmitted(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{
		syncTbl("role", "@app.sync @app.api.list|get"),
	}})
	if len(errs) != 0 {
		t.Fatalf("lint: %v", errs)
	}
	f, err := EmitAuthorize(ents)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.Content, `"role":`) {
		t.Fatalf("read-only table should not be gated:\n%s", f.Content)
	}
}

// Pin the registry to the committed authorize.go so drift is caught.
func TestEmitAuthorizeMatchesCommitted(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{
		syncTbl("role", "@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage"),
		syncTbl("user_to_role", "@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage"),
		syncTbl("permission", "@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage"),
		syncTbl("group", "@app.sync @app.api.list|get @app.api.create|update|delete requires groups.manage"),
		syncTbl("user_to_group", "@app.sync @app.api.list|get @app.api.create|update|delete requires groups.manage"),
		syncTbl("group_to_role", "@app.sync @app.api.list|get @app.api.create|update|delete requires groups.manage, roles.manage"),
		syncTbl("workspace_to_user", "@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage, users.manage"),
	}})
	if len(errs) != 0 {
		t.Fatalf("lint: %v", errs)
	}
	f, err := EmitAuthorize(ents)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "syncer", "gen", f.Name))
	if err != nil {
		t.Fatalf("read %s: %v", f.Name, err)
	}
	if f.Content != string(want) {
		t.Fatalf("%s drifted from committed generated file:\n--- got ---\n%s", f.Name, f.Content)
	}
}
