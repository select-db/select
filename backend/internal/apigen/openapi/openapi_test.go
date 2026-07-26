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

// The emitted document is valid JSON and carries the RPC-over-POST shape, the
// auth scheme, and per-op required actions.
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

	// RPC paths: one POST per op, no REST verbs / path params.
	for _, p := range []string{"/role/list", "/role/get", "/role/create", "/role/update", "/role/delete"} {
		item, ok := doc.Paths[p]
		if !ok || item.Post == nil {
			t.Fatalf("missing POST %s", p)
		}
	}
	// The gated ops carry the required action; the open ops do not.
	if got := doc.Paths["/role/create"].Post.RequiredActions; len(got) != 1 || got[0] != "roles.manage" {
		t.Fatalf("create should require roles.manage, got %v", got)
	}
	if got := doc.Paths["/role/list"].Post.RequiredActions; len(got) != 0 {
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
