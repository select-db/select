package core_references

import (
	"encoding/json"
	"fmt"

	core "github.com/selectDb/dialect/core"
)

// ColumnRef represents a column reference with resolution status.
type ColumnRef struct {
	Column       string
	Qualified    bool
	Resolved     bool
	TokenPos     int
	NestingLevel int
	Line         int
	Col          int
	Len          int
}

type pyColumnRef struct {
	Column       string `json:"column"`
	Qualified    bool   `json:"qualified"`
	Resolved     bool   `json:"resolved"`
	NestingLevel int    `json:"nesting_level"`
	Line         int    `json:"line"`
	Col          int    `json:"col"`
	Len          int    `json:"len"`
}

type pyColumnAlias struct {
	Alias    string `json:"alias"`
	Column   string `json:"column"`
	Type     string `json:"type"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
	Nullable bool   `json:"nullable"`
}

type pyColumnRefsResponse struct {
	ColumnRefs    []pyColumnRef   `json:"column_refs"`
	ColumnAliases []pyColumnAlias `json:"column_aliases"`
}

// CollectColumnRefsFromPython calls the Python analyzer to collect resolved
// column references and SELECT-list aliases.
func CollectColumnRefsFromPython(
	analyzer core.Analyzer,
	sql string,
	dialect core.SQLDialect,
	meta core.Metadata,
	opts ...any,
) ([]ColumnRef, []ColumnAlias, error) {
	if analyzer == nil {
		return nil, nil, fmt.Errorf("analyzer is nil")
	}

	req := map[string]any{
		"action":         "collect_column_refs",
		"sql":            sql,
		"dialect":        dialect.Name(),
		"schema":         core.MetaToSchemaDict(meta),
		"default_schema": core.GetDefaultSchema(meta),
	}

	for _, opt := range opts {
		if c, ok := opt.(*CompletionCaret); ok && c != nil {
			req["caret_line"] = c.Line
			req["caret_col"] = c.Col
		}
	}

	raw, err := analyzer.Call(req)
	if err != nil {
		return nil, nil, fmt.Errorf("collect_column_refs call: %w", err)
	}

	var resp pyColumnRefsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("collect_column_refs unmarshal: %w", err)
	}

	refs := make([]ColumnRef, 0, len(resp.ColumnRefs))
	for _, r := range resp.ColumnRefs {
		refs = append(refs, ColumnRef{
			Column:       r.Column,
			Qualified:    r.Qualified,
			Resolved:     r.Resolved,
			NestingLevel: r.NestingLevel,
			Line:         r.Line,
			Col:          r.Col,
			Len:          r.Len,
		})
	}

	aliases := make([]ColumnAlias, 0, len(resp.ColumnAliases))
	for _, a := range resp.ColumnAliases {
		aliases = append(aliases, ColumnAlias{
			Alias:    a.Alias,
			Column:   a.Column,
			Type:     a.Type,
			Schema:   a.Schema,
			Table:    a.Table,
			Nullable: a.Nullable,
		})
	}

	return refs, aliases, nil
}
