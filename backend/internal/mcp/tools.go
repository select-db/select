package mcp

import (
	"context"
	"encoding/json"
	"net/http"
)

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Annotations *ToolAnnotations
	Run         func(ctx context.Context, r *http.Request, workspaceID string, args json.RawMessage) (any, error)
}

// Hints for host clients; not security gates.
type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool `json:"openWorldHint,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

func defaultTools() []Tool {
	return []Tool{
		toolListDatasources(),
		toolGetDatabaseSchemas(),
		toolGetDatabaseTableDetail(),
		toolExecuteQuery(),
		toolPlanQuery(),
		toolExplainQuery(),
		toolExecuteStatement(),
	}
}

func jsonObjectSchema(properties map[string]any, required []string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func intProp(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}
