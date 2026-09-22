package mysql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
)

// TestCompletionContext covers the completion context MySQL spells its own way:
// backtick quoted identifiers, and @@ for a system variable. The cases every
// dialect shares live in core/references.
//
// These all failed while the analyzer tokenized with sqlglot's generic
// tokenizer, which reads a backtick as an unknown character and loses the
// identifier it quotes.
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
			name: "quoted column takes operators",
			sql:  "SELECT * FROM t1 WHERE `c1` |",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetOperator},
		},
		{
			name: "quoted column takes enum values",
			sql:  "SELECT * FROM t1 WHERE `c1` = '|'",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetEnumValue},
		},
		{
			name: "quoted table qualifies a column",
			sql:  "SELECT `t1`.| FROM t1",
			want: testutil.CompletionContextWant{
				Targets:       core.CompletionTargetColumn,
				Parts:         []string{"t1"},
				CaretAfterDot: true,
				TargetTable:   "t1",
			},
		},
		{
			name: "system variable",
			sql:  "SELECT @@|",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetSetting},
		},
		{
			name: "system variable partially typed",
			sql:  "SELECT @@vers|",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetSetting},
		},
		{
			name: "a single @ is a user variable, not a setting",
			sql:  "SELECT @my|",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetAll | core.CompletionTargetFunction | core.CompletionTargetKeyword},
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
