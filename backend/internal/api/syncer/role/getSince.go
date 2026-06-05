package role

import (
	"context"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/api/syncer/types"
)

func GetChangesSince(ctx context.Context, userID string, since time.Time) ([]types.RoleRow, error) {
	userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
	if err != nil {
		return nil, err
	}

	rows, err := db.Queries.GetRolesForUserSince(ctx, generated.GetRolesForUserSinceParams{
		UserID:    userUUID,
		UpdatedAt: db_types.NewJSONNullTimeFromTime(since),
	})
	if err != nil {
		return nil, err
	}

	out := make([]types.RoleRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, appRoleToTypesRow(r))
	}
	return out, nil
}

func appRoleToTypesRow(row generated.AppRole) types.RoleRow {
	r := types.RoleRow{
		ID:          row.ID.String(),
		WorkspaceID: row.WorkspaceID.String(),
		Name:        row.Name.ValueOrEmpty(),
		UpdatedAt:   row.UpdatedAt.ValueOrZero(),
	}
	if row.DeletedAt.Valid {
		t := row.DeletedAt.ValueOrZero()
		r.DeletedAt = &t
	}
	return r
}
