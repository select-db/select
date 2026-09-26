package permission

import (
	"context"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	roleID := utils.MapGetString(payload, "role_id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	action := utils.MapGetString(payload, "action")
	effect := utils.MapGetString(payload, "effect")
	if id == "" || roleID == "" || workspaceID == "" || action == "" {
		return nil
	}
	if effect == "" {
		effect = "deny"
	}
	return queries.UpsertPermissionForSync(ctx, generated.UpsertPermissionForSyncParams{
		ID:           id,
		RoleID:       roleID,
		WorkspaceID:  workspaceID,
		DatasourceID: db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "datasource_id")),
		SchemaName:   db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "schema_name")),
		TableName:    db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "table_name")),
		ColumnName:   db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "column_name")),
		Action:       action,
		Effect:       effect,
	})
}
