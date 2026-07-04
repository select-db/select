package group_to_role

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
		return false, nil, fmt.Errorf("group_to_role: missing id or workspace_id")
	}
	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("group_to_role: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("group_to_role: invalid workspace_id %q: %w", workspaceID, err)
	}

	// Fetch before delete to fan token revocation out to the group's members:
	// removing a role from a group changes every member's effective roles.
	row, fetchErr := db.Queries.GetGroupToRoleByID(ctx, generated.GetGroupToRoleByIDParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	})

	if err := db.Queries.SetGroupToRoleDeletedAt(ctx, generated.SetGroupToRoleDeletedAtParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return false, nil, fmt.Errorf("group_to_role: set deleted_at: %w", err)
	}
	if fetchErr == nil {
		revokeGroupMembers(ctx, row.GroupID)
	}
	return true, nil, nil
}
