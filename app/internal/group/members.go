package group

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

type GroupUserEntry struct {
	ID            string  `json:"id"`
	Name          *string `json:"name"`
	UserToGroupID string  `json:"user_to_group_id"`
}

func (g *Group) ListGroupMembers(groupID string) ([]GroupUserEntry, error) {
	rows, err := g.Queries.ListUsersByGroup(context.Background(), groupID)
	if err != nil {
		return nil, fmt.Errorf("list users by group: %w", err)
	}
	out := make([]GroupUserEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, GroupUserEntry{
			ID:            row.ID,
			Name:          row.Name.Ptr(),
			UserToGroupID: row.UserToGroupID,
		})
	}
	return out, nil
}

func (g *Group) AddUserToGroup(userID, groupID string) error {
	ctx := context.Background()
	wtu, err := g.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no active workspace")
		}
		return fmt.Errorf("get current workspace: %w", err)
	}
	_, err = g.Queries.InsertUserToGroup(ctx, generated.InsertUserToGroupParams{
		ID:          utils.GenerateUUID(),
		UserID:      userID,
		GroupID:     groupID,
		WorkspaceID: wtu.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("add user to group: %w", err)
	}
	return nil
}

func (g *Group) RemoveUserFromGroup(userToGroupID string) error {
	if err := g.Queries.RemoveUserFromGroup(context.Background(), userToGroupID); err != nil {
		return fmt.Errorf("remove user from group: %w", err)
	}
	return nil
}
