package graph

import (
	"maps"
	"slices"
	"time"

	"selectDb/internal/utils"
)

// The frontend is handed copies of nodes, never the nodes themselves. Wails
// marshals a result or event payload after the call that produced it returned,
// and the watcher keeps rewriting those slices, so encoding a live slice header
// panics with "reflect: slice index out of range".
//
// Metadata interface values and query results stay shared. Both are replaced
// wholesale rather than edited in place, so no copy points at rewritten memory.

// The window every workspaceGraphUpdated emit shares. One constant because a
// Debouncer keeps the window it was created with, so a per-call argument would
// be whatever the first caller happened to pass.
const graphUpdateWindow = 100 * time.Millisecond

// EmitWorkspaceGraphUpdated queues the tree for the frontend. The copy is taken
// when the debounce fires, not per call, so a checkout touching a thousand
// files copies the tree once instead of a thousand times.
func EmitWorkspaceGraphUpdated(g *Graph) {
	utils.DebouncedEventsEmitFunc("workspaceGraphUpdated", graphUpdateWindow, func() []interface{} {
		return []interface{}{SnapshotWorkspaceGraph(g)}
	})
}

// SnapshotWorkspaceGraph copies the tree under the read lock. Callers already
// holding it clone the node directly.
func SnapshotWorkspaceGraph(g *Graph) *WorkspaceNode {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.WorkspaceGraph.Clone()
}

type cloneable[T any] interface{ Clone() T }

func cloneAll[T cloneable[T]](nodes []T) []T {
	if nodes == nil {
		return nil
	}
	clones := make([]T, len(nodes))
	for i, node := range nodes {
		clones[i] = node.Clone()
	}
	return clones
}

func (wg *WorkspaceNode) Clone() *WorkspaceNode {
	if wg == nil {
		return nil
	}
	out := *wg
	out.User = wg.User.Clone()
	out.Folders = cloneAll(wg.Folders)
	out.DBInstances = cloneAll(wg.DBInstances)
	return &out
}

func (u *UserNode) Clone() *UserNode {
	if u == nil {
		return nil
	}
	out := *u
	return &out
}

func (f *FolderNode) Clone() *FolderNode {
	if f == nil {
		return nil
	}
	out := *f
	out.Files = cloneAll(f.Files)
	out.Folders = cloneAll(f.Folders)
	out.DBInstances = cloneAll(f.DBInstances)
	out.Variables = maps.Clone(f.Variables)
	out.Badges = slices.Clone(f.Badges)
	return &out
}

func (f *FileNode) Clone() *FileNode {
	if f == nil {
		return nil
	}
	out := *f
	out.Databases = slices.Clone(f.Databases)
	out.Badges = slices.Clone(f.Badges)
	out.QueryResults = maps.Clone(f.QueryResults)
	out.PlanResults = maps.Clone(f.PlanResults)
	out.ExplainResults = maps.Clone(f.ExplainResults)
	return &out
}

func (d *DBInstanceNode) Clone() *DBInstanceNode {
	if d == nil {
		return nil
	}
	out := *d
	if d.SSH != nil {
		ssh := *d.SSH
		out.SSH = &ssh
	}
	out.Children = cloneAll(d.Children)
	out.Files = cloneAll(d.Files)
	out.Folders = cloneAll(d.Folders)
	return &out
}

func (d *DBInstanceItemNode) Clone() *DBInstanceItemNode {
	if d == nil {
		return nil
	}
	out := *d
	out.Badges = slices.Clone(d.Badges)
	out.Children = cloneAll(d.Children)
	return &out
}
