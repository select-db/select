package permission

import (
	"context"
	"fmt"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/authz"
	"backend/internal/syncer/patch"
	"backend/internal/syncer/scope"
	"backend/internal/syncer/types"
	"backend/internal/utils"
)

func Apply(ctx context.Context, userID string, c types.Commit, lastPulledAt time.Time) (bool, *types.RestoredItem, error) {
	payload, ok := c.Payload.(map[string]any)
	if !ok {
		return false, nil, fmt.Errorf("permission: payload must be an object")
	}

	id := utils.MapGetString(payload, "id")
	roleID := utils.MapGetString(payload, "role_id")
	workspaceID := c.WorkspaceID
	if id == "" || roleID == "" || workspaceID == "" {
		return false, nil, fmt.Errorf("permission: missing id, role_id or workspace_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("permission: invalid id %q: %w", id, err)
	}
	roleUUID, err := db_types.NewJSONNullUUIDFromString(roleID)
	if err != nil {
		return false, nil, fmt.Errorf("permission: invalid role_id %q: %w", roleID, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("permission: invalid workspace_id %q: %w", workspaceID, err)
	}

	// Target role must belong to this workspace, else a roles.manage holder
	// could grant permissions to a role in another workspace.
	if ok, err := scope.RoleInWorkspace(ctx, roleUUID, workspaceUUID); err != nil {
		return false, nil, fmt.Errorf("permission: validate role: %w", err)
	} else if !ok {
		return false, nil, fmt.Errorf("permission: role_id does not belong to workspace")
	}

	return patch.Apply(ctx, c, patch.Handler[generated.AppPermission, generated.UpsertPermissionParams]{
		TableName: "permission",
		Fetch: func(ctx context.Context) (generated.AppPermission, error) {
			return db.Queries.GetPermissionByID(ctx, generated.GetPermissionByIDParams{
				ID:          idUUID,
				WorkspaceID: workspaceUUID,
			})
		},
		UpdatedAt: func(row generated.AppPermission) time.Time {
			return row.UpdatedAt.ValueOrZero()
		},
		DeletedAt: func(row generated.AppPermission) *time.Time {
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				return &t
			}
			return nil
		},
		Restored: func(row generated.AppPermission) (interface{}, error) {
			return types.ToRestoredPayload(appPermissionToTypesRow(row))
		},
		Merge: func(existing generated.AppPermission, isNew bool, payload map[string]any) (generated.UpsertPermissionParams, error) {
			return generated.UpsertPermissionParams{
				ID:           idUUID,
				RoleID:       roleUUID,
				WorkspaceID:  workspaceUUID,
				DbInstanceID: utils.PatchNullStr(payload, "db_instance_id", existing.DbInstanceID),
				SchemaName:   utils.PatchNullStr(payload, "schema_name", existing.SchemaName),
				TableName:    utils.PatchNullStr(payload, "table_name", existing.TableName),
				ColumnName:   utils.PatchNullStr(payload, "column_name", existing.ColumnName),
				Action:       utils.PatchStr(payload, "action", existing.Action),
				Effect:       utils.PatchStr(payload, "effect", existing.Effect),
			}, nil
		},
		Upsert: func(ctx context.Context, params generated.UpsertPermissionParams) error {
			if err := db.Queries.UpsertPermission(ctx, params); err != nil {
				return err
			}
			authz.Invalidate(roleID)
			return nil
		},
	})
}
