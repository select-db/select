package group

import (
	"context"
	"database/sql"
	"errors"

	"backend/db"
	"backend/db/db_types"
	"backend/internal/syncer/types"
)

func FetchCurrent(ctx context.Context, c types.Commit) (*types.RestoredItem, error) {
	idUUID, err := db_types.NewJSONNullUUIDFromString(c.ObjectID)
	if err != nil {
		return nil, nil
	}
	row, err := db.Queries.GetGroupByID(ctx, idUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	payload, err := types.ToRestoredPayload(appGroupToTypesRow(row))
	if err != nil {
		return nil, err
	}
	return &types.RestoredItem{
		ObjectID:      c.ObjectID,
		TableName:     "group",
		ServerPayload: payload,
		UpdatedAt:     row.UpdatedAt.ValueOrZero(),
	}, nil
}
