package group

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
		return false, nil, fmt.Errorf("group: payload must be an object")
	}

	id := utils.MapGetString(payload, "id")
	workspaceID := c.WorkspaceID
	if id == "" || workspaceID == "" {
		return false, nil, fmt.Errorf("group: missing id or workspace_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("group: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("group: invalid workspace_id %q: %w", workspaceID, err)
	}

	return patch.Apply(ctx, c, patch.Handler[generated.AppGroup, generated.UpsertGroupParams]{
		TableName: "group",
		Audit:     &patch.AuditConfig{Spec: audit.GroupUpserted, TargetID: id},
		Fetch: func(ctx context.Context) (generated.AppGroup, error) {
			return db.Queries.GetGroupByID(ctx, idUUID)
		},
		UpdatedAt: func(row generated.AppGroup) time.Time {
			return row.UpdatedAt.ValueOrZero()
		},
		DeletedAt: func(row generated.AppGroup) *time.Time {
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				return &t
			}
			return nil
		},
		Restored: func(row generated.AppGroup) (interface{}, error) {
			return types.ToRestoredPayload(appGroupToTypesRow(row))
		},
		Merge: func(existing generated.AppGroup, isNew bool, payload map[string]any) (generated.UpsertGroupParams, error) {
			// Fetch is by id only; reject hijacking a group that belongs to
			// another workspace (UpsertGroup would rewrite its workspace_id).
			if !isNew && existing.WorkspaceID != workspaceUUID {
				return generated.UpsertGroupParams{}, fmt.Errorf("group: belongs to another workspace")
			}
			// source is NOT NULL; default 'local' for locally-created groups so the
			// explicit upsert value never violates the constraint.
			source := db_types.NewJSONNullString("local")
			if !isNew {
				source = existing.Source
			}
			if v := utils.MapGetString(payload, "source"); v != "" {
				source = db_types.NewJSONNullString(v)
			}
			return generated.UpsertGroupParams{
				ID:          idUUID,
				WorkspaceID: workspaceUUID,
				Name:        utils.PatchStr(payload, "name", existing.Name),
				Source:      source,
				ExternalID:  utils.PatchNullStr(payload, "external_id", existing.ExternalID),
			}, nil
		},
		Upsert: func(ctx context.Context, params generated.UpsertGroupParams) error {
			return db.Queries.UpsertGroup(ctx, params)
		},
	})
}
