package workspace

import (
	"context"
	"fmt"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/syncer/patch"
	"backend/internal/syncer/types"
	"backend/internal/utils"
)

func Apply(ctx context.Context, userID string, c types.Commit, lastPulledAt time.Time) (bool, *types.RestoredItem, error) {
	id := c.WorkspaceID
	if id == "" {
		return false, nil, fmt.Errorf("workspace: missing id")
	}
	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("workspace: invalid id %q: %w", id, err)
	}

	var oldOwnerID db_types.JSONNullUUID
	return patch.Apply(ctx, c, patch.Handler[generated.GetWorkspaceByIDRow, generated.UpsertWorkspaceParams]{
		TableName: "workspace",
		Fetch: func(ctx context.Context) (generated.GetWorkspaceByIDRow, error) {
			return db.Queries.GetWorkspaceByID(ctx, idUUID)
		},
		UpdatedAt: func(row generated.GetWorkspaceByIDRow) time.Time {
			return row.UpdatedAt.ValueOrZero()
		},
		DeletedAt: func(row generated.GetWorkspaceByIDRow) *time.Time {
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				return &t
			}
			return nil
		},
		Restored: func(row generated.GetWorkspaceByIDRow) (interface{}, error) {
			r := types.WorkspaceRow{
				ID:           row.ID.String(),
				Name:         row.Name.ValueOrEmpty(),
				GitRemoteURL: row.GitRemoteUrl.Ptr(),
				UpdatedAt:    row.UpdatedAt.ValueOrZero(),
			}
			if row.OwnerID.Valid {
				s := row.OwnerID.String()
				r.OwnerID = &s
			}
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				r.DeletedAt = &t
			}
			return types.ToRestoredPayload(r)
		},
		Merge: func(existing generated.GetWorkspaceByIDRow, isNew bool, payload map[string]any) (generated.UpsertWorkspaceParams, error) {
			oldOwnerID = existing.OwnerID
			name := utils.PatchStr(payload, "name", existing.Name)
			if isNew && !name.Valid {
				name = db_types.NewJSONNullString("My workspace")
			}
			// owner_id is immutable through sync: a generic LWW upsert must not
			// transfer ownership. PatchUUID here would also resolve a present
			// payload key to an empty UUID, silently nulling the owner.
			ownerID := existing.OwnerID
			if isNew {
				if oid := utils.MapGetString(payload, "owner_id"); oid != "" {
					if parsed, err := db_types.NewJSONNullUUIDFromString(oid); err == nil {
						ownerID = parsed
					}
				}
			}
			return generated.UpsertWorkspaceParams{
				ID:           idUUID,
				Name:         name,
				GitRemoteUrl: utils.PatchNullStr(payload, "git_remote_url", existing.GitRemoteUrl),
				OwnerID:      ownerID,
			}, nil
		},
		Upsert: func(ctx context.Context, params generated.UpsertWorkspaceParams) error {
			if err := db.Queries.UpsertWorkspace(ctx, params); err != nil {
				return err
			}
			// Revoke old owner's tokens so their JWT gets fresh OwnedWorkspaceIDs
			if oldOwnerID.Valid && (!params.OwnerID.Valid || oldOwnerID.String() != params.OwnerID.String()) {
				_ = db.Queries.DeleteUserRefreshTokens(ctx, oldOwnerID)
			}
			return nil
		},
	})
}
