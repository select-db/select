package graph

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// DefaultsFS embeds all files from the defaults folder, including dotfiles.
//
//go:embed all:defaults
var DefaultsFS embed.FS

// SeedDefaultFiles copies every file from the embedded defaults directory into
// workspaceRoot, preserving subdirectory structure and skipping files that already exist on disk.
func SeedDefaultFiles(workspaceRoot string) error {
	return fs.WalkDir(DefaultsFS, "defaults", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel("defaults", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(workspaceRoot, rel)
		if _, err := os.Stat(dest); err == nil {
			return nil // already exists, skip
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := DefaultsFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}
