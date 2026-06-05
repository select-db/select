package mysql

import (
	"context"
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
)

func catchCaret(s string) (string, int) {
	for i, c := range s {
		if c == '|' {
			return s[:i] + s[i+1:], i
		}
	}
	return s, -1
}

func TestOperatorCompletion(t *testing.T) {
	analyzer := testutil.NewTestAnalyzer(t)
	defer analyzer.Close()

	d := NewDialect()
	d.SetAnalyzer(analyzer)
	meta := core.Metadata{
		DefaultSchema: "main",
		Schemas: []core.Schema{
			{
				Name: "main",
				Tables: []core.Table{
					{
						Name: "users",
						Columns: []core.Column{
							{Name: "id", Type: "INT"},
							{Name: "user_id", Type: "BIGINT UNSIGNED"},
							{Name: "full_name", Type: "TEXT"},
							{Name: "email", Type: "VARCHAR(255)"},
							{Name: "code", Type: "CHAR(10)"},
							{Name: "balance", Type: "DECIMAL(10,2)"},
							{Name: "score", Type: "NUMERIC(8,3)"},
							{Name: "rating", Type: "FLOAT"},
							{Name: "amount", Type: "DOUBLE"},
							{Name: "is_active", Type: "TINYINT(1)"},
							{Name: "tags", Type: "JSON"},
							{Name: "created_at", Type: "DATETIME"},
							{Name: "birth_date", Type: "DATE"},
							{Name: "start_time", Type: "TIME"},
							{Name: "year_val", Type: "YEAR"},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		sql      string
		contains []string
		excludes []string
	}{
		{
			name:     "INT column - all operators",
			sql:      "SELECT * FROM users WHERE id |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN", "<", ">", "<=", ">=", "BETWEEN", "<=>"},
			excludes: []string{"LIKE", "REGEXP"},
		},
		{
			name:     "BIGINT UNSIGNED column",
			sql:      "SELECT * FROM users WHERE user_id |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN", "<=>"},
		},
		{
			name:     "DECIMAL column",
			sql:      "SELECT * FROM users WHERE balance |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
			excludes: []string{"LIKE"},
		},
		{
			name:     "TEXT column - string operators",
			sql:      "SELECT * FROM users WHERE full_name |",
			contains: []string{"=", "<>", "LIKE", "NOT LIKE", "REGEXP", "RLIKE", "SOUNDS LIKE"},
			excludes: []string{"ILIKE", "~"},
		},
		{
			name:     "VARCHAR column",
			sql:      "SELECT * FROM users WHERE email |",
			contains: []string{"=", "<>", "LIKE", "REGEXP"},
		},
		{
			name:     "CHAR column",
			sql:      "SELECT * FROM users WHERE code |",
			contains: []string{"=", "LIKE", "REGEXP"},
		},
		{
			name:     "TINYINT(1) acts as BOOLEAN",
			sql:      "SELECT * FROM users WHERE is_active |",
			contains: []string{"IS TRUE", "IS FALSE", "IS NOT TRUE", "IS NOT FALSE"},
			excludes: []string{"LIKE", "<", "BETWEEN"},
		},
		{
			name:     "JSON column",
			sql:      "SELECT * FROM users WHERE tags |",
			contains: []string{"=", "->", "->>", "JSON_CONTAINS", "JSON_OVERLAPS"},
			excludes: []string{"~", "ILIKE"},
		},
		{
			name:     "DATETIME column",
			sql:      "SELECT * FROM users WHERE created_at |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
			excludes: []string{"LIKE"},
		},
		{
			name:     "DATE column",
			sql:      "SELECT * FROM users WHERE birth_date |",
			contains: []string{"=", "<", ">", "BETWEEN"},
		},
		{
			name:     "YEAR column",
			sql:      "SELECT * FROM users WHERE year_val |",
			contains: []string{"=", "<", ">", "BETWEEN"},
		},
		{
			name:     "operator with table alias",
			sql:      "SELECT * FROM users u WHERE u.full_name |",
			contains: []string{"=", "<>", "LIKE", "REGEXP"},
		},
		{
			name:     "operator with backtick-quoted column",
			sql:      "SELECT * FROM users u WHERE u.`full_name` |",
			contains: []string{"=", "LIKE", "REGEXP"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, caretCharPos := catchCaret(tc.sql)
			lines := strings.Split(text, "\n")
			caretLine := 1
			caretOffset := caretCharPos
			for i, line := range lines {
				if caretOffset <= len(line) {
					caretLine = i + 1
					break
				}
				caretOffset -= len(line) + 1
			}

			got, err := d.Complete(context.Background(), text, caretLine, caretOffset, meta)
			if err != nil {
				t.Fatalf("complete: %v", err)
			}

			gotOps := make(map[string]bool)
			for _, c := range got {
				if c.Type == core.CandidateTypeOperator {
					gotOps[c.Text] = true
				}
			}

			for _, op := range tc.contains {
				if !gotOps[op] {
					t.Errorf("Missing expected operator: %s", op)
				}
			}
			for _, op := range tc.excludes {
				if gotOps[op] {
					t.Errorf("Unexpected operator present: %s", op)
				}
			}
		})
	}
}
