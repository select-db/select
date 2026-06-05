package mcp

import (
	"testing"

	"github.com/selectDb/dialect/core"
)

// ruleString is the user-visible projection of the role's permission
// rules in list_datasources. A regression here changes what every MCP
// client sees, so pin the format.
func TestRuleString(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	cases := []struct {
		name  string
		entry core.PermissionEntry
		want  string
	}{
		{
			name: "fully specific allow",
			entry: core.PermissionEntry{
				DbInstanceID: strPtr("db-1"),
				SchemaName:   strPtr("public"),
				TableName:    strPtr("users"),
				ColumnName:   strPtr("password"),
				Action:       "select",
				Effect:       "deny",
			},
			want: "deny.select.public.users.password",
		},
		{
			name: "wildcard column",
			entry: core.PermissionEntry{
				DbInstanceID: strPtr("db-1"),
				SchemaName:   strPtr("public"),
				TableName:    strPtr("events"),
				ColumnName:   nil,
				Action:       "insert",
				Effect:       "allow",
			},
			want: "allow.insert.public.events.*",
		},
		{
			name: "schema only (table + column wildcard)",
			entry: core.PermissionEntry{
				DbInstanceID: strPtr("db-1"),
				SchemaName:   strPtr("public"),
				TableName:    nil,
				ColumnName:   nil,
				Action:       "select",
				Effect:       "allow",
			},
			want: "allow.select.public.*.*",
		},
		{
			name: "all wildcards",
			entry: core.PermissionEntry{
				DbInstanceID: strPtr("db-1"),
				SchemaName:   nil,
				TableName:    nil,
				ColumnName:   nil,
				Action:       "ddl",
				Effect:       "allow",
			},
			want: "allow.ddl.*.*.*",
		},
		{
			name: "explicit star is treated as wildcard",
			entry: core.PermissionEntry{
				DbInstanceID: strPtr("db-1"),
				SchemaName:   strPtr("*"),
				TableName:    strPtr("*"),
				ColumnName:   strPtr("*"),
				Action:       "update",
				Effect:       "allow",
			},
			want: "allow.update.*.*.*",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ruleString(tc.entry)
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
