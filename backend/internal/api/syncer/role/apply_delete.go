package role

import (
	"context"
	"fmt"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/api/syncer/types"
	"backend/internal/utils"
)

func ApplyDelete(ctx context.Context, userID string, c types.Commit) (bool, *types.RestoredItem, error) {
	payload, _ := c.Payload.(map[string]any)
	id := utils.MapGetString(payload, "id")
	if id == "" {
		id = c.ObjectID
	}
	workspaceID := c.WorkspaceID
	if id == "" || workspaceID == "" {
		return false, nil, fmt.Errorf("role: missing id or workspace_id")
	}
	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("role: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("role: invalid workspace_id %q: %w", workspaceID, err)
	}
	if err := db.Queries.SetRoleDeletedAt(ctx, generated.SetRoleDeletedAtParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return false, nil, fmt.Errorf("role: set deleted_at: %w", err)
	}
	return true, nil, nil
}
