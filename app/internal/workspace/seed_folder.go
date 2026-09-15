package workspace

import (
	"fmt"
	"os"

	"selectDb/internal/graph"
	"selectDb/internal/sample"
)

// seedSampleIfEmpty writes the sample only into an otherwise empty folder, since
// seeding one the user has work in scatters files through it. The workspace must
// already be open: sample.Write resolves the root from it.
func (w *Workspace) seedSampleIfEmpty(workspaceID, folder string) error {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return fmt.Errorf("read workspace root: %w", err)
	}
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
