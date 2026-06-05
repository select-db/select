package permission

import (
	"context"
	"database/sql"
	"errors"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/syncer/types"
)

func FetchCurrent(ctx context.Context, c types.Commit) (*types.RestoredItem, error) {
	idUUID, err := db_types.NewJSONNullUUIDFromString(c.ObjectID)
	if err != nil {
		return nil, nil
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(c.WorkspaceID)
	if err != nil {
		return nil, nil
	}
	row, err := db.Queries.GetPermissionByID(ctx, generated.GetPermissionByIDParams{
		ID:          idUUID,
		WorkspaceID: workspaceUUID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	payload, err := types.ToRestoredPayload(appPermissionToTypesRow(row))
	if err != nil {
		return nil, err
	}
	return &types.RestoredItem{
		ObjectID:      c.ObjectID,
		TableName:     "permission",
		ServerPayload: payload,
		UpdatedAt:     row.UpdatedAt.ValueOrZero(),
	}, nil
}
