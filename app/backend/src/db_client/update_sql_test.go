package db_client

import (
	"testing"
)

func TestIsBooleanType(t *testing.T) {
	tests := []struct {
		name     string
		colType  string
		expected bool
	}{
		// Boolean types - should return true
		{"BOOLEAN", "BOOLEAN", true},
		{"BOOL", "BOOL", true},
		{"lowercase boolean", "boolean", true},
		{"lowercase bool", "bool", true},
		{"mixed case Boolean", "Boolean", true},

		// Non-boolean types - should return false
		{"INT", "INT", false},
		{"VARCHAR", "VARCHAR", false},
		{"TEXT", "TEXT", false},
		{"BIT", "BIT", false}, // BIT is not the same as BOOL
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBooleanType(tt.colType)
			if result != tt.expected {
				t.Errorf("isBooleanType(%q) = %v, want %v", tt.colType, result, tt.expected)
			}
		})
	}
}

func TestParseBooleanLiteral(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectedVal bool
		expectedOk  bool
	}{
		// True values
		{"true lowercase", "true", true, true},
		{"TRUE uppercase", "TRUE", true, true},
		{"True mixed", "True", true, true},
		{"t", "t", true, true},
		{"T", "T", true, true},
		{"yes", "yes", true, true},
		{"YES", "YES", true, true},
		{"y", "y", true, true},
		{"on", "on", true, true},
		{"ON", "ON", true, true},
		{"1", "1", true, true},

		// False values
		{"false lowercase", "false", false, true},
		{"FALSE uppercase", "FALSE", false, true},
		{"False mixed", "False", false, true},
		{"f", "f", false, true},
		{"F", "F", false, true},
		{"no", "no", false, true},
		{"NO", "NO", false, true},
		{"n", "n", false, true},
		{"off", "off", false, true},
		{"OFF", "OFF", false, true},
		{"0", "0", false, true},

		// Invalid values
		{"empty string", "", false, false},
		{"random string", "maybe", false, false},
		{"number 2", "2", false, false},
		{"truthy", "truthy", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := parseBooleanLiteral(tt.value)
			if ok != tt.expectedOk {
				t.Errorf("parseBooleanLiteral(%q) ok = %v, want %v", tt.value, ok, tt.expectedOk)
			}
			if ok && val != tt.expectedVal {
				t.Errorf("parseBooleanLiteral(%q) = %v, want %v", tt.value, val, tt.expectedVal)
			}
		})
	}
}

func TestIsNumericType(t *testing.T) {
	tests := []struct {
		name     string
		colType  string
		expected bool
	}{
		// Numeric types - should return true
		{"INT", "INT", true},
		{"INTEGER", "INTEGER", true},
		{"BIGINT", "BIGINT", true},
		{"SMALLINT", "SMALLINT", true},
		{"SERIAL", "SERIAL", true},
		{"BIGSERIAL", "BIGSERIAL", true},
		{"NUMERIC", "NUMERIC", true},
		{"NUMERIC with precision", "NUMERIC(10,2)", true},
		{"DECIMAL", "DECIMAL", true},
		{"DECIMAL with precision", "DECIMAL(10,2)", true},
		{"REAL", "REAL", true},
		{"FLOAT", "FLOAT", true},
		{"DOUBLE", "DOUBLE", true},
		{"DOUBLE PRECISION", "DOUBLE PRECISION", true},

		// Case insensitive
		{"lowercase int", "int", true},
		{"mixed case Integer", "Integer", true},

		// Non-numeric types - should return false
		{"VARCHAR", "VARCHAR", false},
		{"VARCHAR with length", "VARCHAR(255)", false},
		{"TEXT", "TEXT", false},
		{"BOOLEAN", "BOOLEAN", false},
		{"DATE", "DATE", false},
		{"TIMESTAMP", "TIMESTAMP", false},
		{"UUID", "UUID", false},
		{"JSON", "JSON", false},
		{"JSONB", "JSONB", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumericType(tt.colType)
			if result != tt.expected {
				t.Errorf("isNumericType(%q) = %v, want %v", tt.colType, result, tt.expected)
			}
		})
	}
}

func TestIsNumericLiteral(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		// Valid numeric literals
		{"positive integer", "42", true},
		{"zero", "0", true},
		{"negative integer", "-42", true},
		{"positive with plus", "+42", true},
		{"decimal", "3.14", true},
		{"negative decimal", "-3.14", true},
		{"large number", "9999999999", true},
		{"small decimal", "0.001", true},

		// Invalid numeric literals
		{"empty string", "", false},
		{"letters", "abc", false},
		{"mixed letters and numbers", "12abc", false},
		{"number with letters", "42px", false},
		{"spaces", "42 ", false},
		{"comma separator", "1,000", false},
		{"multiple dots", "1.2.3", false},
		{"currency symbol", "$100", false},
		{"percentage", "50%", false},
		{"scientific notation e", "1e10", false}, // Current implementation doesn't support
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumericLiteral(tt.value)
			if result != tt.expected {
				t.Errorf("isNumericLiteral(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestFormatLiteral(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		columnType string
		expected   string
	}{
		// NULL handling (case insensitive)
		{"NULL uppercase", "NULL", "VARCHAR", "NULL"},
		{"NULL lowercase", "null", "VARCHAR", "NULL"},
		{"NULL mixed case", "NuLl", "VARCHAR", "NULL"},
		{"NULL with whitespace", "  NULL  ", "VARCHAR", "NULL"},

		// Numeric values with numeric column types
		{"integer in INT column", "42", "INT", "42"},
		{"negative integer in INT column", "-42", "INT", "-42"},
		{"decimal in DECIMAL column", "123.45", "DECIMAL(10,2)", "123.45"},
		{"negative decimal in FLOAT column", "-3.14", "FLOAT", "-3.14"},
		{"zero in INTEGER column", "0", "INTEGER", "0"},
		{"large number in BIGINT column", "9999999999", "BIGINT", "9999999999"},

		// Numeric values with non-numeric column types (should be quoted)
		{"number in VARCHAR column", "123", "VARCHAR", "'123'"},
		{"number in TEXT column", "42", "TEXT", "'42'"},

		// Non-numeric values in numeric column types (fallback to quoted)
		{"text in INT column", "abc", "INT", "'abc'"},
		{"mixed in DECIMAL column", "12abc", "DECIMAL", "'12abc'"},

		// Boolean values with boolean column types
		{"true in BOOLEAN column", "true", "BOOLEAN", "TRUE"},
		{"TRUE in BOOLEAN column", "TRUE", "BOOLEAN", "TRUE"},
		{"True in BOOL column", "True", "BOOL", "TRUE"},
		{"false in BOOLEAN column", "false", "BOOLEAN", "FALSE"},
		{"FALSE in BOOLEAN column", "FALSE", "BOOLEAN", "FALSE"},
		{"t in BOOLEAN column", "t", "BOOLEAN", "TRUE"},
		{"f in BOOLEAN column", "f", "BOOLEAN", "FALSE"},
		{"yes in BOOLEAN column", "yes", "BOOLEAN", "TRUE"},
		{"no in BOOLEAN column", "no", "BOOLEAN", "FALSE"},
		{"1 in BOOLEAN column", "1", "BOOLEAN", "TRUE"},
		{"0 in BOOLEAN column", "0", "BOOLEAN", "FALSE"},
		{"on in BOOLEAN column", "on", "BOOLEAN", "TRUE"},
		{"off in BOOLEAN column", "off", "BOOLEAN", "FALSE"},

		// Boolean values with non-boolean column types (should be quoted)
		{"true in VARCHAR column", "true", "VARCHAR", "'true'"},
		{"false in TEXT column", "false", "TEXT", "'false'"},

		// Invalid boolean in boolean column (fallback to quoted)
		{"invalid bool in BOOLEAN column", "maybe", "BOOLEAN", "'maybe'"},

		// String values
		{"simple string", "hello", "VARCHAR", "'hello'"},
		{"string with spaces", "hello world", "TEXT", "'hello world'"},
		{"empty string", "", "VARCHAR", "''"},

		// String escaping (single quotes doubled)
		{"string with single quote", "O'Brien", "VARCHAR", "'O''Brien'"},
		{"string with multiple quotes", "It's John's", "TEXT", "'It''s John''s'"},
		{"string with only quote", "'", "VARCHAR", "''''"},
		{"string with consecutive quotes", "''", "VARCHAR", "''''''"},

		// Special characters
		{"string with newline", "line1\nline2", "TEXT", "'line1\nline2'"},
		{"string with tab", "col1\tcol2", "TEXT", "'col1\tcol2'"},
		{"unicode string", "日本語", "VARCHAR", "'日本語'"},
		{"emoji", "👍", "TEXT", "'👍'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatLiteral(tt.value, tt.columnType)
			if err != nil {
				t.Errorf("formatLiteral(%q, %q) returned error: %v", tt.value, tt.columnType, err)
				return
			}
			if result != tt.expected {
				t.Errorf("formatLiteral(%q, %q) = %q, want %q", tt.value, tt.columnType, result, tt.expected)
			}
		})
	}
}

func TestRowGroupKey(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		table    string
		pk       map[string]interface{}
		expected string
	}{
		{
			name:     "single PK",
			schema:   "public",
			table:    "users",
			pk:       map[string]interface{}{"id": 1},
			expected: "public\tusers\tid=1\t",
		},
		{
			name:     "composite PK sorted alphabetically",
			schema:   "public",
			table:    "user_roles",
			pk:       map[string]interface{}{"user_id": 1, "role_id": 2},
			expected: "public\tuser_roles\trole_id=2\tuser_id=1\t",
		},
		{
			name:     "three column PK",
			schema:   "app",
			table:    "audit",
			pk:       map[string]interface{}{"c": 3, "a": 1, "b": 2},
			expected: "app\taudit\ta=1\tb=2\tc=3\t",
		},
		{
			name:     "string PK value",
			schema:   "public",
			table:    "items",
			pk:       map[string]interface{}{"uuid": "abc-123"},
			expected: "public\titems\tuuid=abc-123\t",
		},
		{
			name:     "empty PK",
			schema:   "public",
			table:    "test",
			pk:       map[string]interface{}{},
			expected: "public\ttest\t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rowGroupKey(tt.schema, tt.table, tt.pk)
			if result != tt.expected {
				t.Errorf("rowGroupKey(%q, %q, %v) = %q, want %q",
					tt.schema, tt.table, tt.pk, result, tt.expected)
			}
		})
	}
}

func TestRowGroupKeyDeterminism(t *testing.T) {
	// Ensure the same PK always produces the same key (sorted)
	pk := map[string]interface{}{
		"z_col": 26,
		"a_col": 1,
		"m_col": 13,
	}

	first := rowGroupKey("schema", "table", pk)
	for i := 0; i < 100; i++ {
		result := rowGroupKey("schema", "table", pk)
		if result != first {
			t.Errorf("rowGroupKey is not deterministic: got %q, want %q", result, first)
		}
	}
}

func TestGroupEdits(t *testing.T) {
	// Test that edits for the same row are grouped together
	edits := []TableEditInput{
		{Schema: "public", Table: "users", Column: "name", Value: "John", RowIndex: 0, PrimaryKeyValues: map[string]interface{}{"id": 1}},
		{Schema: "public", Table: "users", Column: "age", Value: "30", RowIndex: 0, PrimaryKeyValues: map[string]interface{}{"id": 1}},
		{Schema: "public", Table: "users", Column: "name", Value: "Jane", RowIndex: 1, PrimaryKeyValues: map[string]interface{}{"id": 2}},
	}

	keyToGroup := make(map[string]*group)
	for i := range edits {
		e := &edits[i]
		k := rowGroupKey(e.Schema, e.Table, e.PrimaryKeyValues)
		if g, ok := keyToGroup[k]; ok {
			g.edits = append(g.edits, *e)
		} else {
			keyToGroup[k] = &group{
				schema:   e.Schema,
				table:    e.Table,
				rowIndex: e.RowIndex,
				pk:       e.PrimaryKeyValues,
				edits:    []TableEditInput{*e},
			}
		}
	}

	// Should have 2 groups (row id=1 and row id=2)
	if len(keyToGroup) != 2 {
		t.Errorf("expected 2 groups, got %d", len(keyToGroup))
	}

	// Check that row 1 has 2 edits grouped together
	key1 := rowGroupKey("public", "users", map[string]interface{}{"id": 1})
	if g, ok := keyToGroup[key1]; ok {
		if len(g.edits) != 2 {
			t.Errorf("expected 2 edits in group for id=1, got %d", len(g.edits))
		}
	} else {
		t.Error("group for id=1 not found")
	}
}

func TestTableEditInputFields(t *testing.T) {
	edit := TableEditInput{
		DatabaseID:       "db-123",
		Schema:           "public",
		Table:            "users",
		Column:           "name",
		Value:            "John",
		RowIndex:         0,
		ColumnIndex:      1,
		PrimaryKeyValues: map[string]interface{}{"id": 1},
	}

	if edit.DatabaseID != "db-123" {
		t.Errorf("DatabaseID = %q, want %q", edit.DatabaseID, "db-123")
	}
	if edit.Schema != "public" {
		t.Errorf("Schema = %q, want %q", edit.Schema, "public")
	}
	if edit.Table != "users" {
		t.Errorf("Table = %q, want %q", edit.Table, "users")
	}
	if edit.Column != "name" {
		t.Errorf("Column = %q, want %q", edit.Column, "name")
	}
	if edit.Value != "John" {
		t.Errorf("Value = %q, want %q", edit.Value, "John")
	}
	if edit.RowIndex != 0 {
		t.Errorf("RowIndex = %d, want %d", edit.RowIndex, 0)
	}
	if edit.ColumnIndex != 1 {
		t.Errorf("ColumnIndex = %d, want %d", edit.ColumnIndex, 1)
	}
	if edit.PrimaryKeyValues["id"] != 1 {
		t.Errorf("PrimaryKeyValues[id] = %v, want %v", edit.PrimaryKeyValues["id"], 1)
	}
}

func TestGenerateUpdateSQLParamsEmpty(t *testing.T) {
	params := GenerateUpdateSQLParams{
		DatabaseID: "db-123",
		Edits:      []TableEditInput{},
	}

	if len(params.Edits) != 0 {
		t.Errorf("expected empty edits, got %d", len(params.Edits))
	}
}

// Benchmark tests
func BenchmarkIsNumericType(b *testing.B) {
	types := []string{"INT", "VARCHAR", "DECIMAL(10,2)", "TEXT", "BIGINT"}
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			isNumericType(t)
		}
	}
}

func BenchmarkIsNumericLiteral(b *testing.B) {
	values := []string{"42", "-3.14", "abc", "123.456", "hello"}
	for i := 0; i < b.N; i++ {
		for _, v := range values {
			isNumericLiteral(v)
		}
	}
}

func BenchmarkFormatLiteral(b *testing.B) {
	testCases := []struct {
		value    string
		colType  string
	}{
		{"42", "INT"},
		{"hello", "VARCHAR"},
		{"NULL", "TEXT"},
		{"O'Brien", "VARCHAR"},
	}
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			formatLiteral(tc.value, tc.colType)
		}
	}
}

func BenchmarkRowGroupKey(b *testing.B) {
	pk := map[string]interface{}{"user_id": 1, "role_id": 2, "tenant_id": 3}
	for i := 0; i < b.N; i++ {
		rowGroupKey("public", "user_roles", pk)
	}
}
