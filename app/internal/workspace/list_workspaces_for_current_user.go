package workspace

import (
	"context"
	"database/sql"
	"fmt"
)

// WorkspaceWithCurrent is a workspace entry with a current flag for the UI.
// Logo is the base64 of a 128x128 PNG, empty when the workspace has none; the
// UI composes the data URL around it so a stored value can never bring its own
// media type.
type WorkspaceWithCurrent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Logo    string `json:"logo"`
	Current bool   `json:"current"`
}

// ListWorkspacesForCurrentUser returns all workspaces for the current user (from workspace_to_user).
func (w *Workspace) ListWorkspacesForCurrentUser() ([]WorkspaceWithCurrent, error) {
	ctx := context.Background()
	user, err := w.Queries.GetCurrentUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get current user: %w", err)
	}
	rows, err := w.Queries.ListWorkspacesByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	out := make([]WorkspaceWithCurrent, 0, len(rows))
	for _, r := range rows {
		out = append(out, WorkspaceWithCurrent{
			ID:      r.ID,
			Name:    r.Name,
			Logo:    r.Logo.Or(""),
			Current: r.Current.Valid && r.Current.Bool,
		})
	}
	return out, nil
}
