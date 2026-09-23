package postgresql

import (
	"testing"

	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/postgresql/cases"
)

// TestSee checks this dialect against the shared see cases. A statement that
// tests a hidden column must be refused, and one that returns it must be
// masked at the right positions.
func TestSee(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), testutil.GetSeeTestCases())
	testutil.RunSeeCases(t, NewDialect(), testutil.GetSeeCasesPostgreSQLAndSQLite())
}

// TestSeePostgreSQLSpecific covers what only PostgreSQL reads: the USING of a
// DELETE, RETURNING on an INSERT, and a name in double quotes.
func TestSeePostgreSQLSpecific(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), cases.SeeCases())
}
