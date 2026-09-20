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
