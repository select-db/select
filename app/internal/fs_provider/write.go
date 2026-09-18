package fs_provider

import (
	"fmt"
	"os"
	"path/filepath"
)

type WriteParams struct {
	URI     string `json:"uri"`
	Content string `json:"content"`
}

// Write replaces the file at the URI. Its folder must exist: writes are
// debounced, so one can land after what it was editing was deleted, and a write
// that makes its own folder puts the deleted thing back. Making a file is a
// Mkdir and then a Write.
func (fsp *FSProvider) Write(params WriteParams) error {
	path, err := fsp.GetOSPathFromURI(params.URI)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("write %s: %w", params.URI, err)
	}

	return os.WriteFile(path, []byte(params.Content), 0o600)
}
