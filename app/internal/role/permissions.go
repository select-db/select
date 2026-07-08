package role

import (
	"context"
	"fmt"

	"selectDb/internal/db/db_types"
	"selectDb/internal/db/generated"
	"selectDb/internal/utils"

	core "github.com/selectDb/dialect/core"
)

type PermissionEntry struct {
	ID           string  `json:"id"`
	RoleID       string  `json:"role_id"`
	WorkspaceID  string  `json:"workspace_id"`
	DbInstanceID *string `json:"db_instance_id"`
	SchemaName   *string `json:"schema_name"`
	TableName    *string `json:"table_name"`
	ColumnName   *string `json:"column_name"`
	Action       string  `json:"action"`
	Effect       string  `json:"effect"`
}

type AddPermissionParams struct {
	RoleID       string  `json:"role_id"`
	DbInstanceID *string `json:"db_instance_id"`
	SchemaName   *string `json:"schema_name"`
	TableName    *string `json:"table_name"`
	ColumnName   *string `json:"column_name"`
	Action       string  `json:"action"`
	Effect       string  `json:"effect"`
}

func (r *Role) ListPermissions(roleID string) ([]PermissionEntry, error) {
	rows, err := r.Queries.ListPermissionsByRole(context.Background(), roleID)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	out := make([]PermissionEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, PermissionEntry{
			ID:           row.ID,
			RoleID:       row.RoleID,
			WorkspaceID:  row.WorkspaceID,
			DbInstanceID: row.DbInstanceID.Ptr(),
			SchemaName:   row.SchemaName.Ptr(),
			TableName:    row.TableName.Ptr(),
			ColumnName:   row.ColumnName.Ptr(),
			Action:       row.Action,
			Effect:       row.Effect,
		})
	}
	return out, nil
}

func (r *Role) AddPermission(params AddPermissionParams) (PermissionEntry, error) {
	if params.Action == "" {
		return PermissionEntry{}, fmt.Errorf("action cannot be empty")
	}
	effect := params.Effect
	if effect == "" {
		effect = "deny"
	}

	workspaceID, err := r.Queries.GetRoleWorkspaceID(context.Background(), params.RoleID)
	if err != nil {
		return PermissionEntry{}, fmt.Errorf("add permission: lookup workspace: %w", err)
	}

	row, err := r.Queries.InsertPermission(context.Background(), generated.InsertPermissionParams{
		ID:           utils.GenerateUUID(),
		RoleID:       params.RoleID,
		WorkspaceID:  workspaceID,
		DbInstanceID: db_types.NewJSONNullStringFromPtr(params.DbInstanceID),
		SchemaName:   db_types.NewJSONNullStringFromPtr(params.SchemaName),
		TableName:    db_types.NewJSONNullStringFromPtr(params.TableName),
		ColumnName:   db_types.NewJSONNullStringFromPtr(params.ColumnName),
		Action:       params.Action,
		Effect:       effect,
	})
	if err != nil {
		return PermissionEntry{}, fmt.Errorf("add permission: %w", err)
	}

	return PermissionEntry{
		ID:           row.ID,
		RoleID:       row.RoleID,
		WorkspaceID:  row.WorkspaceID,
		DbInstanceID: row.DbInstanceID.Ptr(),
		SchemaName:   row.SchemaName.Ptr(),
		TableName:    row.TableName.Ptr(),
		ColumnName:   row.ColumnName.Ptr(),
		Action:       row.Action,
		Effect:       row.Effect,
	}, nil
}

func (r *Role) RemovePermission(id string) error {
	if err := r.Queries.DeletePermission(context.Background(), id); err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}
	return nil
}

func (r *Role) GetMyPermissions() ([]core.PermissionEntry, error) {
	return GetMyPermissions(r.Queries)
}

// GetMyPermissions returns the current user's permissions for the current workspace.
func GetMyPermissions(queries *generated.Queries) ([]core.PermissionEntry, error) {
	ctx := context.Background()
	user, err := queries.GetCurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("get my permissions: current user: %w", err)
	}

	ws, err := queries.GetCurrentWorkspace(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("get my permissions: current workspace: %w", err)
	}

	rows, err := queries.ListMyPermissions(ctx, generated.ListMyPermissionsParams{
		UserID:      user.ID,
		WorkspaceID: ws.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("get my permissions: %w", err)
	}

	out := make([]core.PermissionEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, core.PermissionEntry{
			DbInstanceID: row.DbInstanceID.Ptr(),
			SchemaName:   row.SchemaName.Ptr(),
			TableName:    row.TableName.Ptr(),
			ColumnName:   row.ColumnName.Ptr(),
			Action:       row.Action,
			Effect:       row.Effect,
			RoleName:     row.RoleName.String,
		})
	}
	return out, nil
}
