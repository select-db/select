package role

import (
	"context"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

func Restore(ctx context.Context, queries *generated.Queries, payload map[string]any) error {
	id := utils.MapGetString(payload, "id")
	workspaceID := utils.MapGetString(payload, "workspace_id")
	name := utils.MapGetString(payload, "name")
	if id == "" || workspaceID == "" {
		return nil
	}
	return queries.UpsertRoleForSync(ctx, generated.UpsertRoleForSyncParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        name,
	})
}
