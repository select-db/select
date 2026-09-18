package graph

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// DefaultsFS embeds all files from the defaults folder, including dotfiles.
// The folder is split by ownership:
//
//   - defaults/workspace — shared, git-tracked files seeded into a workspace root
//     (.lint, .gitignore, db.config.json).
//   - defaults/user — personal files seeded into the per-user config dir
//     (.theme, .config keybindings/snippets).
//
//go:embed all:defaults
var DefaultsFS embed.FS

// SeedDefaultFiles copies the shared workspace defaults into workspaceRoot,
// preserving subdirectory structure and skipping files that already exist on disk.
func SeedDefaultFiles(workspaceRoot string) error {
	return seedFrom("defaults/workspace", workspaceRoot)
}

// SeedUserDefaultFiles copies the personal defaults (.theme, .config) into the
// per-user config directory, skipping files that already exist so the user's
// edits are never clobbered.
func SeedUserDefaultFiles(userConfigDir string) error {
	return seedFrom("defaults/user", userConfigDir)
}

// seedFrom copies every embedded file under srcRoot into destRoot, preserving
// the relative subdirectory structure and skipping files that already exist.
func seedFrom(srcRoot, destRoot string) error {
	return fs.WalkDir(DefaultsFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destRoot, rel)
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
