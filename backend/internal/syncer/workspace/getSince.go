package workspace

import (
	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/syncer/types"
	"context"
	"time"
)

// GetChangesSince returns all workspaces the user is a member of that were updated after since.
func GetChangesSince(ctx context.Context, userID string, since time.Time) ([]types.WorkspaceRow, error) {
	userUUID, err := db_types.NewJSONNullUUIDFromString(userID)
	if err != nil {
		return nil, err
	}

	rows, err := db.Queries.GetWorkspacesForUserSince(ctx, generated.GetWorkspacesForUserSinceParams{
		UserID:    userUUID,
		UpdatedAt: db_types.NewJSONNullTimeFromTime(since),
	})
	if err != nil {
		return nil, err
	}

	out := make([]types.WorkspaceRow, 0, len(rows))
	for _, w := range rows {
		out = append(out, appWorkspaceToTypesRow(w))
	}

	return out, nil
}

func appWorkspaceToTypesRow(row generated.GetWorkspacesForUserSinceRow) types.WorkspaceRow {
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
	return r
}
