package testutil

import (
	"slices"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// CompletionContextWant is the expected shape of a parsed completion context.
// The zero value means "no parts, no qualifier", which is what most cases want.
type CompletionContextWant struct {
	Targets       core.CompletionTarget
	Parts         []string
	CaretAfterDot bool
	SchemaFilter  string
	TargetTable   string
}

// AssertCompletionContext reports every field that differs, so one run shows
// the whole disagreement rather than the first field of it. Shared so the
// per-dialect quoting tests carry their cases and nothing else.
func AssertCompletionContext(t *testing.T, got core.CompletionContext, want CompletionContextWant) {
	t.Helper()

	if got.Targets != want.Targets {
		t.Errorf("Targets = %d, want %d", got.Targets, want.Targets)
	}
	if !slices.Equal(got.Parts, want.Parts) {
		t.Errorf("Parts = %v, want %v", got.Parts, want.Parts)
	}
	if got.CaretAfterDot != want.CaretAfterDot {
		t.Errorf("CaretAfterDot = %v, want %v", got.CaretAfterDot, want.CaretAfterDot)
	}
	if got.SchemaFilter != want.SchemaFilter {
		t.Errorf("SchemaFilter = %q, want %q", got.SchemaFilter, want.SchemaFilter)
	}
	if got.TargetTable != want.TargetTable {
		t.Errorf("TargetTable = %q, want %q", got.TargetTable, want.TargetTable)
	}
}
