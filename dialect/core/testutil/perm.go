package testutil

import (
	"slices"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// PermTestDBInstanceID names the connection the cases are checked against.
const PermTestDBInstanceID = "db1"

// RunPermCases checks a dialect against a table of permission cases. inspect
// is the seam the caller measures through: engine.Inspect, which floors a
// statement nobody read to manage, rather than a dialect's Inspect, which
// reports nothing for one.
//
// Each case is checked in both directions. Withholding any one right must
// refuse the statement, so no right in the case is decoration, and holding all
// of them must run it, so the case is not passing because everything is
// refused.
func RunPermCases(t *testing.T, inspect func(sql string) []core.InspectStatement, cases []PermCase) {
	t.Helper()
	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			statements := inspect(testCase.SQL)
			if len(statements) == 0 {
				t.Fatalf("read no statement, so the check had nothing to refuse:\n  %s", testCase.SQL)
			}

			for idx, withheld := range testCase.Needs {
				rest := slices.Delete(slices.Clone(testCase.Needs), idx, idx+1)
				if err := core.CheckQueryPermissions(statements, PermTestDBInstanceID, PermHolding(rest...)); err == nil {
					t.Errorf("ran without %q, and %s:\n  %s", withheld, testCase.Why, testCase.SQL)
				}
			}

			if err := core.CheckQueryPermissions(statements, PermTestDBInstanceID, PermHolding(testCase.Needs...)); err != nil {
				t.Errorf("holding %v still refused it: %v\n  %s", testCase.Needs, err, testCase.SQL)
			}
		})
	}
}

// PermHolding is a policy granting these actions on everything, and nothing
// else. WithDenyUnmanaged is what makes an empty grant mean "no rights" rather
// than "this connection has no rules, so let it through".
func PermHolding(actions ...string) core.CompiledPermissions {
	id := PermTestDBInstanceID
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
