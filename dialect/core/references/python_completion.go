package core_references

import (
	"encoding/json"
	"fmt"

	core "github.com/selectDb/dialect/core"
)

type pyCompletionContext struct {
	Parts             []string               `json:"parts"`
	CaretAfterDot     bool                   `json:"caret_after_dot"`
	Targets           core.CompletionTarget  `json:"targets"`
	SchemaFilter      string                 `json:"schema_filter"`
	TargetTable       string                 `json:"target_table"`
	KeywordContext    core.CompletionTarget  `json:"keyword_context"`
	PrecedingColumn   *pyPrecedingColumnInfo `json:"preceding_column"`
	InsertTargetTable string                 `json:"insert_target_table"`
	ValuePosition     bool                   `json:"value_position"`
}

type pyPrecedingColumnInfo struct {
	Name string `json:"name"`
}

// ParseCompletionContextFromPython calls the Python analyzer to detect the
// completion context at the given caret position.
func ParseCompletionContextFromPython(
	analyzer core.Analyzer,
	sql string,
	caretLine int,
	caretCol int,
	meta core.Metadata,
) (core.CompletionContext, error) {
	if analyzer == nil {
		return core.CompletionContext{}, fmt.Errorf("analyzer is nil")
	}

	req := map[string]any{
		"action":     "complete_context",
		"sql":        sql,
		"caret_line": caretLine,
		"caret_col":  caretCol,
		"schema":     core.MetaToSchemaDict(meta),
	}

	raw, err := analyzer.Call(req)
	if err != nil {
		return core.CompletionContext{}, fmt.Errorf("complete_context call: %w", err)
	}

	var resp pyCompletionContext
	if err := json.Unmarshal(raw, &resp); err != nil {
		return core.CompletionContext{}, fmt.Errorf("complete_context unmarshal: %w", err)
	}

	ctx := core.CompletionContext{
		Parts:             resp.Parts,
		CaretAfterDot:     resp.CaretAfterDot,
		Targets:           resp.Targets,
		SchemaFilter:      resp.SchemaFilter,
		TargetTable:       resp.TargetTable,
		KeywordContext:     resp.KeywordContext,
		InsertTargetTable: resp.InsertTargetTable,
		ValuePosition:     resp.ValuePosition,
	}

	if resp.PrecedingColumn != nil {
		ctx.PrecedingColumn = &core.PrecedingColumnInfo{
			Name: resp.PrecedingColumn.Name,
		}
	}

	return ctx, nil
}
