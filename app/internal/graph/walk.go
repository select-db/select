package graph

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Everything that reads the workspace off disk — the build, a folder resolving
// itself, a file query, the watcher registering its watches and scanning a new
// folder — skips the same things: git internals, sidecars, OS droppings. The
// two helpers here apply that filter once, so a new rule is added in one place
// rather than in each walk that happens to remember it.

// Entry is one thing found on disk: where it is, what the workspace calls it,
// and the directory entry it came from.
type Entry struct {
	fs.DirEntry

	// Path is absolute; Rel is workspace-relative and slash-separated, the form
	// a URI is built from.
	Path string
	Rel  string

	fs *WorkspaceFS
}

// URI is what this entry is addressed by. Built on demand: a walk sees far more
// entries than it keeps.
func (e Entry) URI() string { return e.fs.URI(e.Rel) }

// ParentURI is the folder this entry hangs from.
func (e Entry) ParentURI() string { return e.fs.ParentURI(e.Rel) }

// Walk visits everything under the workspace root, deepest paths last and the
// root itself not visited. Returning fs.SkipDir from fn skips a directory's
// contents, as with fs.WalkDir; unreadable entries are skipped rather than
// failing the walk.
func (c *WorkspaceFS) Walk(fn func(Entry) error) error {
	return c.WalkFrom(c.WorkspaceRoot, fn)
}

// WalkFrom is Walk starting at root, which must be inside the workspace.
func (c *WorkspaceFS) WalkFrom(root string, fn func(Entry) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}

		entry, ok := c.entry(path, d)
		if !ok {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return fn(entry)
	})
}

// ReadDir visits what is directly inside dirPath, applying the same filter.
func (c *WorkspaceFS) ReadDir(dirPath string, fn func(Entry) error) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, d := range entries {
		entry, ok := c.entry(filepath.Join(dirPath, d.Name()), d)
		if !ok {
			continue
		}
		if err := fn(entry); err != nil {
			return err
		}
	}
	return nil
}

// entry describes a path, and reports false for one no reader wants to see.
func (c *WorkspaceFS) entry(path string, d fs.DirEntry) (Entry, bool) {
	rel, inside := c.Rel(path)
	if !inside || IsInternalWorkspacePath(rel) {
		return Entry{}, false
	}
	if !d.IsDir() && IsInternalWorkspaceFile(d.Name()) {
		return Entry{}, false
	}
	return Entry{DirEntry: d, Path: path, Rel: rel, fs: c}, true
}
