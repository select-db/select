package user_to_group

import (
	"context"
	"fmt"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/syncer/patch"
	"backend/internal/syncer/scope"
	"backend/internal/syncer/types"
	"backend/internal/utils"
)

func Apply(ctx context.Context, userID string, c types.Commit, lastPulledAt time.Time) (bool, *types.RestoredItem, error) {
	payload, ok := c.Payload.(map[string]any)
	if !ok {
		return false, nil, fmt.Errorf("user_to_group: payload must be an object")
	}

	id := utils.MapGetString(payload, "id")
	workspaceID := c.WorkspaceID
	uid := utils.MapGetString(payload, "user_id")
	groupID := utils.MapGetString(payload, "group_id")
	if id == "" || workspaceID == "" || uid == "" || groupID == "" {
		return false, nil, fmt.Errorf("user_to_group: missing id, workspace_id, user_id or group_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_group: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_group: invalid workspace_id %q: %w", workspaceID, err)
	}
	userUUID, err := db_types.NewJSONNullUUIDFromString(uid)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_group: invalid user_id %q: %w", uid, err)
	}
	groupUUID, err := db_types.NewJSONNullUUIDFromString(groupID)
	if err != nil {
		return false, nil, fmt.Errorf("user_to_group: invalid group_id %q: %w", groupID, err)
	}

	// Target group must belong to this workspace, else a groups.manage holder
	// could add a member to a group in another workspace (privilege escalation).
	if ok, err := scope.GroupInWorkspace(ctx, groupUUID, workspaceUUID); err != nil {
		return false, nil, fmt.Errorf("user_to_group: validate group: %w", err)
	} else if !ok {
		return false, nil, fmt.Errorf("user_to_group: group_id does not belong to workspace")
	}

	res, err := patch.Apply(ctx, c, patch.Handler[generated.AppUserToGroup, generated.UpsertUserToGroupParams]{
		TableName: "user_to_group",
		Fetch: func(ctx context.Context) (generated.AppUserToGroup, error) {
			return db.Queries.GetUserToGroupByID(ctx, generated.GetUserToGroupByIDParams{
				ID:          idUUID,
				WorkspaceID: workspaceUUID,
			})
		},
		UpdatedAt: func(row generated.AppUserToGroup) time.Time {
			return row.UpdatedAt.ValueOrZero()
		},
		DeletedAt: func(row generated.AppUserToGroup) *time.Time {
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				return &t
			}
			return nil
		},
		Restored: func(row generated.AppUserToGroup) (interface{}, error) {
			return types.ToRestoredPayload(appUserToGroupToTypesRow(row))
		},
		Merge: func(existing generated.AppUserToGroup, isNew bool, payload map[string]any) (generated.UpsertUserToGroupParams, error) {
			uid := userUUID
			gid := groupUUID
			wid := workspaceUUID
			if !isNew {
				uid = utils.PatchUUID(payload, "user_id", existing.UserID, userUUID)
				gid = utils.PatchUUID(payload, "group_id", existing.GroupID, groupUUID)
				wid = utils.PatchUUID(payload, "workspace_id", existing.WorkspaceID, workspaceUUID)
			}
			source := db_types.NewJSONNullString("local")
			if !isNew {
				source = existing.Source
			}
			if v := utils.MapGetString(payload, "source"); v != "" {
				source = db_types.NewJSONNullString(v)
			}
			return generated.UpsertUserToGroupParams{
				ID:          idUUID,
				UserID:      uid,
				GroupID:     gid,
				WorkspaceID: wid,
				Source:      source,
			}, nil
		},
		Upsert: func(ctx context.Context, params generated.UpsertUserToGroupParams) error {
			if err := db.Queries.UpsertUserToGroup(ctx, params); err != nil {
				return err
			}
			// Group membership changes the member's effective roles (baked into the
			// token), so revoke their refresh tokens to force a fresh one.
			_ = db.Queries.DeleteUserRefreshTokens(ctx, userUUID)
			return nil
		},
	})
	if res.Applied {
		audit.EmitChange(ctx, audit.GroupMemberAdded, c.WorkspaceID, groupID, res.Before, res.After)
	}
	return res.Applied, res.Restored, err
}
