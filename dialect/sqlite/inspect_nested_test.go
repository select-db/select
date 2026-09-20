package sqlite

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

// SQLite's UPDATE ... FROM reads the relations it joins against. The CREATE
// cases this dialect also nests are pinned by permission outcome in
// dialect/engine, which runs them for every dialect.
func TestInspectUpdateFromReadsItsRelations(t *testing.T) {
	const s = "main"

	tests := []struct {
		name string
		sql  string
		want []testutil.Touch
	}{
		{
			name: "a single relation",
			sql:  "UPDATE t2 SET c1 = t1.c1 FROM t1",
			want: []testutil.Touch{
				{Op: core.InspectOpUpdate, Schema: s, Name: "t2"},
				{Op: core.InspectOpSelect, Schema: s, Name: "t1"},
			},
		},
		{
			name: "a comma list",
			sql:  "UPDATE t2 SET c1 = t1.c1 FROM t1, other.t3",
			want: []testutil.Touch{
				{Op: core.InspectOpSelect, Schema: s, Name: "t1"},
				{Op: core.InspectOpSelect, Schema: "other", Name: "t3"},
			},
		},
		{
			name: "a join, which hangs off a different grammar node than the comma list",
			sql:  "UPDATE t2 SET c1 = t1.c1 FROM t1 JOIN other.t3 ON t1.c1 = t3.c1",
			want: []testutil.Touch{
				{Op: core.InspectOpSelect, Schema: s, Name: "t1"},
				{Op: core.InspectOpSelect, Schema: "other", Name: "t3"},
			},
		},
		{
			name: "a subquery",
			sql:  "UPDATE t2 SET c1 = s.c1 FROM (SELECT c1 FROM t1) s",
			want: []testutil.Touch{{Op: core.InspectOpSelect, Schema: s, Name: "t1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmts := NewInspector(NewDialect(), core.GetInspectTestMetadata()).Inspect(tt.sql)
			for _, want := range tt.want {
				if !testutil.Touches(stmts, want) {
					t.Errorf("no %s on %s.%s anywhere in %+v", want.Op, want.Schema, want.Name, stmts)
				}
			}
		})
	}
}

// An UPDATE with no FROM names only the table it writes.
func TestInspectUpdateWithoutFromReadsNothing(t *testing.T) {
	stmts := NewInspector(NewDialect(), core.GetInspectTestMetadata()).Inspect("UPDATE t2 SET c1 = 1")
	if len(stmts) != 1 {
		t.Fatalf("got %d statements, want 1", len(stmts))
	}
	if len(stmts[0].Subqueries) != 0 {
		t.Errorf("got %d subqueries, want none: %+v", len(stmts[0].Subqueries), stmts[0].Subqueries)
	}
}

// SQLite takes a subquery in LIMIT and in the OFFSET that follows it, which
// MySQL does not, so the clauses every dialect shares are pinned by permission
// outcome in dialect/engine and only these two live here.
func TestInspectClauseSubqueriesReachTheResult(t *testing.T) {
	read := testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t2"}

	for _, tt := range []struct {
		clause string
		sql    string
	}{
		{"LIMIT", "SELECT c1 FROM t1 LIMIT (SELECT count(*) FROM t2)"},
		{"OFFSET", "SELECT c1 FROM t1 LIMIT 5 OFFSET (SELECT count(*) FROM t2)"},
	} {
		t.Run(tt.clause, func(t *testing.T) {
			stmts := NewInspector(NewDialect(), core.GetInspectTestMetadata()).Inspect(tt.sql)
			if !testutil.Touches(stmts, read) {
				t.Errorf("no read of main.t2 anywhere in %+v", stmts)
			}
		})
	}
}
