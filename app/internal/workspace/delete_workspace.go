package workspace

import (
	"context"
	"fmt"

	"selectDb/internal/api"
)

// DeleteWorkspace deletes the workspace on the server then removes it locally.
// If the deleted workspace was current and ReloadHooks is set, runs switch-or-logout.
// Returns (loggedOut, nil) when the user had no workspaces left and was logged out, (false, nil) otherwise, or (false, err) on error.
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

	if err := w.RemoveWorkspaceFolderByID(workspaceID); err != nil {
		return fmt.Errorf("remove workspace folder: %w", err)
	}

	return nil
}
