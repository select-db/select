package user_to_role

import (
	"context"

	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	userID := utils.MapGetString(payload, "user_id")
	roleID := utils.MapGetString(payload, "role_id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	if id == "" || userID == "" || roleID == "" || workspaceID == "" {
		return nil
	}
	return queries.UpsertUserToRoleForSync(ctx, generated.UpsertUserToRoleForSyncParams{
		ID:          id,
		UserID:      userID,
		RoleID:      roleID,
		WorkspaceID: workspaceID,
	})
}
