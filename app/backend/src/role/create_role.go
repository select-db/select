package role

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

func (r *Role) CreateRole(name string) (generated.Role, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return generated.Role{}, fmt.Errorf("role name cannot be empty")
	}

	ctx := context.Background()

	wtu, err := r.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return generated.Role{}, fmt.Errorf("no active workspace")
		}
		return generated.Role{}, fmt.Errorf("get current workspace: %w", err)
	}

	row, err := r.Queries.InsertRole(ctx, generated.InsertRoleParams{
		ID:          utils.GenerateUUID(),
		WorkspaceID: wtu.WorkspaceID,
		Name:        name,
	})
	if err != nil {
		return generated.Role{}, fmt.Errorf("create role: %w", err)
	}

	return row, nil
}
