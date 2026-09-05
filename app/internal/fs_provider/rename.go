package fs_provider

import (
	"fmt"
	"os"
	"strings"
)

type RenameParams struct {
	OldURI string `json:"old_uri"`
	NewURI string `json:"new_uri"`
}

func (fsp *FSProvider) Rename(params RenameParams) error {
	oldPath, err := fsp.GetOSPathFromURI(params.OldURI)
	if err != nil {
		return err
	}
	newPath, err := fsp.GetOSPathFromURI(params.NewURI)
	if err != nil {
		return err
	}

	// os.Rename replaces the target without a word, so a rename onto a name
	// already in the folder -- typed by hand, or produced by dragging a file
	// where its twin lives -- would destroy the other file.
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("%s already exists", params.NewURI)
	}

	// Check if oldPath exists
	if err := createFileIfNotExists(oldPath); err != nil {
		return fmt.Errorf("ensure file at old path: %w", err)
	}

	// Rename the main file
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}

	// Handle metadata sidecar
	oldMetaPath := oldPath + ".metadata.json"
	newMetaPath := newPath + ".metadata.json"

	if strings.HasSuffix(newPath, ".sql") {
		// Ensure metadata exists at old location, then rename it
		_ = createFileIfNotExists(oldMetaPath)
		_ = os.Rename(oldMetaPath, newMetaPath)
	} else {
		// Remove metadata sidecar (no-op if it doesn't exist)
		_ = os.Remove(oldMetaPath)
	}

	return nil
}
