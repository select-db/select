package workspace

import (
	"context"
	"fmt"

	"selectDb/internal/api"
)

// DeleteWorkspace deletes the workspace on the server then removes its local
// rows. The folder on disk is left alone: it is the user's own directory, and
// what makes it a workspace is a config file, not the directory itself.
// If the deleted workspace was current and ReloadHooks is set, runs switch-or-logout.
func (w *Workspace) DeleteWorkspace(workspaceID string) error {
	ctx := context.Background()

	if err := api.Fetch(ctx, "DELETE", "workspaces/"+workspaceID, nil, api.WorkspaceHeader(workspaceID), nil); err != nil {
		return fmt.Errorf("delete workspace on server: %w", err)
	}

	if err := w.Queries.DeleteWorkspaceToUserByWorkspaceID(ctx, workspaceID); err != nil {
		return fmt.Errorf("delete workspace_to_user: %w", err)
	}

	if err := w.Queries.DeleteWorkspaceByID(ctx, workspaceID); err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	return nil
}
