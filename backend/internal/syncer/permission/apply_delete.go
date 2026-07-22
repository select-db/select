package permission

import (
	"context"
	"fmt"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/authz"
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
		return false, nil, fmt.Errorf("permission: missing id or workspace_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("permission: invalid id %q: %w", id, err)
	}

	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("permission: invalid workspace_id %q: %w", workspaceID, err)
	}

	// Fetch the current row for the audit before-snapshot; it also yields role_id
	// for cache invalidation (the payload may not include it).
	var before any
	roleID := utils.MapGetString(payload, "role_id")
	if perm, err := db.Queries.GetPermissionByID(ctx, generated.GetPermissionByIDParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err == nil {
		before = perm
		if roleID == "" {
			roleID = perm.RoleID.String()
		}
	}

	if err := db.Queries.SetPermissionDeletedAt(ctx, generated.SetPermissionDeletedAtParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	}); err != nil {
		return false, nil, fmt.Errorf("permission: set deleted_at: %w", err)
	}

	if roleID != "" {
		authz.Invalidate(roleID)
	}
	audit.EmitChange(ctx, audit.PermissionDeleted, workspaceID, id, before, nil)
	return true, nil, nil
}
