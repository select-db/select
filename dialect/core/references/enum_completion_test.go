package core_references_test

import (
	"context"
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/postgresql"
)

func enumCompletionMeta() core.Metadata {
	return core.Metadata{
		DefaultSchema: "public",
		Schemas: []core.Schema{
			{
				Name: "public",
				Tables: []core.Table{
					{
						Name: "users",
						Columns: []core.Column{
							{Name: "id", Type: "integer"},
							{Name: "name", Type: "text"},
							{Name: "status", Type: "user_status", EnumValues: []string{"active", "inactive", "banned"}},
						},
					},
				},
			},
		},
	}
}

func enumTexts(cands []core.Candidate) []string {
	var out []string
	for _, c := range cands {
		if c.Type == core.CandidateTypeEnumValue {
			out = append(out, c.Text)
		}
	}
	return out
}

func TestEnumValueCompletion(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	d := postgresql.NewDialect()
	d.SetAnalyzer(analyzer)
	meta := enumCompletionMeta()

	cases := []struct {
		name     string
		sql      string // | marks the caret
		wantEnum bool
	}{
		{"equality enum", "SELECT * FROM users WHERE status = '|'", true},
		{"partial enum value", "SELECT * FROM users WHERE status = 'ac|'", true},
		{"not-equal enum", "SELECT * FROM users WHERE status <> '|'", true},
		{"in list enum", "SELECT * FROM users WHERE status IN ('|')", true},
		{"in list second value", "SELECT * FROM users WHERE status IN ('active', '|')", true},
		{"qualified enum", "SELECT * FROM users u WHERE u.status = '|'", true},
		{"no quotes yet", "SELECT * FROM users WHERE status = |", true},
		{"non-enum column", "SELECT * FROM users WHERE name = '|'", false},
		{"non-enum integer", "SELECT * FROM users WHERE id = |", false},
		{"operator position not value", "SELECT * FROM users WHERE status |", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, caretCharPos := removeCaret(tc.sql)
			caretLine, caretOffset := charPosToLineCol(text, caretCharPos)

			got, err := d.Complete(context.Background(), text, caretLine, caretOffset, meta)
			if err != nil {
				t.Fatalf("complete: %v", err)
			}

			enums := enumTexts(got)
			if tc.wantEnum {
				want := map[string]bool{"active": false, "inactive": false, "banned": false}
				for _, e := range enums {
					if _, ok := want[e]; !ok {
						t.Errorf("unexpected enum candidate %q", e)
					}
					want[e] = true
				}
				for v, seen := range want {
					if !seen {
						t.Errorf("missing enum candidate %q (got %v)", v, enums)
					}
				}
				for _, c := range got {
					if c.Type == core.CandidateTypeEnumValue && c.InsertText != "'"+c.Text+"'" {
						t.Errorf("enum %q InsertText = %q, want quoted", c.Text, c.InsertText)
					}
				}
			} else if len(enums) > 0 {
				t.Errorf("expected no enum candidates, got %v", enums)
			}
		})
	}
}
