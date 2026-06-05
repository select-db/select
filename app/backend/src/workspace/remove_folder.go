package workspace

import (
	"fmt"
	"os"

	"selectDb/backend/graph"
)

// RemoveWorkspaceFolderByID deletes the workspace root directory on disk.
// Safe to call if the folder does not exist (no error).
func (w *Workspace) RemoveWorkspaceFolderByID(workspaceID string) error {
	if workspaceID == "" {
		return nil
	}
	root, err := graph.WorkspaceRootPath(workspaceID)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}
	if err := os.RemoveAll(root); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove workspace folder: %w", err)
	}
	return nil
}
