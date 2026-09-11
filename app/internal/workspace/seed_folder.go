package workspace

import (
	"fmt"
	"os"

	"selectDb/internal/graph"
	"selectDb/internal/sample"
)

// seedSampleIfEmpty writes the sample workspace only into an empty folder: the
// folder is the user's, and seeding one they have work in would scatter files
// through it.
func (w *Workspace) seedSampleIfEmpty(workspaceID, folder string) error {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return fmt.Errorf("read workspace root: %w", err)
	}
	// The config we just wrote is ours, and so is anything else on that list.
	for _, entry := range entries {
		if !graph.IsInternalWorkspaceFile(entry.Name()) {
			return nil
		}
	}

	if err := sample.Write(workspaceID); err != nil {
		return fmt.Errorf("seed sample workspace: %w", err)
	}
	return nil
}
