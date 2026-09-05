// Package fs_uri owns the mapping between selectdb:// URIs and paths on disk.
//
// A URI and the path it names have the same shape once the scheme and the root
// are taken off:
//
//	selectdb://workspaces/<id>/queries/a.sql  ↔  <root>/workspaces/<id>/queries/a.sql
//
// Two packages resolve URIs -- fs_provider, to do the syscall a URI asks for,
// and graph, to find the node a URI names -- and they used to parse and guard
// them separately. They go through here instead, so the scheme, the traversal
// guard and the containment rule cannot drift apart.
package fs_uri

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// Scheme prefixes every URI the app addresses files by.
	Scheme = "selectdb://"

	// WorkspacePrefix namespaces files inside a workspace, UserPrefix the
	// personal files (.theme, .config) that live outside every workspace.
	WorkspacePrefix = "workspaces/"
	UserPrefix      = "user/"
)

// Rel returns what follows the scheme, and false for anything that is not a
// URI or carries no path.
func Rel(uri string) (string, bool) {
	rel, ok := strings.CutPrefix(uri, Scheme)
	if !ok || rel == "" {
		return "", false
	}
	return rel, true
}

// Resolve joins a relative path onto root, refusing one that would leave it —
// absolute, or climbing out through "..". Slash-separated input is accepted, as
// URIs carry it on every platform.
func Resolve(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("no root to resolve %q against", rel)
	}

	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || escapes(clean) {
		return "", fmt.Errorf("path escapes root: %s", rel)
	}

	full := filepath.Join(root, clean)
	if !Contains(root, full) {
		return "", fmt.Errorf("path escapes root: %s", rel)
	}
	return full, nil
}

// Contains reports whether path is root or sits under it, comparing the paths
// as written.
func Contains(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && !escapes(rel)
}

// EnsureWithin is Contains after symlinks are resolved, so a link planted
// inside the root -- by a cloned repo, say -- cannot be followed out of it. It
// walks up to the deepest ancestor that exists, since the path may be one about
// to be created.
//
// Callers that touch the filesystem use this; callers that only describe it can
// use Contains, which needs no syscalls.
func EnsureWithin(root, full string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}

	for p := full; ; {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			if !Contains(realRoot, resolved) {
				return fmt.Errorf("path escapes root: %s", full)
			}
			return nil
		}

		parent := filepath.Dir(p)
		if parent == p {
			return fmt.Errorf("path escapes root: %s", full)
		}
		p = parent
	}
}

// escapes reports whether a cleaned relative path points outside its root.
func escapes(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
