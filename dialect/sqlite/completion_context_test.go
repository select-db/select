package sqlite

import (
	"slices"
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

// TestWordsSQLiteWritesOnlyElsewhere pins the words SQLite has in one role and
// not in another, which a flat keyword list cannot say on its own.
func TestWordsSQLiteWritesOnlyElsewhere(t *testing.T) {
	d := NewDialect()
	cases := []struct {
		group  string
		absent string
		holder string
	}{
		{"statement", "SET", "update_target"},
		{"delete_relation", "USING", "joined_relation"},
		{"set_operand", "TABLE", "object_kind"},
		{"lock_strength", "UPDATE", "statement"},
		{"alter_action", "ALTER", "statement"},
		{"alter_action", "SET", "update_target"},
	}
	for _, tc := range cases {
		t.Run(tc.group+"/"+tc.absent, func(t *testing.T) {
			if slices.Contains(core.KeywordsOfGroup(d, tc.group), tc.absent) {
				t.Errorf("%s offers %q, which SQLite has no syntax for", tc.group, tc.absent)
			}
			if !slices.Contains(core.KeywordsOfGroup(d, tc.holder), tc.absent) {
				t.Errorf("%s no longer offers %q, so the word is gone entirely", tc.holder, tc.absent)
			}
		})
	}
}
