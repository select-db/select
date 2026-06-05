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

func (fsp *FSProvider) Write(params WriteParams) error {
	path, err := fsp.GetOSPathFromURI(params.URI)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ensure parent directory: %w", err)
	}

	return os.WriteFile(path, []byte(params.Content), 0o600)
}
