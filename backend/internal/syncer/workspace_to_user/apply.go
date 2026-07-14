package workspace_to_user

import (
	"context"
	"fmt"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/syncer/patch"
	"backend/internal/syncer/types"
	"backend/internal/utils"
)

func Apply(ctx context.Context, userID string, c types.Commit, lastPulledAt time.Time) (bool, *types.RestoredItem, error) {
	payload, ok := c.Payload.(map[string]any)
	if !ok {
		return false, nil, fmt.Errorf("workspace_to_user: payload must be an object")
	}

	id := utils.MapGetString(payload, "id")
	workspaceID := c.WorkspaceID
	uid := utils.MapGetString(payload, "user_id")
	if id == "" || workspaceID == "" || uid == "" {
		return false, nil, fmt.Errorf("workspace_to_user: missing id, workspace_id or user_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: invalid workspace_id %q: %w", workspaceID, err)
	}
	uidUUID, err := db_types.NewJSONNullUUIDFromString(uid)
	if err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: invalid user_id %q: %w", uid, err)
	}

	// Ensure parent workspace exists before inserting the FK reference.
	if err := db.Queries.InsertWorkspaceIfNotExists(ctx, workspaceUUID); err != nil {
		return false, nil, fmt.Errorf("workspace_to_user: insert workspace if not exists: %w", err)
	}

	res, err := patch.Apply(ctx, c, patch.Handler[generated.AppWorkspaceToUser, generated.UpsertWorkspaceToUserParams]{
		TableName: "workspace_to_user",
		Fetch: func(ctx context.Context) (generated.AppWorkspaceToUser, error) {
			return db.Queries.GetWorkspaceToUserByID(ctx, generated.GetWorkspaceToUserByIDParams{
				ID:          idUUID,
				WorkspaceID: workspaceUUID,
			})
		},
		UpdatedAt: func(row generated.AppWorkspaceToUser) time.Time {
			return row.UpdatedAt.ValueOrZero()
		},
		DeletedAt: func(row generated.AppWorkspaceToUser) *time.Time {
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				return &t
			}
			return nil
		},
		Restored: func(row generated.AppWorkspaceToUser) (interface{}, error) {
			return types.ToRestoredPayload(appWorkspaceToUserToTypesRow(row))
		},
		Merge: func(existing generated.AppWorkspaceToUser, isNew bool, payload map[string]any) (generated.UpsertWorkspaceToUserParams, error) {
			wid := workspaceUUID
			uid := uidUUID
			if !isNew {
				wid = utils.PatchUUID(payload, "workspace_id", existing.WorkspaceID, workspaceUUID)
				uid = utils.PatchUUID(payload, "user_id", existing.UserID, uidUUID)
			}
			return generated.UpsertWorkspaceToUserParams{
				ID:          idUUID,
				WorkspaceID: wid,
				UserID:      uid,
			}, nil
		},
		Upsert: func(ctx context.Context, params generated.UpsertWorkspaceToUserParams) error {
			return db.Queries.UpsertWorkspaceToUser(ctx, params)
		},
	})
	if res.Applied {
		audit.EmitChange(ctx, audit.MemberAdded, c.WorkspaceID, uid, res.Before, res.After)
	}
	return res.Applied, res.Restored, err
}
