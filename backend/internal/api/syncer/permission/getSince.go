package permission

import (
	"context"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/api/syncer/types"
)

func GetChangesSince(ctx context.Context, userID string, since time.Time) ([]types.PermissionRow, error) {
	userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
	if err != nil {
		return nil, err
	}

	rows, err := db.Queries.GetPermissionsForUserSince(ctx, generated.GetPermissionsForUserSinceParams{
		UserID:    userUUID,
		UpdatedAt: db_types.NewJSONNullTimeFromTime(since),
	})
	if err != nil {
		return nil, err
	}

	out := make([]types.PermissionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, appPermissionToTypesRow(generated.AppPermission(r)))
	}
	return out, nil
}

func appPermissionToTypesRow(row generated.AppPermission) types.PermissionRow {
	r := types.PermissionRow{
		ID:          row.ID.String(),
		RoleID:      row.RoleID.String(),
		WorkspaceID: row.WorkspaceID.String(),
		Action:      row.Action.ValueOrEmpty(),
		Effect:      row.Effect.ValueOrEmpty(),
		UpdatedAt:   row.UpdatedAt.ValueOrZero(),
	}
	r.DbInstanceID = row.DbInstanceID.Ptr()
	r.SchemaName = row.SchemaName.Ptr()
	r.TableName = row.TableName.Ptr()
	r.ColumnName = row.ColumnName.Ptr()
	if row.DeletedAt.Valid {
		t := row.DeletedAt.ValueOrZero()
		r.DeletedAt = &t
	}
	return r
}
