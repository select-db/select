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
			{Op: "list"}, // open to any member
			{Op: "get"},
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

// filesByName indexes the emitted set by path-qualified name, and collapses each
// file's whitespace runs (gofmt aligns struct/map values, so single-space
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

func mustContain(t *testing.T, name, content string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(content, w) {
			t.Fatalf("%s is missing:\n%s\n---\n%s", name, w, content)
		}
	}
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
	mustContain(t, "routes.go", c,
		`package gen`,
		`role "backend/internal/api/gen/role"`,
		`event "backend/internal/api/gen/event"`,
		`func RegisterRoutes(mux *http.ServeMux, wrap rest.Wrap)`,
		`role.Register(mux, wrap)`,
		`event.Register(mux, wrap)`,
	)
}

func TestEmitResource(t *testing.T) {
	files := filesByName(t, roleEntity(), logEntity())

	role := mustFile(t, files, "role/resource.go")
	mustContain(t, "role/resource.go", role,
		`package role`,
		`import "backend/internal/api/query"`,
		`var resource = query.Resource{`,
		`Table: "app.role", PK: "id", DefaultSort: "name"`,
		`{Name: "name", Column: "name", Kind: query.KindText, Ops: []query.Op{"eq", "ne", "in", "not in", "contains", "startswith", "endswith"}}`,
		`const singular = "role"`,
		`const table = "role"`, // has write ops
	)
	// resource.go is read-side data only: no handler logic, no Apply wiring, and
	// the write-body contract lives in the write handlers, not here.
	if strings.Contains(role, "Requires") || strings.Contains(role, "http.HandlerFunc") ||
		strings.Contains(role, "syncgen") || strings.Contains(role, "validate") {
		t.Fatalf("resource.go should be read-side data only:\n%s", role)
	}
	// The tenant column must never be exposed as a field.
	if strings.Contains(role, `Column: "workspace_id"`) {
		t.Fatal("tenant column leaked into the field spec")
	}

	log := mustFile(t, files, "event/resource.go")
	mustContain(t, "event/resource.go", log,
		`package event`,
		`Table: "audit.event", PK: "id"`,
		`Enum: []string{"success", "error", "denied"}`,
		`const singular = "log"`,
	)
	if strings.Contains(log, "const table") {
		t.Fatal("read-only entity should not declare the write-commit table const")
	}
	if strings.Contains(log, `principal_hash`) {
		t.Fatal("@app.hide column leaked into the field spec")
	}
}

func TestEmitWriteSpecInlined(t *testing.T) {
	files := filesByName(t, roleEntity())

	// The write-body contract is inlined into each write handler (not referenced
	// from resource.go), so the file shows exactly what it accepts.
	for _, name := range []string{"role/create.go", "role/update.go"} {
		f := mustFile(t, files, name)
		mustContain(t, name, f,
			`spec := validate.Schema{Fields: []validate.Field{`,
			`{Name: "id", Kind: query.KindUUID, Required: true}`,
			`{Name: "name", Kind: query.KindText, Required: true}`,
		)
	}
	// Read handlers and delete carry no write spec.
	for _, name := range []string{"role/list.go", "role/get.go", "role/delete.go"} {
		if strings.Contains(mustFile(t, files, name), "validate.Schema") {
			t.Fatalf("%s should not carry a write spec", name)
		}
	}
}

func TestEmitListInlined(t *testing.T) {
	files := filesByName(t, roleEntity(), logEntity())

	// The full list handler is inlined into the file — the gate, the query runtime
	// call, the envelope response — not a delegation into the rest package.
	role := mustFile(t, files, "role/list.go")
	mustContain(t, "role/list.go", role,
		`func List() http.HandlerFunc {`,
		`a := authz.ActorOf(r)`,
		`if !rest.Gate(a, nil) {`, // list is open to any member
		`page, err := query.List(r.Context(), db.GetDB(), resource, query.Request{`,
		`rest.WriteQueryError(w, err)`,
		`rest.WriteJSON(w, http.StatusOK, page)`,
	)
	// An open op does not import the core action package.
	if strings.Contains(role, "dialect/core") {
		t.Fatalf("open list op should not import core:\n%s", role)
	}

	// A gated read states its required action inline.
	log := mustFile(t, files, "event/list.go")
	mustContain(t, "event/list.go", log,
		`if !rest.Gate(a, []string{core.ActionWorkspaceAuditRead}) {`,
		`core "github.com/selectDb/dialect/core"`,
	)
}

func TestEmitCreateInlined(t *testing.T) {
	role := mustFile(t, filesByName(t, roleEntity()), "role/create.go")
	// The whole create flow is present in the one file: gate, decode, id/conflict
	// checks, the synthesized syncer commit, Apply, and the read-back response.
	mustContain(t, "role/create.go", role,
		`func Create() http.HandlerFunc {`,
		`if !rest.Gate(a, []string{core.ActionWorkspaceRolesManage}) {`,
		`body, ok := rest.DecodeBody(w, r)`,
		// body is validated against the inlined write contract before proceeding.
		`spec := validate.Schema{Fields: []validate.Field{`,
		`if verr := validate.ForCreate(spec, body); verr != nil {`,
		`rest.WriteValidationError(w, verr)`,
		`"backend/internal/api/validate"`,
		`id, _ := body["id"].(string)`,
		`rest.WriteError(w, http.StatusConflict, singular+" already exists")`,
		`c := types.Commit{`,
		`Operation: "create",`,
		`TableName: table,`,
		`applied, _, err := syncgen.Apply(r.Context(), a.UserID, c, time.Time{})`,
		`syncgen "backend/internal/syncer/gen/role"`,
		`rest.WriteJSON(w, http.StatusCreated, row)`,
	)
}

func TestEmitDeleteInlined(t *testing.T) {
	role := mustFile(t, filesByName(t, roleEntity()), "role/delete.go")
	mustContain(t, "role/delete.go", role,
		`func Delete() http.HandlerFunc {`,
		`Operation: "delete",`,
		`if _, _, err := syncgen.ApplyDelete(r.Context(), a.UserID, c); err != nil {`,
		`w.WriteHeader(http.StatusNoContent)`,
	)
}

func TestEmitEntityRoutes(t *testing.T) {
	routes := mustFile(t, filesByName(t, roleEntity()), "role/routes.go")
	mustContain(t, "role/routes.go", routes,
		`func Register(mux *http.ServeMux, wrap rest.Wrap)`,
		`mux.Handle("GET /roles", wrap(rest.ReadRate, List()))`,
		`mux.Handle("GET /roles/{id}", wrap(rest.ReadRate, Get()))`,
		`mux.Handle("POST /roles", wrap(rest.WriteRate, Create()))`,
		`mux.Handle("PATCH /roles/{id}", wrap(rest.WriteRate, Update()))`,
		`mux.Handle("DELETE /roles/{id}", wrap(rest.WriteRate, Delete()))`,
	)
}
