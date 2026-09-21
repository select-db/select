package mysql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

// MySQL spells CREATE TABLE ... SELECT with the AS optional, and the two
// spellings reach different grammar nodes. The AS form is pinned by permission
// outcome in dialect/engine, which runs it for every dialect.
func TestInspectCreateTableSelectWithoutAS(t *testing.T) {
	const s = "main"
	stmts := NewInspector(NewDialect(), core.GetInspectTestMetadata()).
		Inspect("CREATE TABLE t9 SELECT c1 FROM t1")

	for _, want := range []testutil.Touch{
		{Op: core.InspectOpCreate, Schema: s, Name: "t9"},
		{Op: core.InspectOpSelect, Schema: s, Name: "t1"},
	} {
		if !testutil.Touches(stmts, want) {
			t.Errorf("no %s on %s.%s anywhere in %+v", want.Op, want.Schema, want.Name, stmts)
		}
	}
}

// TABLE t1 is SELECT * FROM t1, and the see check hides a column by finding the
// field that read it. Reporting the table without its columns leaves it nothing
// to hide.
func TestInspectTableShorthandReportsItsColumns(t *testing.T) {
	stmts := NewInspector(NewDialect(), core.GetInspectTestMetadata()).Inspect("TABLE t1")
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
