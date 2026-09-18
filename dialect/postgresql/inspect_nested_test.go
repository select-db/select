package postgresql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
)

// A statement nested inside another one is still a statement. Where the
// inspector did not recognise it, it reported nothing, and a permission check
// reads nothing as nothing to check. These pin that each nested statement
// reaches the result, whatever it is wrapped in.

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
			name: "a data-modifying CTE is a delete, not just the select around it",
			sql:  "WITH x AS (DELETE FROM t1 RETURNING c1) SELECT c1 FROM x",
			want: []touch{{core.InspectOpDelete, s, "t1"}},
		},
		{
			name: "a data-modifying CTE is an update",
			sql:  "WITH x AS (UPDATE t1 SET c2 = 'x' RETURNING c1) SELECT c1 FROM x",
			want: []touch{{core.InspectOpUpdate, s, "t1"}},
		},
		{
			name: "a data-modifying CTE is an insert",
			sql:  "WITH x AS (INSERT INTO t1 (c1) VALUES (1) RETURNING c1) SELECT c1 FROM x",
			want: []touch{{core.InspectOpInsert, s, "t1"}},
		},
		{
			name: "CREATE TABLE AS creates one table and reads another",
			sql:  "CREATE TABLE t9 AS SELECT c1 FROM t1",
			want: []touch{{core.InspectOpCreate, s, "t9"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "CREATE VIEW reads the tables behind it",
			sql:  "CREATE VIEW v AS SELECT c1 FROM t1",
			want: []touch{{core.InspectOpCreate, s, "v"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "CREATE MATERIALIZED VIEW reads them too",
			sql:  "CREATE MATERIALIZED VIEW mv AS SELECT c1 FROM t1",
			want: []touch{{core.InspectOpCreate, s, "mv"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "COPY from a query reads the query's tables",
			sql:  "COPY (SELECT c1 FROM t1) TO '/tmp/x.csv'",
			want: []touch{{core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "UPDATE ... FROM reads the relation it joins",
			sql:  "UPDATE t2 SET c1 = t1.c1 FROM t1",
			want: []touch{{core.InspectOpUpdate, s, "t2"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "DELETE ... USING reads the relation it joins",
			sql:  "DELETE FROM t2 USING t1 WHERE t2.c1 = t1.c1",
			want: []touch{{core.InspectOpDelete, s, "t2"}, {core.InspectOpSelect, s, "t1"}},
		},
		{
			name: "an ordinary CTE still reads its table",
			sql:  "WITH x AS (SELECT c1 FROM t1) SELECT c1 FROM x",
			want: []touch{{core.InspectOpSelect, s, "t1"}},
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

// The outer statement maps CTE names to their bodies by position, so a body the
// inspector skipped used to shift every later CTE onto the wrong one.
func TestInspectEveryCTEYieldsOneSubquery(t *testing.T) {
	stmts := inspectNested(t, `
		WITH a AS (SELECT c1 FROM t1),
		     b AS (DELETE FROM t2 RETURNING c1),
		     c AS (SELECT c1 FROM other.t3)
		SELECT c1 FROM a`)

	if len(stmts) != 1 {
		t.Fatalf("got %d statements, want 1", len(stmts))
	}
	if got := len(stmts[0].Subqueries); got < 3 {
		t.Fatalf("3 CTEs produced %d subqueries: a body that was skipped shifts the rest", got)
	}
}

// COPY moves rows between a table and the server's own filesystem, so it stays
// unclassified and takes manage whether or not it carries a query.
func TestInspectCopyIsUnclassified(t *testing.T) {
	for _, sql := range []string{
		"COPY t1 FROM '/tmp/x.csv'",
		"COPY t1 TO '/tmp/x.csv'",
		"COPY (SELECT c1 FROM t1) TO '/tmp/x.csv'",
	} {
		t.Run(sql, func(t *testing.T) {
			stmts := inspectNested(t, sql)
			if len(stmts) != 1 {
				t.Fatalf("got %d statements, want 1", len(stmts))
			}
			if stmts[0].Operation != core.InspectOpUnknown {
				t.Errorf("COPY is %s, so it no longer needs manage", stmts[0].Operation)
			}
		})
	}
}
