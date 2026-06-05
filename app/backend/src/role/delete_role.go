package role

import (
	"context"
	"fmt"
)

// DeleteRole deletes a role by ID. Cascades to user_role and permission.
func (r *Role) DeleteRole(roleID string) error {
	if err := r.Queries.DeleteRole(context.Background(), roleID); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}
