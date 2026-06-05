package workspace

import (
	"context"
	"database/sql"
	"fmt"
)

type WorkspaceUserRole struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	UserToRoleID string `json:"user_to_role_id"`
}

type WorkspaceUserEntry struct {
	ID      string              `json:"id"`
	Name    *string             `json:"name"`
	Email   *string             `json:"email"`
	Roles   []WorkspaceUserRole `json:"roles"`
	IsOwner bool                `json:"is_owner"`
}

func (w *Workspace) ListWorkspaceUsers() ([]WorkspaceUserEntry, error) {
	ctx := context.Background()
	wtu, err := w.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get current workspace: %w", err)
	}

	users, err := w.Queries.ListWorkspaceUsers(ctx, wtu.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace users: %w", err)
	}

	userRoles, err := w.Queries.ListUserRolesByWorkspace(ctx, wtu.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}

	rolesByUser := make(map[string][]WorkspaceUserRole, len(users))
	for _, r := range userRoles {
		rolesByUser[r.UserID] = append(rolesByUser[r.UserID], WorkspaceUserRole{
			ID:           r.RoleID,
			Name:         r.RoleName,
			UserToRoleID: r.UserToRoleID,
		})
	}

	out := make([]WorkspaceUserEntry, 0, len(users))
	for _, u := range users {
		roles := rolesByUser[u.ID]
		if roles == nil {
			roles = []WorkspaceUserRole{}
		}
		out = append(out, WorkspaceUserEntry{
			ID:      u.ID,
			Name:    u.Name.Ptr(),
			Email:   u.Email.Ptr(),
			Roles:   roles,
			IsOwner: u.IsOwner != 0,
		})
	}

	return out, nil
}
