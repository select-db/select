package testutil

import (
	"fmt"
	"slices"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// Right is one grant a statement needs: an action on a table, on one column of
// a table, or manage, which is held on the connection and names no table.
type Right struct {
	Action string
	Schema string
	Table  string
	Column string
}

// Manage is the administration right.
var Manage = Right{Action: core.ActionManage}

// Only narrows a right to one column, which is what a case uses to say the
// statement reaches that column and not the rest of the table.
func (r Right) Only(column string) Right {
	r.Column = column
	return r
}

func (r Right) String() string {
	switch {
	case r.Table == "":
		return r.Action
	case r.Column == "":
		return fmt.Sprintf("%s on %s.%s", r.Action, r.Schema, r.Table)
	default:
		return fmt.Sprintf("%s on %s.%s.%s", r.Action, r.Schema, r.Table, r.Column)
	}
}

// RunPermCases checks a dialect against a table of permission cases. inspect is
// the seam the caller measures through: engine.Inspect, which floors a
// statement nobody read to manage, rather than a dialect's Inspect, which
// reports nothing for one.
//
// Each case is checked in both directions. Withholding any one right must
// refuse the statement, so no right in the case is decoration, and holding all
// of them must run it, so the case is not passing because everything is
// refused. Naming the table each right is on is what catches a statement that
// asks for the right action against the wrong relation.
//
// Denied, where a case sets it, is a grant that must fall short, which is how
// a case says a right does not stretch as far as it looks.
func RunPermCases(t *testing.T, inspect func(sql string) []core.InspectStatement, cases []PermCase) {
	t.Helper()
	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			statements := inspect(testCase.SQL)
			if len(statements) == 0 {
				t.Fatalf("read no statement, so the check had nothing to refuse:\n  %s", testCase.SQL)
			}
			assertResolved(t, statements, testCase)

			for idx, withheld := range testCase.Needs {
				rest := slices.Delete(slices.Clone(testCase.Needs), idx, idx+1)
				if err := core.CheckQueryPermissions(statements, TestDBInstanceID, PermGranting(rest...)); err == nil {
					t.Errorf("ran without %s, and %s:\n  %s", withheld, testCase.Why, testCase.SQL)
				}
			}

			if err := core.CheckQueryPermissions(statements, TestDBInstanceID, PermGranting(testCase.Needs...)); err != nil {
				t.Errorf("holding %v still refused it: %v\n  %s", testCase.Needs, err, testCase.SQL)
			}

			if len(testCase.Denied) > 0 {
				if err := core.CheckQueryPermissions(statements, TestDBInstanceID, PermGranting(testCase.Denied...)); err == nil {
					t.Errorf("ran holding only %v, and %s:\n  %s", testCase.Denied, testCase.Why, testCase.SQL)
				}
			}
		})
	}
}

// assertResolved fails a case that would pass without the inspector having
// understood anything: a right on a table nobody resolved, or an operation the
// floor supplied rather than the statement.
func assertResolved(t *testing.T, statements []core.InspectStatement, testCase PermCase) {
	t.Helper()
	for _, right := range testCase.Needs {
		if right.Table == "" {
			continue
		}
		if !Touches(statements, Touch{Schema: right.Schema, Name: right.Table}) {
			t.Fatalf("resolved no %s.%s, so the check had nothing to ask about:\n  %s",
				right.Schema, right.Table, testCase.SQL)
		}
	}
	if testCase.Op != "" && !Touches(statements, Touch{Op: testCase.Op}) {
		t.Fatalf("read no %s statement, so what passed was the floor rather than the statement:\n  %s",
			testCase.Op, testCase.SQL)
	}
}

// PermGranting is a policy granting exactly these rights and nothing else.
func PermGranting(rights ...Right) core.CompiledPermissions {
	id := TestDBInstanceID
	entries := make([]core.PermissionEntry, 0, len(rights))
	for _, right := range rights {
		entry := core.PermissionEntry{
			DbInstanceID: &id,
			Action:       right.Action,
			Effect:       "allow",
			RoleName:     "test-role",
		}
		if right.Table != "" {
			schema, table := right.Schema, right.Table
			entry.SchemaName, entry.TableName = &schema, &table
		}
		if right.Column != "" {
			column := right.Column
			entry.ColumnName = &column
		}
		entries = append(entries, entry)
	}
	return core.Compile(entries).WithDenyUnmanaged()
}
