package workspace_to_user

import (
	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/syncer/types"
	"context"
	"time"
)

// GetChangesSince returns all workspace_to_user rows for the user updated after since.
func GetChangesSince(ctx context.Context, userID string, since time.Time) ([]types.WorkspaceToUserRow, error) {
	userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
	if err != nil {
		return nil, err
	}

	rows, err := db.Queries.GetWorkspaceToUserForUserSince(ctx, generated.GetWorkspaceToUserForUserSinceParams{
		UserID:    userUUID,
		UpdatedAt: db_types.NewJSONNullTimeFromTime(since),
	})
	if err != nil {
		return nil, err
	}

	out := make([]types.WorkspaceToUserRow, 0, len(rows))
	for _, wtu := range rows {
		out = append(out, appWorkspaceToUserToTypesRow(wtu))
	}

	return out, nil
}

func appWorkspaceToUserToTypesRow(row generated.AppWorkspaceToUser) types.WorkspaceToUserRow {
	r := types.WorkspaceToUserRow{
		ID:          row.ID.String(),
		WorkspaceID: row.WorkspaceID.String(),
		UserID:      row.UserID.String(),
		UpdatedAt:   row.UpdatedAt.ValueOrZero(),
	}
	if row.DeletedAt.Valid {
		t := row.DeletedAt.ValueOrZero()
		r.DeletedAt = &t
	}
	return r
}
