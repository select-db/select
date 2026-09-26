package engine

import (
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/dialects"
)

// TestPermissionCases checks every dialect against the shared table of rights.
// It measures through Inspect rather than a dialect's own, since the floor on
// an unread statement is part of what decides the verdict.
func TestPermissionCases(t *testing.T) {
	meta := core.GetInspectTestMetadata()
	for _, name := range []string{"postgresql", "mysql", "sqlite"} {
		dialect := dialects.Get(name)
		if dialect == nil {
			t.Fatalf("no dialect named %q", name)
		}
		t.Run(name, func(t *testing.T) {
			testutil.RunPermCases(t, func(sql string) []core.InspectStatement {
				return Inspect(dialect, &meta, sql)
			}, testutil.PermCasesFor(name))
		})
	}
}
