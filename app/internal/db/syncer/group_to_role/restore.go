package group_to_role

import (
	"context"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	groupID := utils.MapGetString(payload, "group_id")
	roleID := utils.MapGetString(payload, "role_id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	if id == "" || groupID == "" || roleID == "" || workspaceID == "" {
		return nil
	}
	return queries.UpsertGroupToRoleForSync(ctx, generated.UpsertGroupToRoleForSyncParams{
		ID:          id,
		GroupID:     groupID,
		RoleID:      roleID,
		WorkspaceID: workspaceID,
	})
}
