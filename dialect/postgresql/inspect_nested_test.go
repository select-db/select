package postgresql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

// What PostgreSQL nests inside a statement, and that each nested statement
// reaches the result. The permission consequence is pinned in dialect/engine;
// here is the shape only.

func inspectNested(t *testing.T, sql string) []core.InspectStatement {
	t.Helper()
	return NewInspector(NewDialect(), core.GetInspectTestMetadata()).Inspect(sql)
}

func TestInspectNestedStatementsReachTheResult(t *testing.T) {
	const s = "main"

	tests := []struct {
		name string
		sql  string
		want []testutil.Touch
	}{
		{
			name: "a data-modifying CTE is a delete, not just the select around it",
			sql:  "WITH x AS (DELETE FROM t1 RETURNING c1) SELECT c1 FROM x",
			want: []testutil.Touch{{Op: core.InspectOpDelete, Schema: s, Name: "t1"}},
		},
		{
			name: "a data-modifying CTE is an update",
			sql:  "WITH x AS (UPDATE t1 SET c2 = 'x' RETURNING c1) SELECT c1 FROM x",
			want: []testutil.Touch{{Op: core.InspectOpUpdate, Schema: s, Name: "t1"}},
		},
		{
			name: "a data-modifying CTE is an insert",
			sql:  "WITH x AS (INSERT INTO t1 (c1) VALUES (1) RETURNING c1) SELECT c1 FROM x",
			want: []testutil.Touch{{Op: core.InspectOpInsert, Schema: s, Name: "t1"}},
		},
		{
			name: "a CTE on a DELETE is still a statement",
			sql:  "WITH x AS (DELETE FROM t1 RETURNING c1) DELETE FROM t2 WHERE c1 = 1",
			want: []testutil.Touch{
				{Op: core.InspectOpDelete, Schema: s, Name: "t1"},
				{Op: core.InspectOpDelete, Schema: s, Name: "t2"},
			},
		},
		{
			name: "CREATE MATERIALIZED VIEW reads the tables behind it",
			sql:  "CREATE MATERIALIZED VIEW mv AS SELECT c1 FROM t1",
			want: []testutil.Touch{
				{Op: core.InspectOpCreate, Schema: s, Name: "mv"},
				{Op: core.InspectOpSelect, Schema: s, Name: "t1"},
			},
		},
		{
			name: "COPY from a query reads the query's tables",
			sql:  "COPY (SELECT c1 FROM t1) TO '/tmp/x.csv'",
			want: []testutil.Touch{{Op: core.InspectOpSelect, Schema: s, Name: "t1"}},
		},
		{
			name: "DELETE ... USING reads the relation it joins",
			sql:  "DELETE FROM t2 USING t1 WHERE t2.c1 = t1.c1",
			want: []testutil.Touch{
				{Op: core.InspectOpDelete, Schema: s, Name: "t2"},
				{Op: core.InspectOpSelect, Schema: s, Name: "t1"},
			},
		},
		{
			name: "an ordinary CTE still reads its table",
			sql:  "WITH x AS (SELECT c1 FROM t1) SELECT c1 FROM x",
			want: []testutil.Touch{{Op: core.InspectOpSelect, Schema: s, Name: "t1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmts := inspectNested(t, tt.sql)
			for _, want := range tt.want {
				if !testutil.Touches(stmts, want) {
					t.Errorf("no %s on %s.%s anywhere in %+v", want.Op, want.Schema, want.Name, stmts)
				}
			}
		})
	}
}

// The outer statement maps CTE names to their bodies by position, so a body the
// inspector skips shifts every later CTE onto the wrong one.
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
