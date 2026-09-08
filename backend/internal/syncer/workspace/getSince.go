package workspace

import (
	"backend/db"
	"backend/db/generated"

	"backend/internal/syncer/types"
	"context"
	"github.com/google/uuid"
	"time"
)

// GetChangesSince returns all workspaces the user is a member of that were updated after since.
func GetChangesSince(ctx context.Context, userID string, since time.Time) ([]types.WorkspaceRow, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	rows, err := db.Queries.GetWorkspacesForUserSince(ctx, generated.GetWorkspacesForUserSinceParams{
		UserID:    userUUID,
		UpdatedAt: since,
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
		Name:         row.Name,
		GitRemoteURL: row.GitRemoteUrl.Ptr(),
		Logo:         row.Logo.Ptr(),
		UpdatedAt:    row.UpdatedAt,
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
