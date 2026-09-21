package testutil

import (
	"slices"
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// RunSeeCases checks a dialect against the shared see cases.
//
// It drives the same path a query takes, minus the database: the statement is
// inspected, checked against the policy, and its result columns are matched to
// the fields the inspection found. Nothing here needs a server, so a dialect
// gets the whole boundary covered by reading its own grammar.
func RunSeeCases(t *testing.T, dialect core.SQLDialect, cases []core.SeeCase) {
	t.Helper()
	meta := core.GetSeeTestMetadata()
	perms := core.GetSeeTestPermissions()

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			var masked []int
			statements := dialect.Inspect(meta, testCase.SQL)
			if len(statements) == 0 {
				// The floor engine.Inspect puts under a dialect that reads
				// nothing: a statement nobody could read is not a statement
				// with no rules.
				statements = []core.InspectStatement{core.UnknownStatement()}
			}

			err := core.CheckQueryPermissions(statements, core.SeeTestDBInstanceID, perms)
			if err == nil {
				err = core.CheckSeePredicates(statements, core.SeeTestDBInstanceID, perms)
			}

			if err == nil {
				// A hidden column an expression swallowed has no result column
				// of its own to mask, which only the driver's columns show.
				err = seeOnResult(statements, testCase.Columns, perms, &masked)
			}

			if testCase.Refused {
				if err == nil {
					t.Fatalf("ran, and answering about a hidden column is what it must not do:\n  %s", testCase.SQL)
				}
				if !strings.Contains(err.Error(), core.ActionSee) {
					t.Errorf("refused for the wrong reason: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ordinary work refused: %v\n  %s", err, testCase.SQL)
			}
			if !slices.Equal(masked, testCase.Masked) {
				t.Errorf("masked %v, want %v, over columns %v\n  %s",
					masked, testCase.Masked, testCase.Columns, testCase.SQL)
			}
		})
	}
}

// seeOnResult is the check the engine runs once the driver reports the result
// columns. It reports which positions are masked through into.
func seeOnResult(statements []core.InspectStatement, columns []string, perms core.CompiledPermissions, into *[]int) error {
	if len(columns) == 0 {
		return nil
	}
	for _, statement := range statements {
		if !core.ReturnsRows(statement.Operation) {
			continue
		}
		masked, err := core.EvaluateSee(statement, columns, core.SeeTestDBInstanceID, perms)
		if err != nil {
			return err
		}
		*into = masked
		return nil
	}
	return nil
}
