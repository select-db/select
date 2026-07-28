package httpapi

import (
	"strings"
	"testing"

	"backend/internal/apigen/schema"
)

func roleEntity() schema.Entity {
	return schema.Entity{
		Name: "role", Schema: "app", Table: "role", Sync: true,
		PrimaryKey:  []string{"id"},
		DefaultSort: "name",
		API: []schema.APIOp{
			{Op: "list", Requires: []string{"roles.manage"}},
			{Op: "get", Requires: []string{"roles.manage"}},
			{Op: "create", Requires: []string{"roles.manage"}},
			{Op: "update", Requires: []string{"roles.manage"}},
			{Op: "delete", Requires: []string{"roles.manage"}},
		},
		Fields: []schema.Field{
			{Name: "id", Column: "id", Kind: schema.KindUUID, IsPK: true, Exposed: true},
			{Name: "name", Column: "name", Kind: schema.KindText, Patchable: true, Exposed: true},
			{Name: "updated_at", Column: "updated_at", Kind: schema.KindTime, Exposed: true},
			{Name: "workspace_id", Column: "workspace_id", Kind: schema.KindUUID, Exposed: false},
		},
	}
}

func logEntity() schema.Entity {
	return schema.Entity{
		Name: "log", Schema: "audit", Table: "event", Sync: false,
		PrimaryKey: []string{"id"},
		API: []schema.APIOp{
			{Op: "list", Requires: []string{"audit.read"}},
			{Op: "get", Requires: []string{"audit.read"}},
		},
		Fields: []schema.Field{
			{Name: "id", Column: "id", Kind: schema.KindUUID, IsPK: true, Exposed: true},
			{Name: "status", Column: "status", Kind: schema.KindText, Exposed: true, Values: []string{"success", "error", "denied"}},
			{Name: "principal_hash", Column: "principal_hash", Kind: schema.KindText, Hidden: true, Exposed: false},
		},
	}
}

func TestEmitRoutes(t *testing.T) {
	f, err := EmitRoutes([]schema.Entity{roleEntity(), logEntity()})
	if err != nil {
		t.Fatalf("emit (also runs format.Source): %v", err)
	}
	if f.Name != "routes.go" {
		t.Fatalf("file name: %q", f.Name)
	}
	// gofmt aligns struct/map values, so compare with whitespace runs collapsed.
	c := strings.Join(strings.Fields(f.Content), " ")

	for _, want := range []string{
		`package gen`,
		`core "github.com/selectDb/dialect/core"`,
		`role "backend/internal/syncer/gen/role"`,
		`func RegisterRoutes(mux *http.ServeMux, wrap func(perMinute int, h http.Handler) http.Handler)`,
		`rest.Register(mux, wrap, entities())`,
		`Singular: "role", Plural: "roles", Table: "role"`,
		`Table: "app.role", PK: "id", DefaultSort: "name"`,
		`{Name: "name", Column: "name", Kind: query.KindText, Ops: []query.Op{query.OpEq, query.OpNe, query.OpIn, query.OpNotIn, query.OpContains, query.OpStartsWith, query.OpEndsWith}}`,
		`"list": {core.ActionWorkspaceRolesManage}`,
		`Apply: role.Apply, ApplyDelete: role.ApplyDelete`,
		// read-only entity: audit/log, enum surfaced, no Apply
		`Singular: "log", Plural: "logs", Table: "event"`,
		`Table: "audit.event", PK: "id"`,
		`Enum: []string{"success", "error", "denied"}`,
		`"list": {core.ActionWorkspaceAuditRead}`,
	} {
		if !strings.Contains(c, want) {
			t.Fatalf("generated routes.go is missing:\n%s\n---\n%s", want, c)
		}
	}

	// The tenant column and hidden columns must never be exposed as fields.
	if strings.Contains(c, `Column: "workspace_id"`) {
		t.Fatal("tenant column leaked into the field spec")
	}
	if strings.Contains(c, `principal_hash`) {
		t.Fatal("@app.hide column leaked into the field spec")
	}
	// The read-only entity must not reference an Apply func.
	logBlock := c[strings.Index(c, `Singular: "log"`):]
	if strings.Contains(logBlock, "ApplyDelete") {
		t.Fatal("read-only entity should have no Apply/ApplyDelete")
	}
}
