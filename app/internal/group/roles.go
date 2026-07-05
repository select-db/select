package group

import (
	"context"
	"database/sql"
	"fmt"

	"selectDb/internal/db/generated"
	"selectDb/internal/utils"
)

type GroupRoleEntry struct {
	ID            string  `json:"id"`
	Name          *string `json:"name"`
	GroupToRoleID string  `json:"group_to_role_id"`
}

func (g *Group) ListGroupRoles(groupID string) ([]GroupRoleEntry, error) {
	rows, err := g.Queries.ListRolesByGroup(context.Background(), groupID)
	if err != nil {
		return nil, fmt.Errorf("list roles by group: %w", err)
	}
	out := make([]GroupRoleEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, GroupRoleEntry{
			ID:            row.ID,
			Name:          row.Name.Ptr(),
			GroupToRoleID: row.GroupToRoleID,
		})
	}
	return out, nil
}

func (g *Group) AttachRoleToGroup(roleID, groupID string) error {
	ctx := context.Background()
	wtu, err := g.Queries.GetCurrentWorkspaceToUser(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no active workspace")
		}
		return fmt.Errorf("get current workspace: %w", err)
	}
	_, err = g.Queries.InsertGroupToRole(ctx, generated.InsertGroupToRoleParams{
		ID:          utils.GenerateUUID(),
		GroupID:     groupID,
		RoleID:      roleID,
		WorkspaceID: wtu.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("attach role to group: %w", err)
	}
	return nil
}

func (g *Group) DetachRoleFromGroup(groupToRoleID string) error {
	if err := g.Queries.RemoveGroupToRole(context.Background(), groupToRoleID); err != nil {
		return fmt.Errorf("detach role from group: %w", err)
	}
	return nil
}
