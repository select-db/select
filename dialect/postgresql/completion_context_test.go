package postgresql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	coreRefs "github.com/selectDb/dialect/core/references"
	"github.com/selectDb/dialect/core/testutil"
)

// TestCompletionContext covers the completion context PostgreSQL spells its own
// way: double quoted identifiers, and SHOW for a runtime parameter. The cases
// every dialect shares live in core/references.
func TestCompletionContext(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	dialect := NewDialect()
	meta := core.GetCompletionTestMetadata()
	meta.DefaultSchema = "public"
	if len(meta.Schemas) > 0 {
		meta.Schemas[0].Name = "public"
	}

	tests := []struct {
		name string
		sql  string
		want testutil.CompletionContextWant
	}{
		{
			name: "quoted column takes operators",
			sql:  "SELECT * FROM t1 WHERE \"c1\" |",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetOperator},
		},
		{
			name: "quoted column takes enum values",
			sql:  "SELECT * FROM t1 WHERE \"c1\" = '|'",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetEnumValue},
		},
		{
			name: "quoted table qualifies a column",
			sql:  "SELECT \"t1\".| FROM t1",
			want: testutil.CompletionContextWant{
				Targets:       core.CompletionTargetColumn,
				Parts:         []string{"t1"},
				CaretAfterDot: true,
				TargetTable:   "t1",
			},
		},
		{
			name: "show",
			sql:  "SHOW |",
			want: testutil.CompletionContextWant{Targets: core.CompletionTargetSetting},
		},
		{
			name: "show partially typed",
			sql:  "SHOW time|",
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
