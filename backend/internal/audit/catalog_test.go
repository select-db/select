package audit

import "testing"

// The catalog is the single source of truth for what's logged; keep it honest.
func TestCatalogHasNoDuplicateEventTypes(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range Catalog {
		if s.Domain == "" || s.Action == "" {
			t.Fatalf("catalog entry missing domain/action: %+v", s)
		}
		if seen[s.Type()] {
			t.Fatalf("duplicate event type in catalog: %q", s.Type())
		}
		seen[s.Type()] = true
	}
}

func TestEveryCatalogSpecIsRegistered(t *testing.T) {
	for _, s := range Catalog {
		if !registered[s.Type()] {
			t.Fatalf("spec %q not registered", s.Type())
		}
	}
}
