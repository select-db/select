package mysql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

// TestSee checks this dialect against the shared see cases. A statement that
// tests a hidden column must be refused, and one that returns it must be
// masked at the right positions.
func TestSee(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), core.GetSeeTestCases())
}
