package scope

import (
	"context"
	"database/sql"
	"errors"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
)

// RoleInWorkspace reports whether roleID exists and belongs to workspaceID.
// Sync apply handlers use it so a caller with roles.manage in one workspace
// cannot point a permission / user_to_role / role write at a role outside it.
func RoleInWorkspace(ctx context.Context, roleID, workspaceID db_types.JSONNullUUID) (bool, error) {
	// The by-id query is workspace-scoped, so a match means it's in-workspace.
	_, err := db.Queries.GetRoleByID(ctx, generated.GetRoleByIDParams{ID: roleID, WorkspaceID: workspaceID})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GroupInWorkspace reports whether groupID exists and belongs to workspaceID.
// Same purpose as RoleInWorkspace for user_to_group / group_to_role writes.
func GroupInWorkspace(ctx context.Context, groupID, workspaceID db_types.JSONNullUUID) (bool, error) {
	_, err := db.Queries.GetGroupByID(ctx, generated.GetGroupByIDParams{ID: groupID, WorkspaceID: workspaceID})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
