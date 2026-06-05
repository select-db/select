package role

import (
	"context"
	"fmt"
	"strings"

	"selectDb/backend/db/generated"
)

func (r *Role) RenameRole(id string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("role name cannot be empty")
	}

	if err := r.Queries.UpdateRoleName(context.Background(), generated.UpdateRoleNameParams{
		ID:   id,
		Name: name,
	}); err != nil {
		return fmt.Errorf("rename role: %w", err)
	}

	return nil
}
