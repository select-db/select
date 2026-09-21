package testutil

import (
	"fmt"
	"slices"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// Right is one grant a statement needs: an action on a table, or manage, which
// is held on the connection and names no table.
type Right struct {
	Action string
	Schema string
	Table  string
}

// Manage is the administration right.
var Manage = Right{Action: core.ActionManage}

func (r Right) String() string {
	if r.Table == "" {
		return r.Action
	}
	return fmt.Sprintf("%s on %s.%s", r.Action, r.Schema, r.Table)
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
		})
	}
}

// assertResolved is the guard against a case that passes without the inspector
// having understood anything. A case needing a right on a table proves nothing
// unless that table was resolved, and one needing manage alone can be met by a
// statement floored to unknown, which is why a case that expects an operation
// says so.
func assertResolved(t *testing.T, statements []core.InspectStatement, testCase PermCase) {
	t.Helper()
	for _, right := range testCase.Needs {
		if right.Table == "" {
			continue
		}
		if !touchesTable(statements, right.Schema, right.Table) {
			t.Fatalf("resolved no %s.%s, so the check had nothing to ask about:\n  %s",
				right.Schema, right.Table, testCase.SQL)
		}
	}
	if testCase.Op != "" && !hasOperation(statements, testCase.Op) {
		t.Fatalf("read no %s statement, so what passed was the floor rather than the statement:\n  %s",
			testCase.Op, testCase.SQL)
	}
}

func touchesTable(statements []core.InspectStatement, schema, table string) bool {
	for _, stmt := range statements {
		for _, named := range stmt.Tables {
			if named.Schema == schema && named.Name == table {
				return true
			}
		}
		if touchesTable(stmt.Subqueries, schema, table) || touchesTable(stmt.Also, schema, table) {
			return true
		}
	}
	return false
}

func hasOperation(statements []core.InspectStatement, op core.InspectOperation) bool {
	for _, stmt := range statements {
		if stmt.Operation == op {
			return true
		}
		if hasOperation(stmt.Subqueries, op) || hasOperation(stmt.Also, op) {
			return true
		}
	}
	return false
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
		entries = append(entries, entry)
	}
	return core.Compile(entries).WithDenyUnmanaged()
}

// PermHolding grants these actions on every table. It is what a case cannot
// use, since a right granted everywhere cannot say which relation the
// statement asked about.
func PermHolding(dbID string, actions ...string) core.CompiledPermissions {
	id := dbID
	entries := make([]core.PermissionEntry, 0, len(actions))
	for _, action := range actions {
		entries = append(entries, core.PermissionEntry{
			DbInstanceID: &id,
			Action:       action,
			Effect:       "allow",
			RoleName:     "test-role",
		})
	}
	return core.Compile(entries).WithDenyUnmanaged()
}
