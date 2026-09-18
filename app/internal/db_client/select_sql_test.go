package db_client

import (
	"strings"
	"testing"

	"selectDb/internal/graph"

	"github.com/selectDb/dialect/engine"
)

func TestBuildSelectSQL(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		schema   string
		table    string
		limit    int
		expected string
	}{
		{
			name:     "postgresql qualified",
			dbType:   "postgresql",
			schema:   "public",
			table:    "users",
			limit:    100,
			expected: "SELECT *\nFROM public.users\nLIMIT 100;\n",
		},
		{
			name:     "postgresql quotes mixed case",
			dbType:   "postgresql",
			schema:   "public",
			table:    "AppAccount",
			limit:    100,
			expected: "SELECT *\nFROM public.\"AppAccount\"\nLIMIT 100;\n",
		},
		{
			name:     "postgresql quotes reserved keyword",
			dbType:   "postgresql",
			schema:   "public",
			table:    "order",
			limit:    100,
			expected: "SELECT *\nFROM public.\"order\"\nLIMIT 100;\n",
		},
		{
			name:     "postgresql quotes special characters",
			dbType:   "postgresql",
			schema:   "my schema",
			table:    "user-events",
			limit:    100,
			expected: "SELECT *\nFROM \"my schema\".\"user-events\"\nLIMIT 100;\n",
		},
		{
			name:     "mysql uses backticks",
			dbType:   "mysql",
			schema:   "shop",
			table:    "Orders",
			limit:    25,
			expected: "SELECT *\nFROM shop.`Orders`\nLIMIT 25;\n",
		},
		{
			name:     "mysql without schema is unqualified",
			dbType:   "mysql",
			schema:   "",
			table:    "orders",
			limit:    25,
			expected: "SELECT *\nFROM orders\nLIMIT 25;\n",
		},
		{
			name:     "sqlite main schema",
			dbType:   "sqlite",
			schema:   "main",
			table:    "users",
			limit:    10,
			expected: "SELECT *\nFROM main.users\nLIMIT 10;\n",
		},
		{
			name:     "sqlite quotes special characters",
			dbType:   "sqlite",
			schema:   "main",
			table:    "user events",
			limit:    10,
			expected: "SELECT *\nFROM main.\"user events\"\nLIMIT 10;\n",
		},
		{
			name:     "blank schema is not qualified",
			dbType:   "postgresql",
			schema:   "   ",
			table:    "users",
			limit:    100,
			expected: "SELECT *\nFROM users\nLIMIT 100;\n",
		},
		{
			name:     "table name is trimmed",
			dbType:   "postgresql",
			schema:   "public",
			table:    "  users  ",
			limit:    100,
			expected: "SELECT *\nFROM public.users\nLIMIT 100;\n",
		},
		{
			name:     "zero limit falls back to default",
			dbType:   "postgresql",
			schema:   "public",
			table:    "users",
			limit:    0,
			expected: "SELECT *\nFROM public.users\nLIMIT 100;\n",
		},
		{
			name:     "negative limit falls back to default",
			dbType:   "postgresql",
			schema:   "public",
			table:    "users",
			limit:    -5,
			expected: "SELECT *\nFROM public.users\nLIMIT 100;\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialect := engine.GetDialect(tt.dbType)
			if dialect == nil {
				t.Fatalf("no dialect registered for %q", tt.dbType)
			}

			sql, err := buildSelectSQL(tt.schema, tt.table, tt.limit, dialect)
			if err != nil {
				t.Fatalf("buildSelectSQL() unexpected error: %v", err)
			}
			if sql != tt.expected {
				t.Errorf("buildSelectSQL() =\n%q\nwant\n%q", sql, tt.expected)
			}
		})
	}
}

func TestBuildSelectSQLRequiresTable(t *testing.T) {
	dialect := engine.GetDialect("postgresql")

	for _, table := range []string{"", "   "} {
		if _, err := buildSelectSQL("public", table, 100, dialect); err == nil {
			t.Errorf("buildSelectSQL(table=%q) expected an error, got none", table)
		}
	}
}

// The generated statement must stay a single terminated statement: the frontend
// runs it as-is when the preview tab opens.
func TestBuildSelectSQLIsSingleStatement(t *testing.T) {
	dialect := engine.GetDialect("postgresql")

	sql, err := buildSelectSQL("public", "users", 100, dialect)
	if err != nil {
		t.Fatalf("buildSelectSQL() unexpected error: %v", err)
	}
	if got := strings.Count(sql, ";"); got != 1 {
		t.Errorf("expected exactly one statement terminator, got %d in %q", got, sql)
	}
}

// A quote embedded in an identifier must be escaped, never able to close the
// quoted identifier and leak into the statement.
func TestBuildSelectSQLEscapesQuotesInIdentifiers(t *testing.T) {
	dialect := engine.GetDialect("postgresql")

	sql, err := buildSelectSQL("public", `we"ird`, 100, dialect)
	if err != nil {
		t.Fatalf("buildSelectSQL() unexpected error: %v", err)
	}
	if want := "SELECT *\nFROM public.\"we\"\"ird\"\nLIMIT 100;\n"; sql != want {
		t.Errorf("buildSelectSQL() =\n%q\nwant\n%q", sql, want)
	}
}

func TestGenerateSelectSQLUnknownDatabase(t *testing.T) {
	dbc := &DbClient{Graph: &graph.Graph{}}

	if _, err := dbc.GenerateSelectSQL(GenerateSelectSQLParams{
		DatabaseID: "does-not-exist",
		Schema:     "public",
		Table:      "users",
	}); err == nil {
		t.Error("expected an error for an unknown database, got none")
	}
}
