package fs_provider

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"selectDb/internal/fs_uri"
	"selectDb/internal/utils"
)

// GetOSPathFromURI resolves a logical URI to an absolute OS path.
//
// Two URI namespaces are supported, both under the "selectdb://" scheme:
//
//   - selectdb://workspaces/<id>/...  → joined under the provider root (the
//     current server folder), so URI and filesystem paths share the same shape.
//   - selectdb://user/...             → joined under the per-user config dir,
//     for personal files (.theme, .config) that live outside every workspace.
func (fsp *FSProvider) GetOSPathFromURI(URI string) (string, error) {
	rel, ok := fs_uri.Rel(URI)
	if !ok {
		return "", fmt.Errorf("invalid URI (expected scheme %q and a path): %s", fs_uri.Scheme, URI)
	}

	if strings.HasPrefix(rel, fs_uri.UserPrefix) {
		return userOSPathFromURI(URI, rel)
	}

	if !strings.HasPrefix(rel, fs_uri.WorkspacePrefix) {
		return "", fmt.Errorf("invalid URI path (expected to start with %q or %q): %s", fs_uri.WorkspacePrefix, fs_uri.UserPrefix, URI)
	}

	if fsp.root == "" {
		return "", fmt.Errorf("FSProvider root not set (no server selected)")
	}

	full, err := fs_uri.Resolve(fsp.root, rel)
	if err != nil {
		return "", fmt.Errorf("invalid URI path (cannot be absolute or escape root): %s", URI)
	}

	// This path is about to be read or written, so the lexical check is not
	// enough: a symlink inside the workspace must not lead out of it.
	if err := fs_uri.EnsureWithin(fsp.root, full); err != nil {
		return "", fmt.Errorf("invalid URI path (escapes workspace root): %s", URI)
	}
	return full, nil
}

// userOSPathFromURI resolves a selectdb://user/... URI to an absolute path under
// the per-user config directory. rel is the path portion after the scheme
// (e.g. "user/.theme"). It guards against path traversal so a crafted URI
// cannot escape the user config dir.
func userOSPathFromURI(URI, rel string) (string, error) {
	sub := strings.TrimPrefix(rel, fs_uri.UserPrefix)
	if sub == "" {
		return "", fmt.Errorf("missing path in uri: %s", URI)
	}

	dir, err := utils.UserConfigDir()
	if err != nil {
		return "", err
	}

	full, err := fs_uri.Resolve(dir, sub)
	if err != nil {
		return "", fmt.Errorf("invalid URI path (cannot be absolute or escape user config dir): %s", URI)
	}
	if err := fs_uri.EnsureWithin(dir, full); err != nil {
		return "", fmt.Errorf("invalid URI path (escapes user config dir): %s", URI)
	}
	return full, nil
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
