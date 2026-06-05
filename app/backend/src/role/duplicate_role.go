package role

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/backend/db/generated"
	"selectDb/backend/utils"
)

func (r *Role) DuplicateRole(id string) (generated.Role, error) {
	ctx := context.Background()

	wtu, err := r.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return generated.Role{}, fmt.Errorf("no active workspace")
		}
		return generated.Role{}, fmt.Errorf("get current workspace: %w", err)
	}

	// Find original role name from the workspace roles list
	roles, err := r.Queries.ListRolesByWorkspace(ctx, wtu.WorkspaceID)
	if err != nil {
		return generated.Role{}, fmt.Errorf("list roles: %w", err)
	}

	var originalName string
	for _, role := range roles {
		if role.ID == id {
			originalName = role.Name
			break
		}
	}
	if originalName == "" {
		return generated.Role{}, fmt.Errorf("role not found")
	}

	newRole, err := r.Queries.InsertRole(ctx, generated.InsertRoleParams{
		ID:          utils.GenerateUUID(),
		WorkspaceID: wtu.WorkspaceID,
		Name:        "Copy of " + originalName,
	})
	if err != nil {
		return generated.Role{}, fmt.Errorf("insert duplicated role: %w", err)
	}

	perms, err := r.Queries.ListPermissionsByRole(ctx, id)
	if err != nil {
		return generated.Role{}, fmt.Errorf("list permissions: %w", err)
	}

	for _, p := range perms {
		_, err := r.Queries.InsertPermission(ctx, generated.InsertPermissionParams{
			ID:           utils.GenerateUUID(),
			RoleID:       newRole.ID,
			DbInstanceID: p.DbInstanceID,
			SchemaName:   p.SchemaName,
			TableName:    p.TableName,
			ColumnName:   p.ColumnName,
			Action:       p.Action,
			Effect:       p.Effect,
		})
		if err != nil {
			return generated.Role{}, fmt.Errorf("copy permission: %w", err)
		}
	}

	return newRole, nil
}
