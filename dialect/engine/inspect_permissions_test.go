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
