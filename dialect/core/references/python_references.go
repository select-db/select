package core_references

import (
	"encoding/json"
	"fmt"

	core "github.com/selectDb/dialect/core"
)

type pyRelation struct {
	Table            string `json:"table"`
	Schema           string `json:"schema"`
	Database         string `json:"database"`
	Alias            string `json:"alias"`
	IsVirtual        bool   `json:"is_virtual"`
	NestingLevel     int    `json:"nesting_level"`
	Line             int    `json:"line"`
	Col              int    `json:"col"`
	EndCol           int    `json:"end_col"`
	ScopeStartOffset int    `json:"scope_start_offset"`
	ScopeEndOffset   int    `json:"scope_end_offset"`
}

type pyVirtualTable struct {
	Table            string     `json:"table"`
	Schema           string     `json:"schema"`
	Database         string     `json:"database"`
	Alias            string     `json:"alias"`
	IsVirtual        bool       `json:"is_virtual"`
	NestingLevel     int        `json:"nesting_level"`
	Line             int        `json:"line"`
	Col              int        `json:"col"`
	EndCol           int        `json:"end_col"`
	Columns          []pyColumn `json:"columns"`
	ScopeStartOffset int        `json:"scope_start_offset"`
	ScopeEndOffset   int        `json:"scope_end_offset"`
}

type pyColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type pyReferencesResponse struct {
	Relations     []pyRelation     `json:"relations"`
	VirtualTables []pyVirtualTable `json:"virtual_tables"`
}

// CompletionCaret holds caret info for SQL sanitization during completion.
type CompletionCaret struct {
	Line, Col int
}

// CollectReferencesFromPython calls the Python analyzer to collect scope-aware
// relation references. Scope positions are character offsets from the Python side.
func CollectReferencesFromPython(
	analyzer core.Analyzer,
	sql string,
	dialect core.SQLDialect,
	meta core.Metadata,
	opts ...any,
) ([]RelationRef, []RelationRef, error) {
	if analyzer == nil {
		return nil, nil, fmt.Errorf("analyzer is nil")
	}

	req := map[string]any{
		"action":         "collect_references",
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
		return nil, nil, fmt.Errorf("collect_references call: %w", err)
	}

	var resp pyReferencesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("collect_references unmarshal: %w", err)
	}

	refs := make([]RelationRef, 0, len(resp.Relations))
	for _, r := range resp.Relations {
		refs = append(refs, RelationRef{
			Database:      r.Database,
			Schema:        r.Schema,
			Table:         r.Table,
			Alias:         r.Alias,
			IsVirtual:     r.IsVirtual,
			ScopeStartPos: r.ScopeStartOffset,
			ScopeEndPos:   r.ScopeEndOffset,
			NestingLevel:  r.NestingLevel,
			Line:          r.Line,
			Col:           r.Col,
			EndCol:        r.EndCol,
		})
	}

	vtabs := make([]RelationRef, 0, len(resp.VirtualTables))
	for _, v := range resp.VirtualTables {
		cols := make([]Column, 0, len(v.Columns))
		for _, c := range v.Columns {
			cols = append(cols, Column{
				Name:     c.Name,
				Type:     c.Type,
				Nullable: c.Nullable,
			})
		}
		vtabs = append(vtabs, RelationRef{
			Database:      v.Database,
			Schema:        v.Schema,
			Table:         v.Table,
			Alias:         v.Alias,
			IsVirtual:     v.IsVirtual,
			Columns:       cols,
			ScopeStartPos: v.ScopeStartOffset,
			ScopeEndPos:   v.ScopeEndOffset,
			NestingLevel:  v.NestingLevel,
			Line:          v.Line,
			Col:           v.Col,
			EndCol:        v.EndCol,
		})
	}

	return refs, vtabs, nil
}
