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

// The clauses only PostgreSQL spells this way. A local PostgreSQL 16 runs a
// subquery in each; the clauses every dialect shares are pinned by permission
// outcome in dialect/engine.
func TestInspectClauseSubqueriesReachTheResult(t *testing.T) {
	read := testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t2"}

	for _, tt := range []struct {
		clause string
		sql    string
	}{
		{"DISTINCT ON", "SELECT DISTINCT ON ((SELECT c1 FROM t2)) c1 FROM t1"},
		{"LIMIT", "SELECT c1 FROM t1 LIMIT (SELECT count(*) FROM t2)"},
		{"OFFSET", "SELECT c1 FROM t1 OFFSET (SELECT count(*) FROM t2)"},
		{"FETCH FIRST", "SELECT c1 FROM t1 FETCH FIRST (SELECT count(*) FROM t2) ROWS ONLY"},
	} {
		t.Run(tt.clause, func(t *testing.T) {
			stmts := inspectNested(t, tt.sql)
			if !testutil.Touches(stmts, read) {
				t.Errorf("no read of main.t2 anywhere in %+v", stmts)
			}
		})
	}
}

// Classifying the INTO a statement carries must not cost the reads nested
// inside it. The listener that collects an embedded subquery keeps one only if
// it names a table, so a wrap around a nested query loses the branch and
// everything under it.
func TestInspectClassifyingIntoKeepsNestedReads(t *testing.T) {
	read := testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t2"}

	// Losing the nested read runs main.t2 on a grant covering main.t1 alone,
	// and losing the alias with it refuses the statement to every role, so both
	// directions are pinned.
	db, schema, t1, all := "db1", "main", "t1", "*"
	onlyT1 := core.Compile([]core.PermissionEntry{{
		DbInstanceID: &db, SchemaName: &schema, TableName: &t1,
		Action: core.ActionSelect, Effect: "allow", RoleName: "r",
	}}).WithDenyUnmanaged()

	var entries []core.PermissionEntry
	for _, a := range []string{core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete, core.ActionManage} {
		entries = append(entries, core.PermissionEntry{
			DbInstanceID: &db, SchemaName: &all,
			Action: a, Effect: "allow", RoleName: "r",
		})
	}
	everyAction := core.Compile(entries).WithDenyUnmanaged()

	for _, sql := range []string{
		"SELECT * FROM t1 WHERE c1 IN (SELECT c1 INTO t9 FROM t2)",
		"SELECT * FROM t1 ORDER BY (SELECT c1 INTO t9 FROM t2)",
	} {
		t.Run(sql, func(t *testing.T) {
			stmts := inspectNested(t, sql)
			if !testutil.Touches(stmts, read) {
				t.Fatalf("the nested read of main.t2 was lost: %+v", stmts)
			}

			if err := core.CheckQueryPermissions(stmts, db, onlyT1); err == nil {
				t.Error("read main.t2 holding select on main.t1 alone")
			}
			if err := core.CheckQueryPermissions(stmts, db, everyAction); err != nil {
				t.Errorf("holding every action still refused it: %v", err)
			}
		})
	}
}

// TABLE t1 is SELECT * FROM t1, and the see check hides a column by finding the
// field that read it. Reporting the table without its columns leaves it nothing
// to hide.
func TestInspectTableShorthandReportsItsColumns(t *testing.T) {
	stmts := inspectNested(t, "TABLE t1")
	if len(stmts) != 1 {
		t.Fatalf("expected one statement, got %+v", stmts)
	}
	if got := stmts[0].Operation; got != core.InspectOpSelect {
		t.Fatalf("operation is %s, want select: %+v", got, stmts[0])
	}
	for _, want := range []string{"c1", "c2"} {
		found := false
		for _, f := range stmts[0].Fields {
			if f.Schema == "main" && f.Table == "t1" && f.Name == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no field for main.t1.%s: %+v", want, stmts[0].Fields)
		}
	}
}
