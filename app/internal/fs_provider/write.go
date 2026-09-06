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

// Write replaces the file at the URI, which must be inside a folder that
// already exists. Making a file is a Mkdir and then a Write, in that order.
//
// The folder is deliberately not made here. Writes are debounced — a form
// saves 600ms after the last keystroke, an editor 200ms — so one can land
// after what it was editing has been deleted, and a write that makes its own
// folder puts the deleted thing back: a database returned to the tree seconds
// after being removed, a schema dump rebuilt the directory it belonged to. A
// write with nowhere to go is a lost edit, and says so.
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
