package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/datasource"
)

var grantToProp = map[string]any{
	"type":        "object",
	"description": "Who else gets full access to the new database: workspace user ids and API key ids. The calling key always gets it.",
	"properties": map[string]any{
		"users":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"api_keys": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	},
}

func toolCreateDatasource() Tool {
	return Tool{
		Name: "create_datasource",
		Description: "Creates an empty managed SQLite database in the workspace and returns its id. " +
			"The caller gets full access to it; use execute_statement to create its tables.",
		InputSchema: jsonObjectSchema(map[string]any{
			"name":     stringProp("Display name of the database (required)."),
			"grant_to": grantToProp,
		}, []string{"name"}),
		Annotations: &ToolAnnotations{ReadOnlyHint: boolPtr(false), DestructiveHint: boolPtr(false)},
		Run: func(_ context.Context, r *http.Request, _ string, raw json.RawMessage) (any, error) {
			var args struct {
				Name    string             `json:"name"`
				GrantTo datasource.GrantTo `json:"grant_to"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, errBadArgument("invalid arguments")
			}
			if args.Name == "" {
				return nil, errBadArgument("name is required")
			}
			id, err := datasource.CreateManaged(r, args.Name, "", "", args.GrantTo)
			if err != nil {
				return nil, err
			}
			return created{ID: id, DBType: "sqlite"}, nil
		},
	}
}

func toolForkDatasource() Tool {
	return Tool{
		Name: "fork_datasource",
		Description: "Copies a managed database into a new one and returns the new id; the source is never changed. " +
			"Use it to try changes on a copy. Needs manage on the source.",
		InputSchema: jsonObjectSchema(map[string]any{
			"datasource_id": stringProp("Managed database to copy (required)."),
			"name":          stringProp("Display name of the copy. Defaults to the source's name followed by (fork)."),
			"at":            stringProp("RFC 3339 time to copy the source as it was then, within the plan's retention. Omit for now."),
			"grant_to":      grantToProp,
		}, []string{"datasource_id"}),
		Annotations: &ToolAnnotations{ReadOnlyHint: boolPtr(false), DestructiveHint: boolPtr(false)},
		Run: func(_ context.Context, r *http.Request, _ string, raw json.RawMessage) (any, error) {
			var args struct {
				DatasourceID string             `json:"datasource_id"`
				Name         string             `json:"name"`
				At           string             `json:"at"`
				GrantTo      datasource.GrantTo `json:"grant_to"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, errBadArgument("invalid arguments")
			}
			if args.DatasourceID == "" {
				return nil, errBadArgument("datasource_id is required")
			}
			id, err := datasource.CreateManaged(r, args.Name, args.DatasourceID, args.At, args.GrantTo)
			if err != nil {
				return nil, err
			}
			return created{ID: id, DBType: "sqlite"}, nil
		},
	}
}

// created is what both tools answer: the id to query the new database by.
type created struct {
	ID     string `json:"id"`
	DBType string `json:"db_type"`
}
