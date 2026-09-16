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
	if !onlySelectsOwn(entries) {
		return nil
	}

	if err := sample.Write(workspaceID); err != nil {
		return fmt.Errorf("seed sample workspace: %w", err)
	}
	return nil
}

// onlySelectsOwn reports whether a folder holds nothing but what SELECT put
// there itself.
//
// The workspace config counts as ours even though the tree shows it as a file:
// opening the folder wrote it a moment before this runs, so a folder holding
// only that one is still an empty folder to seed.
func onlySelectsOwn(entries []os.DirEntry) bool {
	for _, entry := range entries {
		name := entry.Name()
		if name == graph.WorkspaceConfigFileName || graph.IsInternalWorkspaceFile(name) {
			continue
		}
		return false
	}
	return true
}
