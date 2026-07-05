package user_to_group

import (
	"context"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	userID := utils.MapGetString(payload, "user_id")
	groupID := utils.MapGetString(payload, "group_id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	if id == "" || userID == "" || groupID == "" || workspaceID == "" {
		return nil
	}
	source := utils.MapGetString(payload, "source")
	if source == "" {
		source = "local"
	}
	return queries.UpsertUserToGroupForSync(ctx, generated.UpsertUserToGroupForSyncParams{
		ID:          id,
		UserID:      userID,
		GroupID:     groupID,
		WorkspaceID: workspaceID,
		Source:      source,
	})
}
