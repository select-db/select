package engine

import (
	"strings"
	"testing"

	"github.com/selectDb/dialect/core"
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
	var entries []core.PermissionEntry
	for _, a := range []string{core.ActionSelect, core.ActionInsert, core.ActionUpdate, core.ActionDelete} {
		entries = append(entries, core.PermissionEntry{Action: a, Effect: "allow", RoleName: "analyst"})
	}
	return compileFor(permDBID, entries...)
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
	for _, dialect := range []string{"postgresql", "mysql", "sqlite"} {
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
	for _, dialect := range []string{"postgresql", "mysql", "sqlite"} {
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

// holding is a role with exactly the actions named, on every table.
func holding(actions ...string) core.CompiledPermissions {
	var entries []core.PermissionEntry
	for _, a := range actions {
		entries = append(entries, core.PermissionEntry{Action: a, Effect: "allow", RoleName: "test-role"})
	}
	return compileFor(permDBID, entries...)
}

// nestedCase is a statement carrying another statement, and the permissions a
// role must hold for it to run: anything less is refused, all of them run.
type nestedCase struct {
	dialect string
	sql     string
	needs   []string
	why     string
}

// TestPermissions_NestedStatementsAreChecked pins the statements that carry
// another statement inside them. A nested statement the inspector did not
// report is a statement nobody checked, and the outer one being refused is no
// help when the outer one is the part that passes.
func TestPermissions_NestedStatementsAreChecked(t *testing.T) {
	tests := []nestedCase{
		{
			dialect: "postgresql",
			sql:     "WITH x AS (DELETE FROM t1 RETURNING c1) SELECT c1 FROM x",
			needs:   []string{core.ActionSelect, core.ActionDelete},
			why:     "the CTE deletes; it used to run under a policy granting nothing at all",
		},
		{
			dialect: "postgresql",
			sql:     "WITH x AS (UPDATE t1 SET c2 = 'x' RETURNING c1) SELECT c1 FROM x",
			needs:   []string{core.ActionSelect, core.ActionUpdate},
			why:     "same shape, an update",
		},
		{
			dialect: "postgresql",
			sql:     "WITH x AS (INSERT INTO t1 (c1) VALUES (1) RETURNING c1) SELECT c1 FROM x",
			needs:   []string{core.ActionSelect, core.ActionInsert},
			why:     "same shape, an insert",
		},
		{
			dialect: "postgresql",
			sql:     "UPDATE t2 SET c1 = t1.c1 FROM t1",
			needs:   []string{core.ActionUpdate, core.ActionSelect},
			why:     "t1 is read to fill t2, and update does not cover reading it",
		},
		{
			dialect: "postgresql",
			sql:     "DELETE FROM t2 USING t1 WHERE t2.c1 = t1.c1",
			needs:   []string{core.ActionDelete, core.ActionSelect},
			why:     "USING reads t1 the same way",
		},
		{
			dialect: "postgresql",
			sql:     "COPY (SELECT c1 FROM t1) TO '/tmp/x.csv'",
			needs:   []string{core.ActionManage, core.ActionSelect},
			why:     "writing the server's filesystem takes manage, reading t1 takes select",
		},
		{
			dialect: "postgresql",
			sql:     "CREATE MATERIALIZED VIEW mv AS SELECT c1 FROM t1",
			needs:   []string{core.ActionManage, core.ActionSelect},
			why:     "a materialized view holds the rows it read",
		},
	}

	for _, dialect := range []string{"postgresql", "mysql", "sqlite"} {
		for _, sql := range []string{
			"CREATE TABLE t9 AS SELECT c1 FROM t1",
			"CREATE VIEW v AS SELECT c1 FROM t1",
		} {
			tests = append(tests, nestedCase{
				dialect: dialect,
				sql:     sql,
				needs:   []string{core.ActionManage, core.ActionSelect},
				why:     "creating it takes manage, and the query it is filled from still reads t1",
			})
		}
	}

	for _, tt := range tests {
		t.Run(tt.dialect+": "+tt.sql, func(t *testing.T) {
			inspected := Inspect(GetDialect(tt.dialect), permMeta(), tt.sql)
			if len(inspected) == 0 {
				t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
			}

			// Every action but one: each is necessary, so each is refused.
			for _, missing := range tt.needs {
				var rest []string
				for _, a := range tt.needs {
					if a != missing {
						rest = append(rest, a)
					}
				}
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
	for _, sql := range []string{
		"WITH x AS (SELECT c1 FROM t1) UPDATE t2 SET c1 = x.c1 FROM x",
		"WITH x AS (SELECT c1 FROM t1) DELETE FROM t2 USING x WHERE t2.c1 = x.c1",
		"UPDATE t2 SET c1 = s.c1 FROM (SELECT c1 FROM t1) s",
		"UPDATE t2 SET c1 = a.c1 FROM t1 a WHERE t2.c1 = a.c1",
	} {
		t.Run(sql, func(t *testing.T) {
			inspected := Inspect(GetDialect("postgresql"), permMeta(), sql)
			if err := core.CheckQueryPermissions(inspected, permDBID, perms); err != nil {
				t.Errorf("want allowed, got %v", err)
			}
		})
	}
}
