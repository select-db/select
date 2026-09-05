package graph

// A picker asks about the whole workspace, not the part of it the graph has
// read, and wants a bounded number of rows back. The walk here scores names as
// strings and keeps the best Limit of them in a heap; nodes, which cost a
// sidecar read apiece, are built only for the survivors. One directory walk and
// at most Limit sidecar reads, whatever the workspace holds.

import (
	"container/heap"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

// FileQuery selects files by name. The zero value matches every file in the
// workspace, capped at DefaultFileQueryLimit.
type FileQuery struct {
	// Matched case-insensitively against the file name, and failing that
	// against its path. Empty matches everything.
	Pattern string `json:"pattern"`

	// Limits the search to one folder and everything below it.
	FolderURI string `json:"folderURI"`

	// Limits the search to these extensions, ".sql" style.
	Extensions []string `json:"extensions"`

	// How far below the scope to go: 0 is the whole subtree, 1 the scope's own
	// files and no deeper.
	Depth int `json:"depth"`

	// Caps how many files come back. Zero means DefaultFileQueryLimit.
	Limit int `json:"limit"`
}

const (
	// Enough rows for a picker to rank and show, few enough to stay small.
	DefaultFileQueryLimit = 200

	// The ceiling on what a caller can ask for, so no query turns into a copy
	// of the workspace.
	MaxFileQueryLimit = 1000
)

// Match quality, best last. The caller ranks the rows it gets back; this only
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
	path   string
	uri    string
	parent string
	name   string
	score  int
}

// FindFiles returns the files matching a query, best matches first.
//
// It reads the filesystem rather than the graph, so it sees files in folders
// that have never been opened, and it adds nothing to the graph.
func (g *Graph) FindFiles(ctx context.Context, q FileQuery) ([]*FileNode, error) {
	g.mu.RLock()
	workspace := g.WorkspaceGraph
	g.mu.RUnlock()

	if workspace == nil {
		return []*FileNode{}, nil
	}

	fsCtx, err := NewWorkspaceFS(workspace.ID)
	if err != nil {
		return nil, err
	}

	root := fsCtx.WorkspaceRoot
	if q.FolderURI != "" {
		scoped, inWorkspace := fsCtx.Path(q.FolderURI)
		if !inWorkspace {
			return []*FileNode{}, nil
		}
		root = scoped
	}

	limit := q.Limit
	if limit <= 0 {
		limit = DefaultFileQueryLimit
	}

	pattern := strings.ToLower(q.Pattern)
	extensions := lowered(q.Extensions)
	best := &fileMatchHeap{limit: min(limit, MaxFileQueryLimit)}

	err = fsCtx.WalkFrom(root, func(entry Entry) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		if entry.IsDir() {
			if q.Depth > 0 && depthBelow(root, entry.Path) >= q.Depth {
				return fs.SkipDir
			}
			return nil
		}

		name := entry.Name()
		if !hasExtension(name, extensions) {
			return nil
		}

		score := scoreFileName(name, entry.Rel, pattern)
		if score == scoreNoMatch {
			return nil
		}

		best.offer(fileMatch{
			path:   entry.Path,
			uri:    entry.URI(),
			parent: entry.ParentURI(),
			name:   name,
			score:  score,
		})
		return nil
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("walk workspace for files: %w", err)
	}

	// Only the rows that made the cut pay for a sidecar read.
	matches := best.sorted()
	files := make([]*FileNode, 0, len(matches))
	for _, m := range matches {
		files = append(files, FileNodeFromDisk(m.path, m.uri, m.parent))
	}

	return files, nil
}

func lowered(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.ToLower(v)
	}
	return out
}

func hasExtension(name string, lowerExtensions []string) bool {
	return len(lowerExtensions) == 0 ||
		slices.Contains(lowerExtensions, strings.ToLower(filepath.Ext(name)))
}

// depthBelow counts the path segments between a root and a descendant: a direct
// child is 1.
func depthBelow(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0
	}
	return len(strings.Split(filepath.ToSlash(rel), "/"))
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

// betterThan is the whole ordering: score first, then name and URI so that ties
// come out the same way on every run.
func betterThan(a, b fileMatch) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.name != b.name {
		return a.name < b.name
	}
	return a.uri < b.uri
}

// fileMatchHeap keeps the best `limit` matches seen so far, worst at the root so
// a better one can replace it.
type fileMatchHeap struct {
	limit   int
	matches []fileMatch
}

func (h *fileMatchHeap) Len() int           { return len(h.matches) }
func (h *fileMatchHeap) Less(i, j int) bool { return betterThan(h.matches[j], h.matches[i]) }
func (h *fileMatchHeap) Swap(i, j int)      { h.matches[i], h.matches[j] = h.matches[j], h.matches[i] }
func (h *fileMatchHeap) Push(x any)         { h.matches = append(h.matches, x.(fileMatch)) }

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
	if betterThan(m, h.matches[0]) {
		h.matches[0] = m
		heap.Fix(h, 0)
	}
}

// sorted returns the kept matches, best first.
func (h *fileMatchHeap) sorted() []fileMatch {
	slices.SortFunc(h.matches, func(a, b fileMatch) int {
		if betterThan(a, b) {
			return -1
		}
		if betterThan(b, a) {
			return 1
		}
		return 0
	})
	return h.matches
}
