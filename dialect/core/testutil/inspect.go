package testutil

import (
	core "github.com/selectDb/dialect/core"
)

// Touch is one operation on one table, looked for anywhere in a statement tree.
// The dialects report the same SQL with different shapes, so the nested-statement
// tests assert that a read or a write is present rather than matching the tree.
// An empty Op matches any operation and an empty Name any table, which is how
// one half of the pair is asked about on its own.
type Touch struct {
	Op     core.InspectOperation
	Schema string
	Name   string
}

// Touches reports whether any statement in the tree performs want.
func Touches(stmts []core.InspectStatement, want Touch) bool {
	for _, stmt := range stmts {
		if (want.Op == "" || stmt.Operation == want.Op) && holdsTable(stmt, want) {
			return true
		}
		if Touches(stmt.Subqueries, want) || Touches(stmt.Also, want) {
			return true
		}
	}
	return false
}

func holdsTable(stmt core.InspectStatement, want Touch) bool {
	if want.Name == "" {
		return true
	}
	for _, table := range stmt.Tables {
		if table.Schema == want.Schema && table.Name == want.Name {
			return true
		}
	}
	return false
}
