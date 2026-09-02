package graph

// A picker asks about the whole workspace, not the part of it the graph has
// read, and the answer it needs is a handful of rows rather than a copy of the
// project. So it asks a question instead of taking a listing: a pattern, a
// scope, a cap.
//
// The walk scores names as strings and keeps only the best `Limit` of them; the
// file nodes — which read a sidecar apiece — are built for the survivors, after
// the walk. A query against a workspace of any size therefore costs one
// directory walk and at most `Limit` sidecar reads.
//
// This is the shape the editors that do this well converge on: one matcher
// taking (pattern, scope, limit, cancellation) rather than an endpoint per
// question, with identity lookups — by ID, by URI — kept separate.

import (
	"container/heap"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// FileQuery selects files by name. The zero value matches every file in the
// workspace, capped at DefaultFileQueryLimit.
type FileQuery struct {
	// Pattern is matched case-insensitively against the file name, and failing
	// that against its path. Empty matches everything.
	Pattern string `json:"pattern"`

	// FolderURI limits the search to one folder and everything below it. Empty
	// searches the whole workspace.
	FolderURI string `json:"folderURI"`

	// Extensions limits the search to these file extensions, ".sql" style.
	Extensions []string `json:"extensions"`

	// Depth limits how far below the scope the search goes: 0 is the whole
	// subtree, 1 the scope's own files and no deeper.
	Depth int `json:"depth"`

	// Limit caps how many files come back. Zero means DefaultFileQueryLimit.
	Limit int `json:"limit"`
}

const (
	// DefaultFileQueryLimit is what a query without a limit gets: enough rows
	// for a picker to rank and show, few enough to keep the answer small.
	DefaultFileQueryLimit = 200

	// MaxFileQueryLimit bounds what a caller can ask for, so no query can turn
	// into a copy of the workspace.
	MaxFileQueryLimit = 1000
)

// Match quality, best first. The caller ranks the rows it gets back; this only
// decides which ones survive the cap.
const (
	scoreNoMatch = iota - 1
	scorePathContains
	scoreNameContains
	scoreNameWordStart
	scoreNamePrefix
	scoreNameExact
)

type fileMatch struct {
	path  string
	uri   string
	name  string
	score int
}

// FindFiles returns the files matching a query, best matches first.
//
// It reads the filesystem rather than the graph, so it sees files in folders
// that have never been opened, and it adds nothing to the graph.
func (g *Graph) FindFiles(ctx context.Context, q FileQuery) ([]*FileNode, error) {
	g.mu.RLock()
	if g.WorkspaceGraph == nil {
		g.mu.RUnlock()
		return []*FileNode{}, nil
	}
	workspaceID := g.WorkspaceGraph.ID
	g.mu.RUnlock()

	fsCtx, err := NewWorkspaceFS(workspaceID)
	if err != nil {
		return nil, err
	}

	root := fsCtx.WorkspaceRoot
	if q.FolderURI != "" {
		scoped, ok := fsCtx.Path(q.FolderURI)
		if !ok {
			return []*FileNode{}, nil
		}
		root = scoped
	}

	limit := q.Limit
	if limit <= 0 {
		limit = DefaultFileQueryLimit
	}
	if limit > MaxFileQueryLimit {
		limit = MaxFileQueryLimit
	}

	pattern := strings.ToLower(q.Pattern)
	best := &fileMatchHeap{limit: limit}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		relSlash, ok := fsCtx.Rel(path)
		if !ok {
			return nil
		}
		relSlash = filepath.ToSlash(filepath.Clean(relSlash))

		if IsInternalWorkspacePath(relSlash) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if q.Depth > 0 && path != root && depthBelow(root, path) >= q.Depth {
				return fs.SkipDir
			}
			return nil
		}

		name := d.Name()
		if IsInternalWorkspaceFile(name) {
			return nil
		}
		if !hasExtension(name, q.Extensions) {
			return nil
		}

		score := scoreFileName(name, relSlash, pattern)
		if score == scoreNoMatch {
			return nil
		}

		best.offer(fileMatch{path: path, uri: fsCtx.URI(relSlash), name: name, score: score})
		return nil
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("walk workspace for files: %w", err)
	}

	matches := best.sorted()

	// The sidecar read per file is why nodes are built here and not during the
	// walk: only the rows that made the cut cost one.
	files := make([]*FileNode, 0, len(matches))
	for _, m := range matches {
		relSlash, _ := fsCtx.Rel(m.path)
		files = append(files, newFSFileNode(m.path, m.uri, fsCtx.ParentURI(filepath.ToSlash(relSlash)), m.name))
	}

	return files, nil
}

// depthBelow counts the path segments between a root and one of its
// descendants: a direct child is 1.
func depthBelow(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0
	}
	return len(strings.Split(filepath.ToSlash(rel), "/"))
}

func hasExtension(name string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, want := range extensions {
		if ext == strings.ToLower(want) {
			return true
		}
	}
	return false
}

// scoreFileName rates a file against a lowercased pattern: an exact name beats
// a prefix, which beats a match at a word boundary, which beats a match
// anywhere in the name, which beats a match elsewhere in the path.
func scoreFileName(name, relPath, pattern string) int {
	if pattern == "" {
		return scoreNameContains
	}

	lowerName := strings.ToLower(name)

	switch {
	case lowerName == pattern:
		return scoreNameExact
	case strings.HasPrefix(lowerName, pattern):
		return scoreNamePrefix
	}

	if idx := strings.Index(lowerName, pattern); idx > 0 {
		switch lowerName[idx-1] {
		case '_', '-', '.', ' ':
			return scoreNameWordStart
		}
		return scoreNameContains
	}

	if strings.Contains(strings.ToLower(relPath), pattern) {
		return scorePathContains
	}

	return scoreNoMatch
}

// fileMatchHeap keeps the best `limit` matches seen so far, with the worst of
// them at the root so a better one can replace it. Bounding here rather than
// sorting at the end is what keeps a query against a large workspace from
// building a list the size of the workspace.
type fileMatchHeap struct {
	limit   int
	matches []fileMatch
}

func (h *fileMatchHeap) Len() int { return len(h.matches) }

// Less orders the worst match first: lower score, then the name that sorts
// later, so that ties are broken the same way on every run.
func (h *fileMatchHeap) Less(i, j int) bool {
	a, b := h.matches[i], h.matches[j]
	if a.score != b.score {
		return a.score < b.score
	}
	if a.name != b.name {
		return a.name > b.name
	}
	return a.uri > b.uri
}

func (h *fileMatchHeap) Swap(i, j int) {
	h.matches[i], h.matches[j] = h.matches[j], h.matches[i]
}

func (h *fileMatchHeap) Push(x any) { h.matches = append(h.matches, x.(fileMatch)) }

func (h *fileMatchHeap) Pop() any {
	last := len(h.matches) - 1
	m := h.matches[last]
	h.matches = h.matches[:last]
	return m
}

func (h *fileMatchHeap) offer(m fileMatch) {
	if len(h.matches) < h.limit {
		heap.Push(h, m)
		return
	}
	// h.matches[0] is the worst kept match; swap it out only for a better one.
	if betterThan(m, h.matches[0]) {
		h.matches[0] = m
		heap.Fix(h, 0)
	}
}

func betterThan(a, b fileMatch) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.name != b.name {
		return a.name < b.name
	}
	return a.uri < b.uri
}

// sorted returns the kept matches, best first.
func (h *fileMatchHeap) sorted() []fileMatch {
	matches := make([]fileMatch, len(h.matches))
	copy(matches, h.matches)
	sort.Slice(matches, func(i, j int) bool { return betterThan(matches[i], matches[j]) })
	return matches
}
