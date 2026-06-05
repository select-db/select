package permission

import (
	"context"
	"fmt"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/api/syncer/types"
	permcache "backend/internal/permission"
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
	
	// Look up role_id for cache invalidation (payload may not include it)
	roleID := utils.MapGetString(payload, "role_id")
	if roleID == "" {
		if perm, err := db.Queries.GetPermissionByID(ctx, generated.GetPermissionByIDParams{
			ID:          idUUID,
			WorkspaceID: workspaceUUID,
		}); err == nil {
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
		permcache.Invalidate(roleID)
	}
	return true, nil, nil
}
