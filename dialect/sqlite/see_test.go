package sqlite

import (
	"testing"

	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/sqlite/cases"
)

// TestSee checks this dialect against the shared see cases. A statement that
// tests a hidden column must be refused, and one that returns it must be
// masked at the right positions.
func TestSee(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), testutil.GetSeeTestCases())
	testutil.RunSeeCases(t, NewDialect(), testutil.GetSeeCasesPostgreSQLAndSQLite())
}

// TestSeeQuoting covers the three ways SQLite lets a statement quote a name.
// A column wearing quotes the dialect does not strip matches no rule, so it
// comes back in the clear.
func TestSeeQuoting(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), cases.SeeCases())
}
