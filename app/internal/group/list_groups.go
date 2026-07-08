package group

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/internal/db/generated"
)

// ListGroups returns all groups for the current workspace, with member and role counts.
func (g *Group) ListGroups() ([]generated.ListGroupsByWorkspaceRow, error) {
	ctx := context.Background()

	wtu, err := g.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get current workspace: %w", err)
	}

	rows, err := g.Queries.ListGroupsByWorkspace(ctx, wtu.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	return rows, nil
}
