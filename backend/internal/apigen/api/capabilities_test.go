package api

import (
	"backend/internal/apigen/schema"
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// Golden test: the capabilities doc for role is committed under testdata and
// diffed on every run. Regenerate with `go test ./internal/apigen -update`.
func TestEmitAPICapabilitiesRole(t *testing.T) {
	ents, errs := schema.Build(schema.RawSchema{Tables: []schema.RawTable{schema.RoleTable()}})
	if len(errs) != 0 {
		t.Fatalf("unexpected lint errors: %v", errs)
	}
	got, err := EmitAPICapabilities(ents)
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "capabilities_role.json")
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
		t.Fatalf("capabilities mismatch:\n--- generated ---\n%s", got)
	}

	// System columns are enforced, never exposed; updated_at stays exposed.
	s := string(got)
	for _, hidden := range []string{`"name": "workspace_id"`, `"name": "deleted_at"`} {
		if strings.Contains(s, hidden) {
			t.Fatalf("system column leaked into capabilities: %s", hidden)
		}
	}
	for _, shown := range []string{`"name": "id"`, `"name": "name"`, `"name": "updated_at"`} {
		if !strings.Contains(s, shown) {
			t.Fatalf("expected field missing from capabilities: %s", shown)
		}
	}
}
