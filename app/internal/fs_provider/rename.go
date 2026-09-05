package fs_provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RenameParams struct {
	OldURI string `json:"old_uri"`
	NewURI string `json:"new_uri"`
}

// within reports whether path sits under root. Comparing the paths as written
// is enough here: both have already been resolved, and symlinks were checked
// against the provider root on the way.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// workspaceRootURI returns the "selectdb://workspaces/<id>" a URI belongs to,
// and false for a URI in another namespace (the per-user config files).
func workspaceRootURI(uri string) (string, bool) {
	rel := strings.TrimPrefix(uri, "selectdb://")
	parts := strings.SplitN(rel, "/", 3)
	if len(parts) < 2 || parts[0] != "workspaces" || parts[1] == "" {
		return "", false
	}
	return "selectdb://" + parts[0] + "/" + parts[1], true
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

	// A rename names a file, or moves it within the workspace it belongs to. It
	// never takes it out: the provider's own guard only keeps a path under the
	// server folder, which holds every workspace, so "../../.." in a typed name
	// would land the file in a sibling workspace and out of sight. The check is
	// on the resolved path because that is where ".." has been applied.
	if wsRootURI, ok := workspaceRootURI(params.OldURI); ok {
		wsRoot, err := fsp.GetOSPathFromURI(wsRootURI)
		if err != nil {
			return err
		}
		if !within(wsRoot, newPath) {
			return fmt.Errorf("rename would leave the workspace: %s", params.NewURI)
		}
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
