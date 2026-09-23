package mysql

import (
	"testing"

	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/mysql/cases"
)

// TestSee checks this dialect against the shared see cases. A statement that
// tests a hidden column must be refused, and one that returns it must be
// masked at the right positions.
func TestSee(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), testutil.GetSeeTestCases())
}

// TestSeeMySQLSpecific covers the ways only MySQL has of reading a column: a
// multi-table UPDATE or DELETE, and backtick quoting.
func TestSeeMySQLSpecific(t *testing.T) {
	testutil.RunSeeCases(t, NewDialect(), cases.SeeCases())
}
