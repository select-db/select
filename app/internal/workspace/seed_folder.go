package workspace

import (
	"fmt"
	"os"

	"selectDb/internal/graph"
	"selectDb/internal/sample"
)

// seedIfEmpty writes the sample workspace, gated on the folder being empty
// rather than new: the folder is the user's, and seeding one they have work in
// would scatter files through it.
func (w *Workspace) seedIfEmpty(workspaceID string) error {
	root, err := graph.WorkspaceRootPath(workspaceID)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read workspace root: %w", err)
	}
	// The config is ours and is written on the way in, so it does not count.
	for _, entry := range entries {
		if entry.Name() != graph.WorkspaceConfigFileName {
			return nil
		}
	}

	if err := sample.Write(workspaceID); err != nil {
		return fmt.Errorf("seed sample workspace: %w", err)
	}
	return nil
}
