package role

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

type RoleUserEntry struct {
	ID           string  `json:"id"`
	Name         *string `json:"name"`
	UserToRoleID string  `json:"user_to_role_id"`
}

func (r *Role) ListUsersInRole(roleID string) ([]RoleUserEntry, error) {
	rows, err := r.Queries.ListUsersByRole(context.Background(), roleID)
	if err != nil {
		return nil, fmt.Errorf("list users by role: %w", err)
	}
	out := make([]RoleUserEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, RoleUserEntry{
			ID:           row.ID,
			Name:         row.Name.Ptr(),
			UserToRoleID: row.UserToRoleID,
		})
	}
	return out, nil
}

func (r *Role) AssignUserToRole(userID, roleID string) error {
	ctx := context.Background()
	wtu, err := r.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no active workspace")
		}
		return fmt.Errorf("get current workspace: %w", err)
	}
	_, err = r.Queries.InsertUserToRole(ctx, generated.InsertUserToRoleParams{
		ID:          utils.GenerateUUID(),
		UserID:      userID,
		RoleID:      roleID,
		WorkspaceID: wtu.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("assign user to role: %w", err)
	}
	return nil
}

func (r *Role) RemoveUserFromRole(userToRoleID string) error {
	if err := r.Queries.RemoveUserFromRole(context.Background(), userToRoleID); err != nil {
		return fmt.Errorf("remove user from role: %w", err)
	}
	return nil
}
