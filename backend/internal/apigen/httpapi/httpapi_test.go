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

// filesByName indexes the emitted set by path-qualified name, and collapses
// each file's whitespace runs (gofmt aligns struct/map values, so single-space
// assertions would otherwise be brittle).
func filesByName(t *testing.T, ents ...schema.Entity) map[string]string {
	t.Helper()
	fs, err := EmitRoutes(ents)
	if err != nil {
		t.Fatalf("emit (also runs format.Source): %v", err)
	}
	out := make(map[string]string, len(fs))
	for _, f := range fs {
		out[f.Name] = strings.Join(strings.Fields(f.Content), " ")
	}
	return out
}

func mustFile(t *testing.T, files map[string]string, name string) string {
	t.Helper()
	c, ok := files[name]
	if !ok {
		var have []string
		for n := range files {
			have = append(have, n)
		}
		t.Fatalf("missing generated file %q; got %v", name, have)
	}
	return c
}

func TestEmitRoutesLayout(t *testing.T) {
	files := filesByName(t, roleEntity(), logEntity())

	// One folder per entity: resource.go, a file per op, and routes.go — plus the
	// top-level routes.go aggregating them. Mirrors the syncer's gen/<table>/ tree.
	for _, name := range []string{
		"role/resource.go", "role/list.go", "role/get.go",
		"role/create.go", "role/update.go", "role/delete.go", "role/routes.go",
		"event/resource.go", "event/list.go", "event/get.go", "event/routes.go",
		"routes.go",
	} {
		mustFile(t, files, name)
	}

	// A read-only entity emits no write endpoints.
	for _, name := range []string{"event/create.go", "event/update.go", "event/delete.go"} {
		if _, ok := files[name]; ok {
			t.Fatalf("read-only entity emitted a write endpoint file %q", name)
		}
	}
}

func TestEmitTopRoutes(t *testing.T) {
	c := mustFile(t, filesByName(t, roleEntity(), logEntity()), "routes.go")
	for _, want := range []string{
		`package gen`,
		`role "backend/internal/api/gen/role"`,
		`event "backend/internal/api/gen/event"`,
		`func RegisterRoutes(mux *http.ServeMux, wrap rest.Wrap)`,
		`role.Register(mux, wrap)`,
		`event.Register(mux, wrap)`,
	} {
		if !strings.Contains(c, want) {
			t.Fatalf("top routes.go is missing:\n%s\n---\n%s", want, c)
		}
	}
}

func TestEmitResource(t *testing.T) {
	files := filesByName(t, roleEntity(), logEntity())

	role := mustFile(t, files, "role/resource.go")
	for _, want := range []string{
		`package role`,
		`core "github.com/selectDb/dialect/core"`,
		`syncgen "backend/internal/syncer/gen/role"`,
		`var entity = rest.Entity{`,
		`Singular: "role", Plural: "roles", Table: "role"`,
		`Table: "app.role", PK: "id", DefaultSort: "name"`,
		`{Name: "name", Column: "name", Kind: query.KindText, Ops: []query.Op{query.OpEq, query.OpNe, query.OpIn, query.OpNotIn, query.OpContains, query.OpStartsWith, query.OpEndsWith}}`,
		`"list": {core.ActionWorkspaceRolesManage}`,
		`Apply: syncgen.Apply, ApplyDelete: syncgen.ApplyDelete`,
	} {
		if !strings.Contains(role, want) {
			t.Fatalf("role/resource.go is missing:\n%s\n---\n%s", want, role)
		}
	}
	// The tenant column and hidden columns must never be exposed as fields.
	if strings.Contains(role, `Column: "workspace_id"`) {
		t.Fatal("tenant column leaked into the field spec")
	}

	log := mustFile(t, files, "event/resource.go")
	for _, want := range []string{
		`package event`,
		`Singular: "log", Plural: "logs", Table: "event"`,
		`Table: "audit.event", PK: "id"`,
		`Enum: []string{"success", "error", "denied"}`,
		`"list": {core.ActionWorkspaceAuditRead}`,
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("event/resource.go is missing:\n%s\n---\n%s", want, log)
		}
	}
	// The read-only entity must not reference the syncer Apply funcs, nor leak the
	// @app.hide column into the field spec.
	if strings.Contains(log, "ApplyDelete") || strings.Contains(log, "syncgen") {
		t.Fatal("read-only entity should have no Apply/ApplyDelete")
	}
	if strings.Contains(log, `principal_hash`) {
		t.Fatal("@app.hide column leaked into the field spec")
	}
}

func TestEmitOpAndRoutes(t *testing.T) {
	files := filesByName(t, roleEntity())

	list := mustFile(t, files, "role/list.go")
	for _, want := range []string{
		`package role`,
		`func List() http.HandlerFunc { return rest.ListHandler(entity) }`,
	} {
		if !strings.Contains(list, want) {
			t.Fatalf("role/list.go is missing:\n%s\n---\n%s", want, list)
		}
	}

	routes := mustFile(t, files, "role/routes.go")
	for _, want := range []string{
		`func Register(mux *http.ServeMux, wrap rest.Wrap)`,
		`mux.Handle("GET /roles", wrap(rest.ReadRate, List()))`,
		`mux.Handle("GET /roles/{id}", wrap(rest.ReadRate, Get()))`,
		`mux.Handle("POST /roles", wrap(rest.WriteRate, Create()))`,
		`mux.Handle("PATCH /roles/{id}", wrap(rest.WriteRate, Update()))`,
		`mux.Handle("DELETE /roles/{id}", wrap(rest.WriteRate, Delete()))`,
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("role/routes.go is missing:\n%s\n---\n%s", want, routes)
		}
	}
}
