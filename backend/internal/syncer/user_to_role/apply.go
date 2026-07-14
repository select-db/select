package user_to_role

import (
	"context"
	"fmt"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/syncer/patch"
	"backend/internal/syncer/scope"
	"backend/internal/syncer/types"
	"backend/internal/utils"
)

func Apply(ctx context.Context, userID string, c types.Commit, lastPulledAt time.Time) (bool, *types.RestoredItem, error) {
	payload, ok := c.Payload.(map[string]any)
	if !ok {
		return false, nil, fmt.Errorf("user_to_role: payload must be an object")
	}

	id := utils.MapGetString(payload, "id")
	workspaceID := c.WorkspaceID
	uid := utils.MapGetString(payload, "user_id")
	roleID := utils.MapGetString(payload, "role_id")
	if id == "" || workspaceID == "" || uid == "" || roleID == "" {
		return false, nil, fmt.Errorf("user_to_role: missing id, workspace_id, user_id or role_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_role: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_role: invalid workspace_id %q: %w", workspaceID, err)
	}
	userUUID, err := db_types.NewJSONNullUUIDFromString(uid)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_role: invalid user_id %q: %w", uid, err)
	}
	roleUUID, err := db_types.NewJSONNullUUIDFromString(roleID)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_role: invalid role_id %q: %w", roleID, err)
	}

	// Target role must belong to this workspace, else a roles.manage holder
	// could bind a user to a role in another workspace (privilege escalation).
	if ok, err := scope.RoleInWorkspace(ctx, roleUUID, workspaceUUID); err != nil {
		return false, nil, fmt.Errorf("user_to_role: validate role: %w", err)
	} else if !ok {
		return false, nil, fmt.Errorf("user_to_role: role_id does not belong to workspace")
	}

	res, err := patch.Apply(ctx, c, patch.Handler[generated.AppUserToRole, generated.UpsertUserToRoleParams]{
		TableName: "user_to_role",
		Fetch: func(ctx context.Context) (generated.AppUserToRole, error) {
			return db.Queries.GetUserToRoleByID(ctx, generated.GetUserToRoleByIDParams{
				ID:          idUUID,
				WorkspaceID: workspaceUUID,
			})
		},
		UpdatedAt: func(row generated.AppUserToRole) time.Time {
			return row.UpdatedAt.ValueOrZero()
		},
		DeletedAt: func(row generated.AppUserToRole) *time.Time {
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				return &t
			}
			return nil
		},
		Restored: func(row generated.AppUserToRole) (interface{}, error) {
			return types.ToRestoredPayload(appUserToRoleToTypesRow(row))
		},
		Merge: func(existing generated.AppUserToRole, isNew bool, payload map[string]any) (generated.UpsertUserToRoleParams, error) {
			uid := userUUID
			rid := roleUUID
			wid := workspaceUUID
			if !isNew {
				uid = utils.PatchUUID(payload, "user_id", existing.UserID, userUUID)
				rid = utils.PatchUUID(payload, "role_id", existing.RoleID, roleUUID)
				wid = utils.PatchUUID(payload, "workspace_id", existing.WorkspaceID, workspaceUUID)
			}
			return generated.UpsertUserToRoleParams{
				ID:          idUUID,
				UserID:      uid,
				RoleID:      rid,
				WorkspaceID: wid,
			}, nil
		},
		Upsert: func(ctx context.Context, params generated.UpsertUserToRoleParams) error {
			if err := db.Queries.UpsertUserToRole(ctx, params); err != nil {
				return err
			}
			_ = db.Queries.DeleteUserRefreshTokens(ctx, userUUID)
			return nil
		},
	})
	return res.Applied, res.Restored, err
}
