package query

import (
	"errors"
	"strings"
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/dialects"
)

var permDialects = []string{"postgresql", "mysql", "sqlite"}

func dialectOrFail(t *testing.T, name string) core.SQLDialect {
	t.Helper()
	dialect := dialects.Get(name)
	if dialect == nil {
		t.Fatalf("no dialect named %q", name)
	}
	return dialect
}

// TestPermissionCases checks every dialect against the shared table of rights.
// It measures through Inspect rather than a dialect's own, since the floor on
// an unread statement is part of what decides the verdict.
func TestPermissionCases(t *testing.T) {
	meta := core.GetInspectTestMetadata()
	for _, name := range permDialects {
		dialect := dialectOrFail(t, name)
		t.Run(name, func(t *testing.T) {
			testutil.RunPermCases(t, func(sql string) []core.InspectStatement {
				return Inspect(dialect, &meta, sql)
			}, testutil.PermCasesFor(name))
		})
	}
}

// TestUnknownNameInUndescribedSchema pins the one case that still refuses an
// unqualified name the metadata is missing: the session's own schema is one the
// metadata does not describe, so the name may live in it and nothing here can
// say what it holds. The refusal has to name the table rather than a right on
// an empty schema, which no entry can carry and no administrator can grant.
func TestUnknownNameInUndescribedSchema(t *testing.T) {
	meta := core.GetInspectTestMetadata()
	meta.CurrentSchema = "undescribed"

	for _, name := range permDialects {
		dialect := dialectOrFail(t, name)
		t.Run(name, func(t *testing.T) {
			for _, sql := range []string{
				"SELECT c1 FROM t9",
				"UPDATE t9 SET c1 = 1",
				"DELETE FROM t9",
				"INSERT INTO t9 (c1) VALUES (1)",
				"CREATE TABLE t9 (c1 INTEGER)",
				"DROP TABLE t9",
			} {
				statements := Inspect(dialect, &meta, sql)
				err := core.CheckQueryPermissions(statements, testutil.TestDatasourceID, testutil.PermGranting(
					testutil.Right{Action: core.ActionSelect, Schema: "undescribed", Table: "t9"},
					testutil.Right{Action: core.ActionUpdate, Schema: "undescribed", Table: "t9"},
					testutil.Right{Action: core.ActionDelete, Schema: "undescribed", Table: "t9"},
				))
				if err == nil {
					t.Errorf("ran against a schema the metadata does not describe:\n  %s", sql)
					continue
				}
				if !strings.Contains(err.Error(), `"t9"`) {
					t.Errorf("refusal does not name the table: %v\n  %s", err, sql)
				}
			}
		})
	}
}

// TestNoRightNamesAnEmptySchema is the standing guard behind case 5 of issue
// #335: a refusal an administrator cannot act on is not a refusal. Every case
// of the shared table is checked holding nothing, so each one reaches its
// refusal.
func TestNoRightNamesAnEmptySchema(t *testing.T) {
	meta := core.GetInspectTestMetadata()
	for _, name := range permDialects {
		dialect := dialectOrFail(t, name)
		t.Run(name, func(t *testing.T) {
			for _, testCase := range testutil.PermCasesFor(name) {
				statements := Inspect(dialect, &meta, testCase.SQL)
				err := core.CheckQueryPermissions(statements, testutil.TestDatasourceID, testutil.PermGranting())
				if err == nil {
					continue
				}
				var denied *core.PermissionDeniedError
				if errors.As(err, &denied) && denied.Table != "" && denied.Schema == "" {
					t.Errorf("refused with a right on an empty schema, which nobody can grant: %v\n  %s", err, testCase.SQL)
				}
			}
		})
	}
}
