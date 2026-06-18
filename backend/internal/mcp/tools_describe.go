package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/authz"
	"backend/internal/datasource"

	"github.com/google/uuid"
	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

// ----------------------------------------------------------------------
// list_datasources
// ----------------------------------------------------------------------

func toolListDatasources() Tool {
	return Tool{
		Name: "list_datasources",
		Description: "Lists datasources the calling API key's role can touch in the current " +
			"workspace. Each entry returns the role's permission rules on that datasource " +
			"as `effect.action.schema.table.column` strings, padded to 5 segments. " +
			"Datasources the role has no allow rules on are omitted (querying them would " +
			"be denied by default).\n\n" +
			"Examples: `allow.select.public.*.*` (read any column of any table in public), " +
			"`deny.select.public.users.password` (deny that one column), " +
			"`allow.insert.public.events.*` (insert any column of events). " +
			"Deny rules win at the most specific level they match. " +
			"Action values: select, insert, update, delete, ddl.",
		InputSchema: jsonObjectSchema(nil, nil),
		Annotations: &ToolAnnotations{
			ReadOnlyHint:   boolPtr(true),
			IdempotentHint: boolPtr(true),
		},
		Run: func(ctx context.Context, r *http.Request, workspaceID string, _ json.RawMessage) (any, error) {
			parsedWS, err := uuid.Parse(workspaceID)
			if err != nil {
				return nil, errBadArgument("invalid workspace id")
			}
			rows, err := db.Queries.ListDatasourcesByWorkspace(ctx, db_types.NewJSONNullUUID(parsedWS))
			if err != nil {
				return nil, errUpstream("could not list datasources: " + err.Error())
			}

			// Wildcard-DB entries (DbInstanceID "*") apply to every datasource
			entries := authz.EntriesFromRequest(r)
			scopedByDB, wildcardEntries := indexEntriesByDB(entries)

			type item struct {
				ID          string   `json:"id"`
				Name        string   `json:"name"`
				Dialect     string   `json:"dialect"`
				Permissions []string `json:"permissions"`
			}

			out := make([]item, 0, len(rows))
			for _, row := range rows {
				id := row.ID.String()
				specific := scopedByDB[id]
				if !hasAllowEntry(specific) && !hasAllowEntry(wildcardEntries) {
					continue
				}
				combined := make([]string, 0, len(specific)+len(wildcardEntries))
				for _, e := range specific {
					combined = append(combined, ruleString(e))
				}
				for _, e := range wildcardEntries {
					combined = append(combined, ruleString(e))
				}
				out = append(out, item{
					ID:          id,
					Name:        row.Name.String,
					Dialect:     row.DbType.String,
					Permissions: combined,
				})
			}
			return map[string]any{"datasources": out}, nil
		},
	}
}

// indexEntriesByDB splits entries into per-DB and wildcard buckets.
// Nil DbInstanceID entries (workspace-level) are skipped.
func indexEntriesByDB(entries []core.PermissionEntry) (perDB map[string][]core.PermissionEntry, wildcard []core.PermissionEntry) {
	perDB = map[string][]core.PermissionEntry{}
	for _, e := range entries {
		if e.DbInstanceID == nil {
			continue
		}
		id := *e.DbInstanceID
		if id == "*" {
			wildcard = append(wildcard, e)
			continue
		}
		perDB[id] = append(perDB[id], e)
	}
	return perDB, wildcard
}

// ruleString renders a permission entry as effect.action.schema.table.column
func ruleString(e core.PermissionEntry) string {
	return strings.Join([]string{
		e.Effect,
		e.Action,
		wildIfNil(e.SchemaName),
		wildIfNil(e.TableName),
		wildIfNil(e.ColumnName),
	}, ".")
}

func wildIfNil(s *string) string {
	if s == nil || *s == "" {
		return "*"
	}
	return *s
}

func hasAllowEntry(entries []core.PermissionEntry) bool {
	for _, e := range entries {
		if e.Effect == "allow" {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------
// get_database_schemas
// ----------------------------------------------------------------------

func toolGetDatabaseSchemas() Tool {
	return Tool{
		Name: "get_database_schemas",
		Description: "Returns all schemas for a datasource with table and view names in each. " +
			"Use this first; then call get_database_table_detail(schemaId, tableName) when you need a table's DDL.",
		InputSchema: jsonObjectSchema(map[string]any{
			"datasource_id": stringProp("Datasource ID (required)."),
		}, []string{"datasource_id"}),
		Annotations: &ToolAnnotations{
			ReadOnlyHint:   boolPtr(true),
			IdempotentHint: boolPtr(true),
		},
		Run: func(ctx context.Context, _ *http.Request, workspaceID string, raw json.RawMessage) (any, error) {
			var args struct {
				DatasourceID string `json:"datasource_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil || args.DatasourceID == "" {
				return nil, errBadArgument("datasource_id is required")
			}
			meta, dialect, err := loadMetadata(ctx, args.DatasourceID, workspaceID)
			if err != nil {
				return nil, err
			}

			type schemaRow struct {
				ID     string   `json:"id"`
				Name   string   `json:"name"`
				Tables []string `json:"tables"`
				Views  []string `json:"views,omitempty"`
			}
			out := make([]schemaRow, 0, len(meta.Schemas))
			for _, s := range meta.Schemas {
				row := schemaRow{ID: s.Name, Name: s.Name}
				for _, t := range s.Tables {
					row.Tables = append(row.Tables, t.Name)
				}
				for _, v := range s.Views {
					row.Views = append(row.Views, v.Name)
				}
				out = append(out, row)
			}
			return map[string]any{
				"datasource_id": args.DatasourceID,
				"dialect":       dialect.Name(),
				"schemas":       out,
			}, nil
		},
	}
}

// ----------------------------------------------------------------------
// get_database_table_detail
// ----------------------------------------------------------------------

func toolGetDatabaseTableDetail() Tool {
	return Tool{
		Name: "get_database_table_detail",
		Description: "Returns the DDL for a single table or view. Call after get_database_schemas " +
			"when you need the full definition (columns, types, constraints) to write or validate queries.",
		InputSchema: jsonObjectSchema(map[string]any{
			"datasource_id": stringProp("Datasource ID (required)."),
			"schema_id":     stringProp("Schema ID from get_database_schemas(...).schemas[].id"),
			"table_name":    stringProp("Table or view name from get_database_schemas(...).schemas[].tables[] or .views[]"),
		}, []string{"datasource_id", "schema_id", "table_name"}),
		Annotations: &ToolAnnotations{
			ReadOnlyHint:   boolPtr(true),
			IdempotentHint: boolPtr(true),
		},
		Run: func(ctx context.Context, _ *http.Request, workspaceID string, raw json.RawMessage) (any, error) {
			var args struct {
				DatasourceID string `json:"datasource_id"`
				SchemaID     string `json:"schema_id"`
				TableName    string `json:"table_name"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, errBadArgument("invalid arguments")
			}
			if args.DatasourceID == "" || args.SchemaID == "" || args.TableName == "" {
				return nil, errBadArgument("datasource_id, schema_id, and table_name are required")
			}
			meta, _, err := loadMetadata(ctx, args.DatasourceID, workspaceID)
			if err != nil {
				return nil, err
			}
			for _, s := range meta.Schemas {
				if s.Name != args.SchemaID {
					continue
				}
				if t := findRelation(s, args.TableName); t != nil {
					return map[string]any{
						"datasource_id": args.DatasourceID,
						"schema_id":     args.SchemaID,
						"table_name":    t.Name,
						"ddl":           t.DDL,
					}, nil
				}
				return nil, errNotFound(fmt.Sprintf("table or view %q not found in schema %q", args.TableName, args.SchemaID))
			}
			return nil, errNotFound(fmt.Sprintf("schema %q not found", args.SchemaID))
		},
	}
}

// ----------------------------------------------------------------------
// shared helpers
// ----------------------------------------------------------------------

func loadMetadata(ctx context.Context, datasourceID, workspaceID string) (*core.Metadata, core.SQLDialect, error) {
	ds, err := datasource.GetOrLoadDatasource(ctx, datasourceID, workspaceID)
	if err != nil {
		return nil, nil, errNotFound("datasource not found")
	}
	dbConn, err := engine.GetOrOpenConn(workspaceID, ds.DBType, ds.DSN, ds.SSH, ds.Pool)
	if err != nil {
		return nil, nil, errUpstream("could not open datasource connection: " + err.Error())
	}
	d := engine.GetDialect(ds.DBType)
	if d == nil {
		return nil, nil, errExecution("unsupported database type: "+ds.DBType, "")
	}
	meta, err := engine.GetOrFetchMetadata(ctx, workspaceID, ds.DSN, dbConn, d, "", false)
	if err != nil {
		return nil, nil, errUpstream("could not fetch metadata: " + err.Error())
	}
	return meta, d, nil
}

// findRelation looks up a table or view by name; tables first, then views
func findRelation(s core.Schema, name string) *core.Table {
	for i := range s.Tables {
		if s.Tables[i].Name == name {
			return &s.Tables[i]
		}
	}
	for i := range s.Views {
		if s.Views[i].Name == name {
			return &s.Views[i]
		}
	}
	return nil
}
