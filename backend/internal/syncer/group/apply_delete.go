package group

import (
	"context"
	"fmt"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
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
		return false, nil, fmt.Errorf("group: missing id or workspace_id")
	}
	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("group: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("group: invalid workspace_id %q: %w", workspaceID, err)
	}
	// Snapshot the group (audit before-state) and its members before the delete:
	// removing the group removes every member's group-derived roles, so their
	// refresh tokens must be revoked to force a fresh token that no longer carries
	// those roles.
	var before any
	if g, err := db.Queries.GetGroupByID(ctx, idUUID); err == nil {
		before = g
	}
	members, membersErr := db.Queries.GetUserIDsByGroupID(ctx, idUUID)

	if err := db.Queries.SetGroupDeletedAt(ctx, generated.SetGroupDeletedAtParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return false, nil, fmt.Errorf("group: set deleted_at: %w", err)
	}

	// Best-effort: a revocation failure must not fail the sync.
	if membersErr == nil {
		for _, m := range members {
			_ = db.Queries.DeleteUserRefreshTokens(ctx, m)
		}
	}
	audit.EmitChange(ctx, audit.GroupDeleted, workspaceID, id, before, nil)
	return true, nil, nil
}
