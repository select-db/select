package openapi

import (
	"backend/internal/apigen/schema"
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// Golden test: the OpenAPI doc for the role + permission entities is committed
// under testdata and diffed on every run. Regenerate with
// `go test ./internal/apigen/openapi -update`.
func TestEmitOpenAPIGolden(t *testing.T) {
	ents := build(t, schema.RoleTable(), schema.PermissionTable())
	got, err := EmitOpenAPI(ents)
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "openapi.json")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("openapi mismatch (run -update to refresh):\n--- generated ---\n%s", got)
	}
}

// The emitted document is valid JSON and carries the REST shape (proper verbs,
// collection + item paths, id path param), the auth scheme, and per-op actions.
func TestEmitOpenAPIShape(t *testing.T) {
	ents := build(t, schema.RoleTable())
	raw, err := EmitOpenAPI(ents)
	if err != nil {
		t.Fatal(err)
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("emitted invalid JSON: %v", err)
	}

	// Collection path: GET (list) + POST (create).
	coll := doc.Paths["/roles"]
	if coll == nil || coll.Get == nil || coll.Post == nil {
		t.Fatalf("expected GET+POST on /roles, got %+v", coll)
	}
	// Item path: GET (get) + PATCH (update) + DELETE (delete).
	item := doc.Paths["/roles/{id}"]
	if item == nil || item.Get == nil || item.Patch == nil || item.Delete == nil {
		t.Fatalf("expected GET+PATCH+DELETE on /roles/{id}, got %+v", item)
	}
	// The item ops take id as a path parameter.
	if len(item.Get.Parameters) != 1 || item.Get.Parameters[0].In != "path" || item.Get.Parameters[0].Name != "id" {
		t.Fatalf("get should take an id path param, got %+v", item.Get.Parameters)
	}
	// Proper status codes: 201 on create, 204 on delete, 404 on item ops.
	if _, ok := coll.Post.Responses["201"]; !ok {
		t.Fatal("create should respond 201")
	}
	if _, ok := item.Delete.Responses["204"]; !ok {
		t.Fatal("delete should respond 204")
	}
	if _, ok := item.Get.Responses["404"]; !ok {
		t.Fatal("get should document 404")
	}
	// The gated ops carry the required action; the open ops do not.
	if got := coll.Post.RequiredActions; len(got) != 1 || got[0] != "roles.manage" {
		t.Fatalf("create should require roles.manage, got %v", got)
	}
	if got := coll.Get.RequiredActions; len(got) != 0 {
		t.Fatalf("list should be open, got %v", got)
	}
	// Global bearer security is declared.
	if _, ok := doc.Components.SecuritySchemes[bearerScheme]; !ok {
		t.Fatal("bearer security scheme missing")
	}

	// System columns never surface in the response schema; updated_at is read-only.
	s := string(raw)
	for _, leaked := range []string{`"workspace_id"`, `"deleted_at"`} {
		if strings.Contains(s, leaked) {
			t.Fatalf("system column leaked into spec: %s", leaked)
		}
	}
	if !doc.Components.Schemas["Role"].Properties["updated_at"].ReadOnly {
		t.Fatal("updated_at should be read-only in the response schema")
	}
}

// A foreign key is a writable field in the create request; the tenant FK is not.
func TestWriteSchemaIncludesFK(t *testing.T) {
	ents := build(t, schema.PermissionTable())
	raw, err := EmitOpenAPI(ents)
	if err != nil {
		t.Fatal(err)
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	req := doc.Components.Schemas["PermissionCreateRequest"]
	if req == nil || req.Properties["role_id"] == nil {
		t.Fatal("create request should include the role_id FK")
	}
	if req.Properties["workspace_id"] != nil {
		t.Fatal("tenant FK workspace_id must not be client-settable")
	}
}

func build(t *testing.T, tables ...schema.RawTable) []schema.Entity {
	t.Helper()
	ents, errs := schema.Build(schema.RawSchema{Tables: tables})
	if len(errs) != 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}
	return ents
}
