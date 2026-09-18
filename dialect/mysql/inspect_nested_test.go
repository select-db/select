package mysql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
)

// A statement nested inside another one is still a statement. Where the
// inspector did not recognise it, it reported nothing, and a permission check
// reads nothing as nothing to check. These pin that the query a CREATE is
// filled from reaches the result.

// touch is one operation on one table, looked for anywhere in a statement tree.
type touch struct {
	op           core.InspectOperation
	schema, name string
}

func inspectNested(t *testing.T, sql string) []core.InspectStatement {
	t.Helper()
	meta := core.GetInspectTestMetadata()
	return NewInspector(NewDialect(), meta).Inspect(sql)
}

func touches(stmts []core.InspectStatement, want touch) bool {
	for _, stmt := range stmts {
		if stmt.Operation == want.op {
			for _, tbl := range stmt.Tables {
				if tbl.Schema == want.schema && tbl.Name == want.name {
					return true
				}
			}
		}
		if touches(stmt.Subqueries, want) {
			return true
		}
	}
	return false
}

func TestInspectNestedStatementsReachTheResult(t *testing.T) {
	const s = "main"

	tests := []struct {
		name string
		sql  string
		want []touch
	}{
		{
			name: "CREATE TABLE AS creates one table and reads another",
			sql:  "CREATE TABLE t9 AS SELECT c1 FROM t1",
			want: []touch{{core.InspectOpCreate, s, "t9"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "CREATE TABLE ... SELECT, the spelling without AS, reads it too",
			sql:  "CREATE TABLE t9 SELECT c1 FROM t1",
			want: []touch{{core.InspectOpCreate, s, "t9"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "CREATE VIEW reads the tables behind it",
			sql:  "CREATE VIEW v AS SELECT c1 FROM t1",
			want: []touch{{core.InspectOpCreate, s, "v"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "INSERT ... SELECT still reads its source",
			sql:  "INSERT INTO t2 (c1) SELECT c1 FROM t1",
			want: []touch{{core.InspectOpInsert, s, "t2"}, {core.InspectOpSelect, s, "t1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmts := inspectNested(t, tt.sql)
			for _, want := range tt.want {
				if !touches(stmts, want) {
					t.Errorf("no %s on %s.%s anywhere in %+v", want.op, want.schema, want.name, stmts)
				}
			}
		})
	}
}
