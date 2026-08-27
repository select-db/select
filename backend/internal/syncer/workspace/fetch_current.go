package workspace

import (
	"context"
	"database/sql"
	"errors"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/syncer/types"
)

func FetchCurrent(ctx context.Context, c types.Commit) (*types.RestoredItem, error) {
	id := c.WorkspaceID
	if id == "" {
		id = c.ObjectID
	}
	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return nil, nil
	}
	row, err := db.Queries.GetWorkspaceByID(ctx, idUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r := types.WorkspaceRow{
		ID:           row.ID.String(),
		Name:         row.Name.ValueOrEmpty(),
		GitRemoteURL: row.GitRemoteUrl.Ptr(),
		Logo:         row.Logo.Ptr(),
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
	payload, err := types.ToRestoredPayload(r)
	if err != nil {
		return nil, err
	}
	return &types.RestoredItem{
		ObjectID:      id,
		TableName:     "workspace",
		ServerPayload: payload,
		UpdatedAt:     row.UpdatedAt.ValueOrZero(),
	}, nil
}
