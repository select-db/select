package core

import (
	"testing"
)

const testDBID = "db-1"

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func pe(dbID, schema, table, column, action, effect string) PermissionEntry {
	return PermissionEntry{
		DbInstanceID: strPtr(dbID),
		SchemaName:   strPtr(schema),
		TableName:    strPtr(table),
		ColumnName:   strPtr(column),
		Action:       action,
		Effect:       effect,
	}
}

func selectRes(schema, table string, columns ...string) InspectStatement {
	r := InspectStatement{
		Operation: InspectOpSelect,
		Tables:    []InspectTable{{Name: table, Schema: schema}},
	}
	for _, col := range columns {
		r.Fields = append(r.Fields, InspectField{Name: col, Table: table, Schema: schema})
	}
	return r
}

type permTestCase struct {
	name    string
	results []InspectStatement
	entries []PermissionEntry
	wantErr bool
}

func getPermissionTestCases() []permTestCase {
	return []permTestCase{
		// --- Unmanaged instance bypass ---
		{
			name:    "no entries means unmanaged, allow all",
			results: []InspectStatement{selectRes("public", "secrets", "password")},
			entries: nil,
			wantErr: false,
		},
		{
			name:    "entries for other instance only, this instance unmanaged",
			results: []InspectStatement{selectRes("public", "secrets", "password")},
			entries: []PermissionEntry{pe("other-db", "public", "t1", "", "select", "allow")},
			wantErr: false,
		},
		// --- Basic allow / deny ---
		{
			name:    "explicit column allow",
			results: []InspectStatement{selectRes("public", "t1", "c1")},
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "c1", "select", "allow")},
			wantErr: false,
		},
		{
			name:    "instance managed but no matching allow rule",
			results: []InspectStatement{selectRes("public", "t1", "c1")},
			entries: []PermissionEntry{pe(testDBID, "public", "other", "c1", "select", "allow")},
			wantErr: true,
		},
		{
			name:    "explicit deny overrides allow at same level",
			results: []InspectStatement{selectRes("public", "t1", "secret")},
			entries: []PermissionEntry{
				pe(testDBID, "public", "t1", "secret", "select", "allow"),
				pe(testDBID, "public", "t1", "secret", "select", "deny"),
			},
			wantErr: true,
		},

		// --- Wildcard matching ---
		{
			name:    "table wildcard allows all tables",
			results: []InspectStatement{selectRes("public", "t1", "c1")},
			entries: []PermissionEntry{pe(testDBID, "public", "", "", "select", "allow")},
			wantErr: false,
		},
		{
			name:    "schema wildcard allows all schemas",
			results: []InspectStatement{selectRes("public", "t1", "c1")},
			entries: []PermissionEntry{pe(testDBID, "", "", "", "select", "allow")},
			wantErr: false,
		},
		{
			name:    "column wildcard allows all columns",
			results: []InspectStatement{selectRes("public", "t1", "c1", "c2")},
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "", "select", "allow")},
			wantErr: false,
		},
		{
			// table-level deny blocks even if a column is individually allowed
			name:    "table-level deny blocks column-level allow",
			results: []InspectStatement{selectRes("public", "secrets", "password")},
			entries: []PermissionEntry{
				pe(testDBID, "public", "secrets", "password", "select", "allow"),
				pe(testDBID, "public", "secrets", "", "select", "deny"),
			},
			wantErr: true,
		},

		// --- Specificity: most-specific rule wins ---
		{
			// deny at any level blocks access, even if a more specific allow exists.
			// Admins use schema-level denies as hard blocks; specific allows cannot undo them.
			name:    "wildcard deny blocks specific allow",
			results: []InspectStatement{selectRes("public", "t1", "c1")},
			entries: []PermissionEntry{
				pe(testDBID, "public", "", "", "select", "deny"),
				pe(testDBID, "public", "t1", "c1", "select", "allow"),
			},
			wantErr: true,
		},
		{
			name:    "specific deny overrides wildcard allow",
			results: []InspectStatement{selectRes("public", "t1", "secret")},
			entries: []PermissionEntry{
				pe(testDBID, "public", "", "", "select", "allow"),
				pe(testDBID, "public", "t1", "secret", "select", "deny"),
			},
			wantErr: true,
		},

		// --- Wrong action ---
		{
			name: "allow select does not grant insert",
			results: []InspectStatement{{
				Operation: InspectOpInsert,
				Tables:    []InspectTable{{Name: "t1", Schema: "public"}},
				Fields:    []InspectField{{Name: "c1", Table: "t1", Schema: "public"}},
			}},
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "c1", "select", "allow")},
			wantErr: true,
		},

		// --- Unknown tables (inspector sets Schema="") ---
		{
			// Schema="" means the inspector couldn't resolve the table, always deny,
			// even with blanket allow rules, to prevent silent data exfiltration.
			name:    "unknown table always denied",
			results: []InspectStatement{selectRes("", "unknown_table", "c1")},
			entries: []PermissionEntry{pe(testDBID, "", "", "", "select", "allow")},
			wantErr: true,
		},

		// --- Multi-statement ---
		{
			name: "deny if any statement is blocked",
			results: []InspectStatement{
				selectRes("public", "t1", "c1"),
				selectRes("public", "secrets", "password"),
			},
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "c1", "select", "allow")},
			wantErr: true,
		},

		// --- Subquery bypass (the core attack vector) ---
		{
			// Attacker hides a forbidden SELECT inside a scalar subquery or CTE.
			// The outer table is allowed; the inner must still be checked.
			name: "subquery reading forbidden table is denied",
			results: func() []InspectStatement {
				outer := selectRes("public", "t1", "c1")
				inner := selectRes("public", "secrets", "password")
				outer.Subqueries = []InspectStatement{inner}
				return []InspectStatement{outer}
			}(),
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "c1", "select", "allow")},
			wantErr: true,
		},
		{
			name: "deeply nested subquery is checked recursively",
			results: func() []InspectStatement {
				level3 := selectRes("public", "secrets", "password")
				level2 := selectRes("public", "t2", "c1")
				level2.Subqueries = []InspectStatement{level3}
				level1 := selectRes("public", "t1", "c1")
				level1.Subqueries = []InspectStatement{level2}
				return []InspectStatement{level1}
			}(),
			entries: []PermissionEntry{
				pe(testDBID, "public", "t1", "c1", "select", "allow"),
				pe(testDBID, "public", "t2", "c1", "select", "allow"),
			},
			wantErr: true,
		},
		{
			name: "all subquery tables allowed passes",
			results: func() []InspectStatement {
				inner := selectRes("public", "t2", "c1")
				outer := selectRes("public", "t1", "c1")
				outer.Subqueries = []InspectStatement{inner}
				return []InspectStatement{outer}
			}(),
			entries: []PermissionEntry{
				pe(testDBID, "public", "t1", "c1", "select", "allow"),
				pe(testDBID, "public", "t2", "c1", "select", "allow"),
			},
			wantErr: false,
		},

		// --- DDL ---
		{
			name: "select permission does not grant ddl",
			results: []InspectStatement{{
				Operation: InspectOpDrop,
				Tables:    []InspectTable{{Name: "t1", Schema: "public"}},
			}},
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "", "select", "allow")},
			wantErr: true,
		},
		{
			name: "ddl permission allows drop",
			results: []InspectStatement{{
				Operation: InspectOpDrop,
				Tables:    []InspectTable{{Name: "t1", Schema: "public"}},
			}},
			entries: []PermissionEntry{pe(testDBID, "public", "t1", "", "ddl", "allow")},
			wantErr: false,
		},
	}
}

func TestCheckQueryPermissions(t *testing.T) {
	for _, tc := range getPermissionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckQueryPermissions(tc.results, testDBID, Compile(tc.entries))
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}

func TestEvaluateSee(t *testing.T) {
	tests := []struct {
		name       string
		stmt       InspectStatement
		driverCols []string
		entries    []PermissionEntry
		wantMask   []int
		wantErr    bool
	}{
		{
			name: "unmanaged instance returns nil",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields:    []InspectField{{Name: "email", Table: "users", Schema: "public"}},
			},
			driverCols: []string{"email"},
			entries:    nil,
			wantMask:   nil,
		},
		{
			name: "non-SELECT statement returns nil",
			stmt: InspectStatement{
				Operation: InspectOpInsert,
				Fields:    []InspectField{{Name: "email", Table: "users", Schema: "public"}},
			},
			driverCols: []string{"email"},
			entries:    []PermissionEntry{pe(testDBID, "public", "users", "email", "see", "deny")},
			wantMask:   nil,
		},
		{
			name: "bare projection see-allowed produces no mask",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields:    []InspectField{{Name: "email", Table: "users", Schema: "public"}},
			},
			driverCols: []string{"email"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "users", "email", "select", "allow"),
				pe(testDBID, "public", "users", "email", "see", "allow"),
			},
			wantMask: nil,
		},
		{
			name: "bare projection see-denied is masked, not rejected",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields:    []InspectField{{Name: "email", Table: "users", Schema: "public"}},
			},
			driverCols: []string{"email"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "users", "email", "select", "allow"),
			},
			wantMask: []int{0},
		},
		{
			name: "alias matches driver column",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields: []InspectField{
					{Name: "email", Alias: strPtr("e"), Table: "users", Schema: "public"},
				},
			},
			driverCols: []string{"e"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "users", "email", "select", "allow"),
			},
			wantMask: []int{0},
		},
		{
			name: "unaliased function over see-denied column is rejected",
			// LENGTH(email): inspector emits Field{email} with no alias,
			// driver returns the function's text. Names don't match, Field
			// is unmatched, see-denied → reject.
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields:    []InspectField{{Name: "email", Table: "users", Schema: "public"}},
			},
			driverCols: []string{"length"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "users", "email", "select", "allow"),
			},
			wantErr: true,
		},
		{
			name: "expression with all see-allowed columns passes through",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields: []InspectField{
					{Name: "a", Alias: strPtr("calc"), Table: "t", Schema: "public"},
					{Name: "b", Table: "t", Schema: "public"},
				},
			},
			driverCols: []string{"calc"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "t", "a", "select", "allow"),
				pe(testDBID, "public", "t", "b", "select", "allow"),
				pe(testDBID, "public", "t", "a", "see", "allow"),
				pe(testDBID, "public", "t", "b", "see", "allow"),
			},
			wantMask: nil,
		},
		{
			name: "expression with a see-denied non-alias-bearing field is rejected",
			// SELECT a + b AS calc: Field a carries the alias, Field b does not.
			// Only the alias-bearing Field matches the driver column. b stays
			// unmatched; if it's see-denied, reject.
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields: []InspectField{
					{Name: "a", Alias: strPtr("calc"), Table: "t", Schema: "public"},
					{Name: "b", Table: "t", Schema: "public"},
				},
			},
			driverCols: []string{"calc"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "t", "a", "select", "allow"),
				pe(testDBID, "public", "t", "b", "select", "allow"),
				pe(testDBID, "public", "t", "a", "see", "allow"),
				// b: no see grant
			},
			wantErr: true,
		},
		{
			name: "mixed bare projection: only see-denied position is masked",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields: []InspectField{
					{Name: "id", Table: "users", Schema: "public"},
					{Name: "email", Table: "users", Schema: "public"},
				},
			},
			driverCols: []string{"id", "email"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "users", "", "select", "allow"),
				pe(testDBID, "public", "users", "id", "see", "allow"),
			},
			wantMask: []int{1},
		},
		{
			name: "see wildcard at table level covers all columns",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields: []InspectField{
					{Name: "id", Table: "users", Schema: "public"},
					{Name: "email", Table: "users", Schema: "public"},
				},
			},
			driverCols: []string{"id", "email"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "users", "", "select", "allow"),
				pe(testDBID, "public", "users", "", "see", "allow"),
			},
			wantMask: nil,
		},
		{
			name: "JOIN with overlapping names: any see-denied source masks the position",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				Fields: []InspectField{
					{Name: "email", Table: "users", Schema: "public"},
					{Name: "email", Table: "contacts", Schema: "public"},
				},
			},
			driverCols: []string{"email", "email"},
			entries: []PermissionEntry{
				pe(testDBID, "public", "users", "", "select", "allow"),
				pe(testDBID, "public", "contacts", "", "select", "allow"),
				pe(testDBID, "public", "users", "email", "see", "allow"),
				// contacts.email has no see grant
			},
			wantMask: []int{0, 1},
		},
		{
			name: "driver column with no matching field is ignored when no see-denial applies",
			stmt: InspectStatement{
				Operation: InspectOpSelect,
				// SELECT 1 → no Fields
				Fields: nil,
			},
			driverCols: []string{"?column?"},
			entries:    []PermissionEntry{pe(testDBID, "public", "users", "", "see", "allow")},
			wantMask:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			perms := Compile(tc.entries)
			mask, err := EvaluateSee(tc.stmt, tc.driverCols, testDBID, perms)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil (mask=%v)", mask)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if !equalIntSlices(mask, tc.wantMask) {
				t.Errorf("mask = %v, want %v", mask, tc.wantMask)
			}
		})
	}
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
