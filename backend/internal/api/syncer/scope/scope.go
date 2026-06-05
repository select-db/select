package scope

import (
	"context"
	"database/sql"
	"errors"

	"backend/db"
	"backend/db/db_types"
)

// RoleInWorkspace reports whether roleID exists and belongs to workspaceID.
// Sync apply handlers use it so a caller with roles.manage in one workspace
// cannot point a permission / user_to_role / role write at a role outside it.
func RoleInWorkspace(ctx context.Context, roleID, workspaceID db_types.JSONNullUUID) (bool, error) {
	role, err := db.Queries.GetRoleByID(ctx, roleID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return role.WorkspaceID == workspaceID, nil
}
