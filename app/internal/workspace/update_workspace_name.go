package workspace

import (
	"context"
	"selectDb/internal/db/generated"
)

func (w *Workspace) UpdateName(workspaceID, name string) error {
	return w.Queries.UpdateWorkspaceName(context.Background(), generated.UpdateWorkspaceNameParams{
		ID:   workspaceID,
		Name: name,
	})
}
