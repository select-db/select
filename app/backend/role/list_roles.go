package role

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/backend/db/generated"
)

// ListRoles returns all roles for the current workspace.
func (r *Role) ListRoles() ([]generated.ListRolesByWorkspaceRow, error) {
	ctx := context.Background()

	wtu, err := r.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get current workspace: %w", err)
	}

	rows, err := r.Queries.ListRolesByWorkspace(ctx, wtu.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	return rows, nil
}
