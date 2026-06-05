package workspace_to_user

import (
	"context"

	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

// Restore upserts the server-authoritative workspace_to_user row (called on LWW conflict).
func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	userID := utils.MapGetString(payload, "user_id")
	return queries.UpsertWorkspaceToUserForSync(ctx, generated.UpsertWorkspaceToUserForSyncParams{
		ID:          id,
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
}
