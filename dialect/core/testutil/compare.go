package testutil

import (
	"fmt"
	"testing"

	core "github.com/selectDb/dialect/core"
)

// CompareResult reports every way actual differs from expected. idx numbers
// the statement within a script; pass a negative number for a subquery, whose
// position is its parent's.
func CompareResult(t *testing.T, idx int, expected, actual core.InspectStatement) {
	t.Helper()
	prefix := ""
	if idx >= 0 {
		prefix = fmt.Sprintf("Result %d: ", idx)
	}

	if actual.Operation != expected.Operation {
		t.Errorf("%sexpected operation %q, got %q", prefix, expected.Operation, actual.Operation)
	}
	if !sameTables(expected.Tables, actual.Tables) {
		t.Errorf("%stables mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Tables, actual.Tables)
	}
	if !sameFields(expected.Fields, actual.Fields) {
		t.Errorf("%sfields mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Fields, actual.Fields)
	}
	if !sameFields(expected.Where, actual.Where) {
		t.Errorf("%sWHERE fields mismatch\nExpected: %+v\nGot: %+v", prefix, expected.Where, actual.Where)
	}

	if len(expected.Subqueries) != len(actual.Subqueries) {
		t.Errorf("%ssubqueries count mismatch: expected %d, got %d",
			prefix, len(expected.Subqueries), len(actual.Subqueries))
		return
	}
	for i, expectedSub := range expected.Subqueries {
		CompareResult(t, -1, expectedSub, actual.Subqueries[i])
	}
}

func sameFields(expected, actual []core.InspectField) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].Name != actual[i].Name ||
			expected[i].Table != actual[i].Table ||
			expected[i].Schema != actual[i].Schema ||
			alias(expected[i].Alias) != alias(actual[i].Alias) {
			return false
		}
	}
	return true
}

func sameTables(expected, actual []core.InspectTable) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].Name != actual[i].Name ||
			expected[i].Schema != actual[i].Schema ||
			alias(expected[i].Alias) != alias(actual[i].Alias) {
			return false
		}
	}
	return true
}

// alias reads an optional name, which the expectations write as nil where the
// statement gave none.
func alias(name *string) string {
	if name == nil {
		return ""
	}
	return *name
}
