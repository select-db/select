package query

import (
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/dialects"
)

const permDatasourceID = testutil.TestDatasourceID

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
	return testutil.PermGranting(
		testutil.Right{Action: core.ActionSelect},
		testutil.Right{Action: core.ActionInsert},
		testutil.Right{Action: core.ActionUpdate},
		testutil.Right{Action: core.ActionDelete},
	)
}

// Blank input is not a statement, so it stays empty rather than becoming an
// unknown one that a caller would then have to refuse.
func TestInspect_BlankSQLStaysEmpty(t *testing.T) {
	for _, dialect := range dialects.Builtin() {
		t.Run(dialect, func(t *testing.T) {
			for _, sql := range []string{"", "   ", "\n\t "} {
				if got := Inspect(dialects.Get(dialect), permMeta(), sql); len(got) != 0 {
					t.Errorf("Inspect(%q) = %d statements, want 0", sql, len(got))
				}
			}
		})
	}
}

// A dialect registered from outside this module is not one this package can
// audit, so query.Inspect puts a floor under an empty result rather than
// trusting every implementation to have read the contract.
func TestInspect_ForeignDialectCannotReturnNothing(t *testing.T) {
	dialects.Register("silent-test-dialect", silentDialect{})
	t.Cleanup(func() { dialects.Register("silent-test-dialect", nil) })

	got := Inspect(dialects.Get("silent-test-dialect"), permMeta(), "DROP TABLE t1")
	if len(got) != 1 || got[0].Operation != core.InspectOpUnknown {
		t.Fatalf("got %+v, want one unknown statement", got)
	}
	if err := core.CheckQueryPermissions(got, permDatasourceID, dataActionsOnly()); err == nil {
		t.Error("a dialect that inspects to nothing ran unchecked")
	}
}

// silentDialect inspects to nothing, the way a dialect written against the old
// contract does.
type silentDialect struct {
	core.SQLDialect
}

func (silentDialect) Inspect(core.Metadata, string) []core.InspectStatement { return nil }

// The same collision written with a subquery alias rather than a CTE. What each
// dialect reports for the subquery itself differs, so only the qualified read
// is pinned; the grants it needs are pinned by the CTE cases in perm_data.go.
func TestInspect_AQualifiedNameSurvivesAnAliasOfTheSameName(t *testing.T) {
	want := testutil.Touch{Op: core.InspectOpSelect, Schema: "other", Name: "t3"}

	for _, sql := range []string{
		"SELECT * FROM (SELECT c1 FROM t1) t3 JOIN other.t3 ON 1=1",
		"SELECT other.t3.c4 FROM (SELECT c1 FROM t1) t3, other.t3",
	} {
		for _, dialect := range dialects.Builtin() {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				inspected := Inspect(dialects.Get(dialect), permMeta(), sql)
				if !testutil.Touches(inspected, want) {
					t.Errorf("no read of other.t3 anywhere in %+v", inspected)
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
// SQLite owns this spelling and resolves it, so its cases are in perm_data.go:
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
					inspected = Inspect(dialects.Get(dialect), permMeta(), sql)
				}()
				if len(inspected) == 0 {
					t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
				}
				if err := core.CheckQueryPermissions(inspected, permDatasourceID, nothing); err == nil {
					t.Error("ran on a policy granting nothing")
				}
			})
		}
	}
}

// TestPermissions_ATruncatedWriteNeverRunsUnchecked pins the writes that name
// no table because the statement stops before naming one. checkTables iterates
// the tables a statement names, so a write naming none is checked against
// nothing unless the inspector refuses it outright. Some of these also reach a
// node error recovery leaves incomplete, where a dereference fails the request
// rather than refusing the statement.
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
		for _, dialect := range dialects.Builtin() {
			t.Run(dialect+": "+sql, func(t *testing.T) {
				var inspected []core.InspectStatement
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("inspecting panicked: %v", r)
						}
					}()
					inspected = Inspect(dialects.Get(dialect), permMeta(), sql)
				}()
				if len(inspected) == 0 {
					t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
				}
				if err := core.CheckQueryPermissions(inspected, permDatasourceID, nothing); err == nil {
					t.Error("ran on a policy granting nothing")
				}
			})
		}
	}
}

// dialectSQL is one statement and the dialect it is written in.
type dialectSQL struct {
	dialect string
	sql     string
}

// Text the parser stumbled over that no reported statement covers is SQL the
// caller will run and we never checked.
func TestPermissions_TextNoStatementCoversIsReported(t *testing.T) {
	for _, tt := range []dialectSQL{
		{"postgresql", "SELECT c1 FROM t1; ]]] not sql"},
		{"mysql", "SELECT c1 FROM t1; ]]] not sql"},
		{"sqlite", "SELECT c1 FROM t1; ]]] not sql"},
	} {
		t.Run(tt.dialect, func(t *testing.T) {
			inspected := Inspect(dialects.Get(tt.dialect), permMeta(), tt.sql)
			if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t1"}) {
				t.Fatalf("no read of main.t1, so nothing here was checked: %+v", inspected)
			}
			unknown := false
			for _, stmt := range inspected {
				if stmt.Operation == core.InspectOpUnknown {
					unknown = true
				}
			}
			if !unknown {
				t.Errorf("the text after the select went unreported: %+v", inspected)
			}
		})
	}
}

// A CTE shadows a table spelled the same way, so a star over it reads the CTE
// and nothing else. Reporting the table's columns too refuses the statement to
// a role that may read everything the statement actually touches.
func TestPermissions_ACTEShadowsTheTableItIsNamedAfter(t *testing.T) {
	schema, table, db := "main", "t1", permDatasourceID
	onlyT1 := core.Compile([]core.PermissionEntry{{
		DatasourceID: &db, SchemaName: &schema, TableName: &table,
		Action: core.ActionSelect, Effect: "allow", RoleName: "r",
	}}).WithDenyUnmanaged()

	for _, tt := range []dialectSQL{
		{"postgresql", "WITH t2 AS (SELECT c1 FROM t1) SELECT * FROM t2"},
		{"mysql", "WITH t2 AS (SELECT c1 FROM t1) SELECT * FROM t2"},
		{"sqlite", "WITH t2 AS (SELECT c1 FROM t1) SELECT * FROM t2"},
	} {
		t.Run(tt.dialect, func(t *testing.T) {
			inspected := Inspect(dialects.Get(tt.dialect), permMeta(), tt.sql)
			if !testutil.Touches(inspected, testutil.Touch{Op: core.InspectOpSelect, Schema: "main", Name: "t1"}) {
				t.Fatalf("no read of main.t1, so nothing here was checked: %+v", inspected)
			}
			for _, stmt := range inspected {
				for _, f := range stmt.Fields {
					if f.Table == "t2" {
						t.Errorf("read a column of the table the CTE shadows: %+v", stmt.Fields)
					}
				}
			}
			if err := core.CheckQueryPermissions(inspected, db, onlyT1); err != nil {
				t.Errorf("refused holding select on every table it reads: %v", err)
			}
		})
	}
}

// Calling such a routine in one branch of a compound select classifies the
// whole of it, since the branches are one statement.
func TestPermissions_ACompoundSelectIsOneStatement(t *testing.T) {
	sql := "SELECT c1 FROM t1 UNION SELECT readfile('/etc/passwd')"
	inspected := Inspect(dialects.Get("sqlite"), permMeta(), sql)
	if len(inspected) == 0 {
		t.Fatal("inspected to nothing: a caller reading this as an empty result runs it unchecked")
	}
	for _, stmt := range inspected {
		if stmt.Operation != core.InspectOpUnknown {
			t.Errorf("a branch of it reads as %s: %+v", stmt.Operation, inspected)
		}
	}
	if err := core.CheckQueryPermissions(inspected, permDatasourceID, dataActionsOnly()); err == nil {
		t.Error("ran on the four row actions")
	}
}

// One statement calling such a routine must not drag the rest of a script with
// it, nor be excused by them.
func TestPermissions_AScriptIsClassifiedStatementByStatement(t *testing.T) {
	dataActions := dataActionsOnly()

	for _, tt := range []dialectSQL{
		{"postgresql", "SELECT pg_read_file('/etc/passwd'); SELECT c1 FROM t1"},
		{"mysql", "SELECT LOAD_FILE('/etc/passwd'); SELECT c1 FROM t1"},
		{"sqlite", "SELECT readfile('/etc/passwd'); SELECT c1 FROM t1"},
		{"sqlite", "SELECT c1 FROM t1; SELECT readfile('/etc/passwd')"},
	} {
		t.Run(tt.dialect+": "+tt.sql, func(t *testing.T) {
			inspected := Inspect(dialects.Get(tt.dialect), permMeta(), tt.sql)
			unknown := 0
			for _, stmt := range inspected {
				if stmt.Operation == core.InspectOpUnknown {
					unknown++
				}
			}
			if unknown != 1 {
				t.Errorf("%d statements need manage, want 1: %+v", unknown, inspected)
			}
			if err := core.CheckQueryPermissions(inspected, permDatasourceID, dataActions); err == nil {
				t.Error("the calling statement ran on the four row actions")
			}
		})
	}
}

// TestPermissions_AColumnScopedRoleStillUpserts pins the direction the second
// right a statement asks for can break. An upsert needs update as well as
// insert, and asking for it on the whole table would refuse a role granted
// update on exactly the columns the upsert sets.
func TestPermissions_AColumnScopedRoleStillUpserts(t *testing.T) {
	column := "c2"
	perms := compileFor(permDatasourceID,
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("t1"),
			Action: core.ActionInsert, Effect: "allow", RoleName: "test-role"},
		core.PermissionEntry{SchemaName: sptr("main"), TableName: sptr("t1"), ColumnName: &column,
			Action: core.ActionUpdate, Effect: "allow", RoleName: "test-role"},
	).WithDenyUnmanaged()

	for _, name := range []string{"postgresql", "sqlite"} {
		t.Run(name, func(t *testing.T) {
			sql := "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO UPDATE SET c2 = 'x'"
			if err := core.CheckQueryPermissions(Inspect(dialects.Get(name), permMeta(), sql), permDatasourceID, perms); err != nil {
				t.Errorf("refused a role holding update on the column it sets: %v", err)
			}
			other := "INSERT INTO t1 (c1) VALUES (1) ON CONFLICT (c1) DO UPDATE SET c1 = 2"
			if err := core.CheckQueryPermissions(Inspect(dialects.Get(name), permMeta(), other), permDatasourceID, perms); err == nil {
				t.Error("ran, and it sets a column the role may not update")
			}
		})
	}
}
