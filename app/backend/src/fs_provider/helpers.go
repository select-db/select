package fs_provider

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// GetOSPathFromURI resolves a logical workspace URI to an absolute OS path.
// It enforces that URIs use the "selectdb://" scheme and that the path portion
// starts with "workspaces/". The path portion after the scheme is joined
// directly under the provider root, ensuring that URI and filesystem paths
// have the exact same shape.
func (fsp *FSProvider) GetOSPathFromURI(URI string) (string, error) {
	const scheme = "selectdb://"

	if !strings.HasPrefix(URI, scheme) {
		return "", fmt.Errorf("invalid URI (expected scheme %q): %s", scheme, URI)
	}

	rel := strings.TrimPrefix(URI, scheme)
	if rel == "" {
		return "", fmt.Errorf("missing path in uri: %s", URI)
	}

	if !strings.HasPrefix(rel, "workspaces/") {
		return "", fmt.Errorf("invalid URI path (expected to start with %q): %s", "workspaces/", URI)
	}

	// Normalise and guard against path traversal or absolute paths.
	rel = filepath.Clean(rel)
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid URI path (cannot be absolute or escape root): %s", URI)
	}
	if fsp.root == "" {
		return "", fmt.Errorf("FSProvider root not set (no server selected)")
	}
	full := filepath.Join(fsp.root, rel)
	if err := ensureWithinRoot(fsp.root, full); err != nil {
		return "", fmt.Errorf("invalid URI path (escapes workspace root): %s", URI)
	}
	return full, nil
}

// ensureWithinRoot resolves symlinks on the deepest existing ancestor of full
// and verifies it stays under root, so a symlink placed inside the workspace
// (e.g. by a cloned repo) cannot be followed out to the host filesystem.
func ensureWithinRoot(root, full string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	p := full
	for {
		if resolved, rerr := filepath.EvalSymlinks(p); rerr == nil {
			rel, relErr := filepath.Rel(realRoot, resolved)
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("escapes root")
			}
			return nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return fmt.Errorf("escapes root")
		}
		p = parent
	}
}

func fileInfoToStat(info fs.FileInfo) FileStat {
	var ft FileType
	switch {
	case info.IsDir():
		ft = FileTypeDirectory
	default:
		ft = FileTypeFile
	}

	return FileStat{
		Type:  ft,
		Size:  info.Size(),
		Mtime: info.ModTime(),
	}
}

func dirEntryType(entry fs.DirEntry) FileType {
	if entry.Type().IsDir() {
		return FileTypeDirectory
	}
	if entry.Type().IsRegular() {
		return FileTypeFile
	}
	return FileTypeUnknown
}

// createFileIfNotExists creates an empty file at the given path if it doesn't exist.
// If the file already exists, it does nothing and returns nil.
// The parent directory is created if it doesn't exist.
func createFileIfNotExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		// File already exists, nothing to do
		return nil
	}
	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ensure parent directory: %w", err)
	}
	// File doesn't exist, create it
	return os.WriteFile(path, []byte{}, 0o600)
}
