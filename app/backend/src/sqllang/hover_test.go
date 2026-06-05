package sqllang

import (
	"reflect"
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
)

func TestQuotedIdentPairAt(t *testing.T) {
	tests := []struct {
		name string
		line string
		col  int
		want []string
	}{
		{
			name: "cursor inside quoted identifier with qualifier",
			line: `mytable."my col"`,
			col:  strings.Index(`mytable."my col"`, "m") + len(`mytable."`) + 1, // inside "my col"
			want: []string{"mytable", "my col"},
		},
		{
			name: "cursor on opening quote returns nil",
			line: `t."col"`,
			col:  strings.Index(`t."col"`, `"`),
			want: nil,
		},
		{
			name: "cursor on closing quote returns nil",
			line: `t."col"`,
			col:  strings.LastIndex(`t."col"`, `"`),
			want: nil,
		},
		{
			name: "doubled quotes inside identifier",
			line: `t."say ""hi"""`,
			col:  strings.Index(`t."say ""hi"""`, "s") + 3, // inside the quoted part
			want: []string{"t", `say "hi"`},
		},
		{
			name: "bare quoted identifier without qualifier",
			line: `"col"`,
			col:  2,
			want: []string{"col"},
		},
		{
			name: "empty line returns nil",
			line: ``,
			col:  0,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quotedIdentPairAt(tt.line, tt.col)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIdentChainAt(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		line int
		col  int
		want []string
	}{
		{
			name: "three-part chain",
			sql:  "SELECT public.users.id FROM public.users",
			line: 1,
			col:  strings.Index("SELECT public.users.id FROM public.users", "id"),
			want: []string{"public", "users", "id"},
		},
		{
			name: "two-part chain cursor on table",
			sql:  "SELECT u.name FROM users u",
			line: 1,
			col:  strings.Index("SELECT u.name FROM users u", "u.name"),
			want: []string{"u", "name"},
		},
		{
			name: "single identifier",
			sql:  "SELECT name FROM users",
			line: 1,
			col:  strings.Index("SELECT name FROM users", "name"),
			want: []string{"name"},
		},
		{
			name: "quoted column with qualifier",
			sql:  "SELECT\n  c.\"createdAt\"\nFROM c",
			line: 2,
			col:  strings.Index(`  c."createdAt"`, "c") + len(`  c."create`),
			want: []string{"c", "createdAt"},
		},
		{
			name: "out of range line returns nil",
			sql:  "SELECT 1",
			line: 5,
			col:  0,
			want: nil,
		},
		{
			name: "cursor at col 0 on non-ident char returns nil",
			sql:  "(a + b)",
			line: 1,
			col:  0, // opening paren, no ident to walk back to
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identChainAt(tt.sql, tt.line, tt.col)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSqlHoverMarkdownFunctionDescription(t *testing.T) {
	mdWith := sqlHoverMarkdownFunction(core.Function{
		Name:        "array_agg",
		Args:        "anynonarray",
		Result:      "anyarray",
		Schema:      "pg_catalog",
		Description: "aggregate values into an array",
	})
	if !strings.Contains(mdWith, "aggregate values into an array") {
		t.Fatalf("expected function description in hover markdown, got: %q", mdWith)
	}

	mdWithout := sqlHoverMarkdownFunction(core.Function{
		Name:   "array_agg",
		Args:   "anynonarray",
		Result: "anyarray",
		Schema: "pg_catalog",
	})
	if strings.Contains(mdWithout, "aggregate values into an array") {
		t.Fatalf("did not expect description section when empty, got: %q", mdWithout)
	}
}

func TestRenderHoverMarkdownTypeDescription(t *testing.T) {
	meta := &core.Metadata{
		Schemas: []core.Schema{{
			Name: "pg_catalog",
			Types: []core.Type{
				{
					Schema:      "pg_catalog",
					Name:        "text",
					Kind:        "b",
					Display:     "text",
					Description: "variable-length string",
				},
			},
		}},
	}
	obj := &resolvedObject{Kind: "type", Rel: "text"}
	md := renderHoverMarkdown(meta, nil, obj)
	if !strings.Contains(md, "variable-length string") {
		t.Fatalf("expected type description in hover markdown, got: %q", md)
	}

	meta.Schemas[0].Types[0].Description = ""
	md = renderHoverMarkdown(meta, nil, obj)
	if strings.Contains(md, "variable-length string") {
		t.Fatalf("did not expect description section when empty, got: %q", md)
	}
}
