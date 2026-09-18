package group

import (
	"context"

	"selectDb/internal/db/db_types"
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
	source := utils.MapGetString(payload, "source")
	if source == "" {
		source = "local"
	}
	return queries.UpsertGroupForSync(ctx, generated.UpsertGroupForSyncParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        name,
		Source:      source,
		ExternalID:  db_types.NewJSONNullStringFromPtr(utils.MapGetStringPtr(payload, "external_id")),
	})
}
