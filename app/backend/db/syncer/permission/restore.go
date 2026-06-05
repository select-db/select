package permission

import (
	"context"

	"selectDb/backend/db/db_types"
	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
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
		DbInstanceID: db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "db_instance_id")),
		SchemaName:   db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "schema_name")),
		TableName:    db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "table_name")),
		ColumnName:   db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "column_name")),
		Action:       action,
		Effect:       effect,
	})
}
