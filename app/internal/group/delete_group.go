package group

import (
	"context"
	"fmt"
)

// DeleteGroup deletes a group by ID. Cascades to user_to_group and group_to_role.
func (g *Group) DeleteGroup(groupID string) error {
	if err := g.Queries.DeleteGroup(context.Background(), groupID); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}
