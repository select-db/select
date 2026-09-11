package workspace

import (
	"context"
	"fmt"

	"selectDb/internal/api"
	"selectDb/internal/graph"
)

// DeleteWorkspace removes the workspace on the server, its local rows, and the
// select.config.json that named it. The folder is left as it was.
//
// A config can outlive its workspace when the delete happened on another
// machine; the init screen cleans that up on the next open.
func (w *Workspace) DeleteWorkspace(workspaceID string) error {
	ctx := context.Background()

	// The open workspace is the one whose folder is known for certain; a
	// local_path is a hint, and a stale one would take another workspace's config.
	folder := ""
	if id, root, ok := graph.OpenWorkspace(); ok && id == workspaceID {
		folder = root
	}

	if err := api.Fetch(ctx, "DELETE", "workspaces/"+workspaceID, nil, api.WorkspaceHeader(workspaceID), nil); err != nil {
		return fmt.Errorf("delete workspace on server: %w", err)
	}

	if err := w.Queries.DeleteWorkspaceToUserByWorkspaceID(ctx, workspaceID); err != nil {
		return fmt.Errorf("delete workspace_to_user: %w", err)
	}

	if err := w.Queries.DeleteWorkspaceByID(ctx, workspaceID); err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	if folder != "" {
		if err := graph.RemoveWorkspaceConfig(folder); err != nil {
			return err
		}
	}

	if folder != "" {
		return w.CloseFolder()
	}
	return nil
}
