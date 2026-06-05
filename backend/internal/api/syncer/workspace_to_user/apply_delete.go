package workspace_to_user

import (
	"context"
	"fmt"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/api/syncer/types"
	"backend/internal/utils"
)

// ApplyDelete sets deleted_at and updated_at for the workspace_to_user row identified by the commit.
// Uses payload id and commit WorkspaceID, or ObjectID as id when payload is minimal.
func ApplyDelete(ctx context.Context, userID string, c types.Commit) (bool, *types.RestoredItem, error) {
	payload, _ := c.Payload.(map[string]any)
	id := utils.MapGetString(payload, "id")
	if id == "" {
		id = c.ObjectID
	}
	workspaceID := c.WorkspaceID
	if id == "" || workspaceID == "" {
		return false, nil, fmt.Errorf("workspace_to_user: missing id or workspace_id")
	}
	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: invalid workspace_id %q: %w", workspaceID, err)
	}
	// Prevent removing the workspace owner
	wtu, err := db.Queries.GetWorkspaceToUserByID(ctx, generated.GetWorkspaceToUserByIDParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: fetch row: %w", err)
	}
	ownerID, err := db.Queries.GetWorkspaceOwnerID(ctx, workspaceUUID)
	if err == nil && ownerID.String() == wtu.UserID.String() {
		return false, nil, fmt.Errorf("workspace_to_user: cannot remove workspace owner")
	}

	if err := db.Queries.SetWorkspaceToUserDeletedAt(ctx, generated.SetWorkspaceToUserDeletedAtParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: set deleted_at: %w", err)
	}
	return true, nil, nil
}
