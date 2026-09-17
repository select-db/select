package sqlite

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
)

// TestCompletionContext covers the completion context SQLite spells its own
// way: bracket quoted identifiers, and PRAGMA for a runtime parameter. The
// cases every dialect shares live in core/references.
//
// The bracket cases all failed while the analyzer tokenized with sqlglot's
// generic tokenizer, which has no bracket quoting and loses the identifier.
func TestCompletionContext(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	dialect := NewDialect()
	meta := core.GetCompletionTestMetadata()

	tests := []struct {
		name string
		sql  string
		want testutil.CompletionContextWant
	}{
		{
			name: "bracket quoted column takes operators",
			sql:  "SELECT * FROM t1 WHERE [c1] |",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetOperator},
		},
		{
			name: "bracket quoted column takes enum values",
			sql:  "SELECT * FROM t1 WHERE [c1] = '|'",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetEnumValue},
		},
		{
			name: "bracket quoted table qualifies a column",
			sql:  "SELECT [t1].| FROM t1",
			want: testutil.CompletionContextWant{
				Targets:       core.CompletionTargetColumn,
				Parts:         []string{"t1"},
				CaretAfterDot: true,
				TargetTable:   "t1",
			},
		},
		{
			// SQLite accepts the double quote for identifiers as well.
			name: "double quoted column takes operators",
			sql:  "SELECT * FROM t1 WHERE \"c1\" |",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetOperator},
		},
		{
			name: "pragma",
			sql:  "PRAGMA |",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetSetting},
		},
		{
			name: "pragma partially typed",
			sql:  "PRAGMA foreign|",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetSetting},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Every case here is one line, so the caret offset is the column.
			sql, caretCol := catchCaret(tt.sql)
			got, err := coreRefs.ParseCompletionContextFromPython(analyzer, sql, dialect, 1, caretCol, meta)
			if err != nil {
				t.Fatalf("complete_context: %v", err)
			}
			testutil.AssertCompletionContext(t, got, tt.want)
		})
	}
}
