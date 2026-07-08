package group

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

func (g *Group) CreateGroup(name string) (generated.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return generated.Group{}, fmt.Errorf("group name cannot be empty")
	}

	ctx := context.Background()

	wtu, err := g.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return generated.Group{}, fmt.Errorf("no active workspace")
		}
		return generated.Group{}, fmt.Errorf("get current workspace: %w", err)
	}

	row, err := g.Queries.InsertGroup(ctx, generated.InsertGroupParams{
		ID:          utils.GenerateUUID(),
		WorkspaceID: wtu.WorkspaceID,
		Name:        name,
	})
	if err != nil {
		return generated.Group{}, fmt.Errorf("create group: %w", err)
	}

	return row, nil
}
