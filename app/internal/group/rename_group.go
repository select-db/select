package group

import (
	"context"
	"fmt"
	"strings"

	"selectDb/internal/db/generated"
)

func (g *Group) RenameGroup(id string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	if err := g.Queries.UpdateGroupName(context.Background(), generated.UpdateGroupNameParams{
		ID:   id,
		Name: name,
	}); err != nil {
		return fmt.Errorf("rename group: %w", err)
	}

	return nil
}
