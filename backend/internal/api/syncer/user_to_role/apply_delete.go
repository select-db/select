package user_to_role

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
		return false, nil, fmt.Errorf("user_to_role: missing id or workspace_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_role: invalid id %q: %w", id, err)
	}

	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_role: invalid workspace_id %q: %w", workspaceID, err)
	}

	// Fetch before delete to get user_id for token revocation
	row, fetchErr := db.Queries.GetUserToRoleByID(ctx, generated.GetUserToRoleByIDParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	})

	if err := db.Queries.SetUserToRoleDeletedAt(ctx, generated.SetUserToRoleDeletedAtParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return false, nil, fmt.Errorf("user_to_role: set deleted_at: %w", err)
	}
	if fetchErr == nil {
		_ = db.Queries.DeleteUserRefreshTokens(ctx, row.UserID)
	}
	return true, nil, nil
}
