package testutil

import (
	"slices"
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// RunSeeCases checks a dialect against the shared see cases. It drives the
// path a query takes, minus the database: the statement is inspected, checked
// against the policy, and its result columns are matched to the fields the
// inspection found.
func RunSeeCases(t *testing.T, dialect core.SQLDialect, cases []SeeCase) {
	t.Helper()
	meta := GetSeeTestMetadata()
	perms := GetSeeTestPermissions()

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			if len(testCase.Masked) > 0 && len(testCase.Columns) == 0 {
				t.Fatal("the case expects masking but names no result columns, so nothing would be checked")
			}
			// A case that runs and leaves Columns nil checks no masking at
			// all. Where the statement really hands back no columns, the case
			// says so with an empty slice rather than by omission.
			if !testCase.Refused && testCase.Columns == nil {
				t.Fatal("the case runs but names no result columns; write Columns: []string{} if it returns none")
			}

			statements := dialect.Inspect(meta, testCase.SQL)
			if len(statements) == 0 {
				// The floor engine.Inspect puts under a dialect that read
				// nothing.
				statements = []core.InspectStatement{core.UnknownStatement()}
			}

			err := core.CheckQueryPermissions(statements, TestDatasourceID, perms)
			if err == nil {
				err = core.CheckSeePredicates(statements, TestDatasourceID, perms)
			}

			var masked []int
			if err == nil {
				// A hidden column an expression swallowed has no result column
				// of its own to mask, which only the driver's columns show.
				masked, err = seeOnResult(statements, testCase.Columns, perms)
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
// columns, and the positions it masks.
func seeOnResult(statements []core.InspectStatement, columns []string, perms core.CompiledPermissions) ([]int, error) {
	if len(columns) == 0 {
		return nil, nil
	}
	for _, statement := range statements {
		if !core.ReturnsRows(statement.Operation) {
			continue
		}
		return core.EvaluateSee(statement, columns, TestDatasourceID, perms)
	}
	return nil, nil
}
