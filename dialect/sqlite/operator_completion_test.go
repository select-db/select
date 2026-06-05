package sqlite

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
							{Name: "id", Type: "INTEGER"},
							{Name: "name", Type: "TEXT"},
							{Name: "email", Type: "VARCHAR(255)"},
							{Name: "age", Type: "INTEGER"},
							{Name: "balance", Type: "REAL"},
							{Name: "score", Type: "NUMERIC"},
							{Name: "is_active", Type: "BOOLEAN"},
							{Name: "avatar", Type: "BLOB"},
							{Name: "created_at", Type: "DATETIME"},
							{Name: "birth_date", Type: "DATE"},
							{Name: "notes", Type: "CLOB"},
						},
					},
					{
						Name: "products",
						Columns: []core.Column{
							{Name: "id", Type: "INTEGER"},
							{Name: "price", Type: "DOUBLE"},
							{Name: "description", Type: "CHAR(100)"},
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
		// INTEGER column tests
		{
			name:     "INTEGER column - common operators",
			sql:      "SELECT * FROM users WHERE id |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN"},
		},
		{
			name:     "INTEGER column - comparison operators",
			sql:      "SELECT * FROM users WHERE age |",
			contains: []string{"<", ">", "<=", ">=", "BETWEEN"},
			excludes: []string{"LIKE", "GLOB", "REGEXP"},
		},
		{
			name:     "INTEGER column - qualified reference",
			sql:      "SELECT * FROM users WHERE users.id |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
		},

		// REAL/NUMERIC/DOUBLE column tests
		{
			name:     "REAL column operators",
			sql:      "SELECT * FROM users WHERE balance |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
			excludes: []string{"LIKE", "GLOB"},
		},
		{
			name:     "NUMERIC column operators",
			sql:      "SELECT * FROM users WHERE score |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},
		{
			name:     "DOUBLE column operators",
			sql:      "SELECT * FROM products WHERE price |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},

		// TEXT column tests
		{
			name:     "TEXT column - common operators",
			sql:      "SELECT * FROM users WHERE name |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN"},
		},
		{
			name:     "TEXT column - string operators",
			sql:      "SELECT * FROM users WHERE name |",
			contains: []string{"LIKE", "NOT LIKE", "GLOB", "REGEXP", "MATCH"},
		},
		{
			name:     "TEXT column - comparison operators",
			sql:      "SELECT * FROM users WHERE name |",
			contains: []string{"<", ">", "<=", ">="},
		},

		// VARCHAR column tests
		{
			name:     "VARCHAR column operators",
			sql:      "SELECT * FROM users WHERE email |",
			contains: []string{"=", "<>", "LIKE", "NOT LIKE", "GLOB", "REGEXP", "MATCH"},
		},

		// CHAR column tests
		{
			name:     "CHAR column operators",
			sql:      "SELECT * FROM products WHERE description |",
			contains: []string{"=", "<>", "LIKE", "NOT LIKE", "GLOB"},
		},

		// CLOB column tests
		{
			name:     "CLOB column operators",
			sql:      "SELECT * FROM users WHERE notes |",
			contains: []string{"LIKE", "NOT LIKE", "GLOB", "REGEXP", "MATCH"},
		},

		// BOOLEAN column tests
		{
			name:     "BOOLEAN column operators",
			sql:      "SELECT * FROM users WHERE is_active |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IS", "IS NOT"},
			excludes: []string{"LIKE", "GLOB", "BETWEEN"},
		},

		// BLOB column tests
		{
			name:     "BLOB column - basic operators only",
			sql:      "SELECT * FROM users WHERE avatar |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN"},
			excludes: []string{"LIKE", "GLOB", "REGEXP", "<", ">", "BETWEEN"},
		},

		// DATE/TIME column tests
		{
			name:     "DATETIME column operators",
			sql:      "SELECT * FROM users WHERE created_at |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
			excludes: []string{"LIKE", "GLOB"},
		},
		{
			name:     "DATE column operators",
			sql:      "SELECT * FROM users WHERE birth_date |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},

		// Complex query contexts
		{
			name:     "operator in JOIN ON clause",
			sql:      "SELECT * FROM users u JOIN products p ON u.id |",
			contains: []string{"=", "<>", "<", ">"},
		},
		{
			name:     "operator in subquery",
			sql:      "SELECT * FROM users WHERE id IN (SELECT id FROM products WHERE price |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
		},
		{
			name:     "operator after AND",
			sql:      "SELECT * FROM users WHERE age > 18 AND name |",
			contains: []string{"=", "<>", "LIKE", "GLOB"},
		},
		{
			name:     "operator after OR",
			sql:      "SELECT * FROM users WHERE age > 18 OR balance |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
		},
		// Multi-line and qualified column tests
		{
			name: "operator after AND with qualified column",
			sql: `SELECT id FROM users u
WHERE u.is_active IS NOT NULL
  AND u.name |`,
			contains: []string{"=", "<>", "LIKE", "GLOB"},
		},
		{
			name: "operator after AND with quoted qualified column",
			sql: `SELECT id FROM users u
WHERE u."is_active" IS NOT NULL
  AND u."name" |`,
			contains: []string{"=", "<>", "LIKE", "GLOB"},
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

			// Build map of operators returned
			gotOps := make(map[string]bool)
			for _, c := range got {
				if c.Type == core.CandidateTypeOperator {
					gotOps[c.Text] = true
				}
			}

			// Check that all expected operators are present
			for _, op := range tc.contains {
				if !gotOps[op] {
					t.Errorf("Missing expected operator: %s", op)
				}
			}

			// Check that excluded operators are not present
			for _, op := range tc.excludes {
				if gotOps[op] {
					t.Errorf("Unexpected operator present: %s", op)
				}
			}
		})
	}
}
