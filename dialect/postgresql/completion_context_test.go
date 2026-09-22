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

// TestKeywordGroupWords pins the words themselves for one dialect. The shared
// cases pin which group each caret takes; this pins what a group holds, so a
// word added to the wrong group fails somewhere.
func TestKeywordGroupWords(t *testing.T) {
	d := NewDialect()
	cases := []struct {
		group string
		want  []string
	}{
		{"select_item", []string{"FROM", "AS", "UNION", "EXCEPT", "INTERSECT"}},
		{"sort_item", []string{"LIMIT", "OFFSET", "FETCH", "ASC", "DESC"}},
		{"group_item", []string{"ORDER BY", "HAVING", "LIMIT", "OFFSET"}},
		{"row_count", []string{"OFFSET", "FETCH"}},
		{"after_cte", []string{"SELECT", "INSERT", "UPDATE", "DELETE"}},
		{"aliased_select_item", []string{"FROM", "UNION", "EXCEPT", "INTERSECT"}},
		{"join_word", []string{"JOIN"}},
		{"is_test", []string{"NULL", "NOT", "TRUE", "FALSE"}},
		{"not_test", []string{"NULL", "IN", "LIKE", "ILIKE", "BETWEEN", "EXISTS"}},
		{"set_operand", []string{"SELECT", "ALL", "DISTINCT", "VALUES"}},
		{"query_word", []string{"SELECT", "VALUES"}},
		{"insert_target", []string{"VALUES", "SELECT", "AS"}},
		{"update_target", []string{"SET", "AS"}},
		{"delete_target", []string{"FROM"}},
		{"case_test", []string{"THEN"}},
		{"case_body", []string{"WHEN", "ELSE", "END"}},
	}
	for _, tc := range cases {
		t.Run(tc.group, func(t *testing.T) {
			got := core.KeywordsOfGroup(d, tc.group)
			if len(got) != len(tc.want) {
				t.Fatalf("%s = %v, want %v", tc.group, got, tc.want)
			}
			seen := make(map[string]bool, len(got))
			for _, w := range got {
				seen[w] = true
			}
			for _, w := range tc.want {
				if !seen[w] {
					t.Errorf("%s is missing %q (got %v)", tc.group, w, got)
				}
			}
		})
	}
}
