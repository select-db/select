package user_to_group

import (
	"context"
	"fmt"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/syncer/types"
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
		return false, nil, fmt.Errorf("user_to_group: missing id or workspace_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_group: invalid id %q: %w", id, err)
	}

	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_group: invalid workspace_id %q: %w", workspaceID, err)
	}

	// Fetch before delete to get user_id for token revocation.
	row, fetchErr := db.Queries.GetUserToGroupByID(ctx, generated.GetUserToGroupByIDParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	})

	if err := db.Queries.SetUserToGroupDeletedAt(ctx, generated.SetUserToGroupDeletedAtParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return false, nil, fmt.Errorf("user_to_group: set deleted_at: %w", err)
	}
	if fetchErr == nil {
		_ = db.Queries.DeleteUserRefreshTokens(ctx, row.UserID)
	}
	return true, nil, nil
}
