package postgresql

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
		DefaultSchema: "public",
		Schemas: []core.Schema{
			{
				Name: "public",
				Tables: []core.Table{
					{
						Name: "users",
						Columns: []core.Column{
							{Name: "id", Type: "INTEGER"},
							{Name: "user_id", Type: "BIGINT"},
							{Name: "full_name", Type: "TEXT"},
							{Name: "email", Type: "VARCHAR(255)"},
							{Name: "description", Type: "CHARACTER VARYING"},
							{Name: "code", Type: "CHAR(10)"},
							{Name: "age", Type: "SMALLINT"},
							{Name: "balance", Type: "DECIMAL(10,2)"},
							{Name: "score", Type: "NUMERIC"},
							{Name: "rating", Type: "REAL"},
							{Name: "amount", Type: "DOUBLE PRECISION"},
							{Name: "price", Type: "MONEY"},
							{Name: "is_active", Type: "BOOLEAN"},
							{Name: "uuid", Type: "UUID"},
							{Name: "created_at", Type: "TIMESTAMP"},
							{Name: "birth_date", Type: "DATE"},
							{Name: "start_time", Type: "TIME"},
							{Name: "interval_val", Type: "INTERVAL"},
							{Name: "tags", Type: "TEXT[]"},
							{Name: "scores", Type: "integer[]"},
							{Name: "matrix", Type: "numeric[]"},
							{Name: "metadata", Type: "JSONB"},
							{Name: "config", Type: "JSON"},
							{Name: "search_name", Type: "CITEXT"},
						},
					},
					{
						Name: "products",
						Columns: []core.Column{
							{Name: "id", Type: "SERIAL"},
							{Name: "sku", Type: "BIGSERIAL"},
							{Name: "data", Type: "BYTEA"},
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
		// INTEGER/BIGINT/SMALLINT column tests
		{
			name:     "INTEGER column - all operators",
			sql:      "SELECT * FROM users WHERE id |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN", "<", ">", "<=", ">=", "BETWEEN"},
			excludes: []string{"LIKE", "ILIKE", "~"},
		},
		{
			name:     "BIGINT column operators",
			sql:      "SELECT * FROM users WHERE user_id |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},
		{
			name:     "SERIAL column operators",
			sql:      "SELECT * FROM products WHERE id |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
		},
		{
			name:     "BIGSERIAL column operators",
			sql:      "SELECT * FROM products WHERE sku |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
		},

		// DECIMAL/NUMERIC/REAL/DOUBLE column tests
		{
			name:     "DECIMAL column operators",
			sql:      "SELECT * FROM users WHERE balance |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
			excludes: []string{"LIKE", "ILIKE"},
		},
		{
			name:     "NUMERIC column operators",
			sql:      "SELECT * FROM users WHERE score |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},
		{
			name:     "REAL column operators",
			sql:      "SELECT * FROM users WHERE rating |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},
		{
			name:     "DOUBLE PRECISION column operators",
			sql:      "SELECT * FROM users WHERE amount |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},
		{
			name:     "MONEY column operators",
			sql:      "SELECT * FROM users WHERE price |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},

		// TEXT/VARCHAR/CHAR column tests
		{
			name:     "TEXT column - all operators",
			sql:      "SELECT * FROM users WHERE full_name |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN", "LIKE", "ILIKE", "NOT LIKE", "NOT ILIKE", "SIMILAR TO", "~", "~*", "!~", "!~*"},
		},
		{
			name:     "VARCHAR column operators",
			sql:      "SELECT * FROM users WHERE email |",
			contains: []string{"=", "<>", "LIKE", "ILIKE", "~", "~*"},
		},
		{
			name:     "CHARACTER VARYING column operators",
			sql:      "SELECT * FROM users WHERE description |",
			contains: []string{"LIKE", "ILIKE", "SIMILAR TO", "~"},
		},
		{
			name:     "CHAR column operators",
			sql:      "SELECT * FROM users WHERE code |",
			contains: []string{"=", "<>", "LIKE", "ILIKE"},
		},
		{
			name:     "CITEXT column operators",
			sql:      "SELECT * FROM users WHERE search_name |",
			contains: []string{"=", "<>", "LIKE", "ILIKE", "~", "~*"},
		},

		// BOOLEAN column tests
		{
			name:     "BOOLEAN column - boolean operators",
			sql:      "SELECT * FROM users WHERE is_active |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IS TRUE", "IS FALSE", "IS NOT TRUE", "IS NOT FALSE"},
			excludes: []string{"LIKE", "ILIKE", "<", ">", "BETWEEN"},
		},

		// UUID column tests
		{
			name:     "UUID column - basic operators only",
			sql:      "SELECT * FROM users WHERE uuid |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN"},
			excludes: []string{"LIKE", "ILIKE", "<", ">", "BETWEEN"},
		},

		// DATE/TIME column tests
		{
			name:     "TIMESTAMP column operators",
			sql:      "SELECT * FROM users WHERE created_at |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
			excludes: []string{"LIKE", "ILIKE"},
		},
		{
			name:     "DATE column operators",
			sql:      "SELECT * FROM users WHERE birth_date |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},
		{
			name:     "TIME column operators",
			sql:      "SELECT * FROM users WHERE start_time |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},
		{
			name:     "INTERVAL column operators",
			sql:      "SELECT * FROM users WHERE interval_val |",
			contains: []string{"=", "<>", "<", ">", "<=", ">=", "BETWEEN"},
		},

		// ARRAY column tests
		{
			name:     "TEXT[] array - common operators",
			sql:      "SELECT * FROM users WHERE tags |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN"},
		},
		{
			name:     "TEXT[] array - array operators",
			sql:      "SELECT * FROM users WHERE tags |",
			contains: []string{"@>", "<@", "&&", "||", "ANY", "ALL"},
		},
		{
			name:     "integer[] array operators",
			sql:      "SELECT * FROM users WHERE scores |",
			contains: []string{"@>", "<@", "&&", "||", "ANY", "ALL"},
		},
		{
			name:     "numeric[] array operators",
			sql:      "SELECT * FROM users WHERE matrix |",
			contains: []string{"@>", "<@", "&&", "||"},
		},

		// JSON/JSONB column tests
		{
			name:     "JSONB column - common operators",
			sql:      "SELECT * FROM users WHERE metadata |",
			contains: []string{"=", "<>", "IS NULL", "IS NOT NULL", "IN"},
		},
		{
			name:     "JSONB column - path operators",
			sql:      "SELECT * FROM users WHERE metadata |",
			contains: []string{"->", "->>", "#>", "#>>"},
		},
		{
			name:     "JSONB column - containment operators",
			sql:      "SELECT * FROM users WHERE metadata |",
			contains: []string{"@>", "<@", "?", "?|", "?&"},
		},
		{
			name:     "JSON column - path operators",
			sql:      "SELECT * FROM users WHERE config |",
			contains: []string{"->", "->>", "#>", "#>>"},
		},

		// Complex query contexts
		{
			name:     "operator in JOIN ON clause",
			sql:      "SELECT * FROM users u JOIN products p ON u.id |",
			contains: []string{"=", "<>", "<", ">"},
		},
		{
			name:     "operator in subquery",
			sql:      "SELECT * FROM users WHERE id IN (SELECT id FROM products WHERE id |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
		},
		{
			name:     "operator after AND",
			sql:      "SELECT * FROM users WHERE age > 18 AND full_name |",
			contains: []string{"=", "<>", "LIKE", "ILIKE", "~"},
		},
		{
			name:     "operator after OR",
			sql:      "SELECT * FROM users WHERE age > 18 OR balance |",
			contains: []string{"=", "<>", "<", ">", "BETWEEN"},
		},
		{
			name:     "operator with table alias",
			sql:      "SELECT * FROM users u WHERE u.full_name |",
			contains: []string{"=", "<>", "LIKE", "ILIKE", "~"},
		},
		{
			name:     "operator in CTE",
			sql:      "WITH active_users AS (SELECT * FROM users WHERE is_active |",
			contains: []string{"=", "<>", "IS TRUE", "IS FALSE"},
		},
		// Multi-line and qualified column tests (regression tests for AND/OR handling)
		{
			name: "operator after AND with qualified column multiline",
			sql: `SELECT id FROM users u
WHERE u.is_active IS NOT NULL
  AND u.full_name |`,
			contains: []string{"=", "<>", "LIKE", "ILIKE"},
		},
		{
			name: "operator after AND with quoted qualified column",
			sql: `SELECT id FROM users u
WHERE u."is_active" IS NOT NULL
  AND u."full_name" |`,
			contains: []string{"=", "<>", "LIKE", "ILIKE"},
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
