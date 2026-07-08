package group

import (
	"context"
	"fmt"
)

// DeleteGroup deletes a group by ID. The delete syncs to the backend, which
// stops honoring the group's role grants; the membership and role-attachment
// rows are not explicitly removed (SQLite FKs are not enforced locally, and
// backend effective-role resolution ignores deleted groups).
func (g *Group) DeleteGroup(groupID string) error {
	if err := g.Queries.DeleteGroup(context.Background(), groupID); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}
