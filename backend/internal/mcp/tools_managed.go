package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/datasource/managed"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
)

var grantToSchema = map[string]any{
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
			"grant_to": grantToSchema,
		}, []string{"name"}),
		Annotations: &ToolAnnotations{ReadOnlyHint: boolPtr(false), DestructiveHint: boolPtr(false)},
		Run: func(_ context.Context, r *http.Request, _ string, rawArgs json.RawMessage) (any, error) {
			var args struct {
				Name    string          `json:"name"`
				GrantTo managed.GrantTo `json:"grant_to"`
			}
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return nil, errBadArgument("invalid arguments")
			}
			actor := authz.ActorOf(r)
			// As POST /datasources: adding a datasource takes manage on "*".
			if !actor.IsOwner() && !actor.Can(core.ActionManage) {
				audit.EmitDenied(r.Context(), audit.DatasourceCreated, actor.WorkspaceID, uuid.NewString())
				return nil, &toolError{Code: "forbidden", Message: "creating a datasource needs manage on the workspace"}
			}
			id, err := managed.Create(r.Context(), actor, args.Name, "", "", withCallerGranted(r, args.GrantTo))
			if err != nil {
				return nil, err
			}
			return createdDatabase{ID: id, DBType: "sqlite"}, nil
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
			"grant_to":      grantToSchema,
		}, []string{"datasource_id"}),
		Annotations: &ToolAnnotations{ReadOnlyHint: boolPtr(false), DestructiveHint: boolPtr(false)},
		Run: func(_ context.Context, r *http.Request, _ string, rawArgs json.RawMessage) (any, error) {
			var args struct {
				DatasourceID string          `json:"datasource_id"`
				Name         string          `json:"name"`
				At           string          `json:"at"`
				GrantTo      managed.GrantTo `json:"grant_to"`
			}
			if err := json.Unmarshal(rawArgs, &args); err != nil {
				return nil, errBadArgument("invalid arguments")
			}
			if args.DatasourceID == "" {
				return nil, errBadArgument("datasource_id is required")
			}
			actor := authz.ActorOf(r)
			// As POST /datasources/{id}/fork: a fork hands over all the source's data.
			if !actor.IsOwner() && !actor.CanManage(args.DatasourceID) {
				audit.EmitDenied(r.Context(), audit.DatasourceCreated, actor.WorkspaceID, args.DatasourceID)
				return nil, &toolError{Code: "forbidden", Message: "forking needs manage on the source"}
			}
			id, err := managed.Create(r.Context(), actor, args.Name, args.DatasourceID, args.At, withCallerGranted(r, args.GrantTo))
			if err != nil {
				return nil, err
			}
			return createdDatabase{ID: id, DBType: "sqlite"}, nil
		},
	}
}

// createdDatabase is what both tools answer: the id to query the new database by.
type createdDatabase struct {
	ID     string `json:"id"`
	DBType string `json:"db_type"`
}

// withCallerGranted adds the calling key to grants: an API key is never an
// owner, so it could not use a database its own role does not cover.
func withCallerGranted(r *http.Request, grants managed.GrantTo) managed.GrantTo {
	// An API key caller's UserID is its key id.
	callerKeyID := authz.ActorOf(r).UserID
	if !slices.Contains(grants.APIKeys, callerKeyID) {
		grants.APIKeys = append(grants.APIKeys, callerKeyID)
	}
	return grants
}
