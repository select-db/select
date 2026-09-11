package graph

import (
	"maps"
	"slices"
)

// The frontend is handed copies of nodes, never the nodes themselves.
//
// Wails marshals a bound call's result, and an event's payload, after the call
// that produced it returned. The watcher keeps writing to the same slices in
// the meantime -- turning a new directory into a database node is exactly that
// -- and the JSON encoder reading a slice header while it is rewritten panics
// with "reflect: slice index out of range", taking the app with it.
//
// Interface values (a schema item's metadata) and query results are shared:
// both are replaced wholesale rather than edited in place, so the copy's own
// header always describes memory nobody rewrites.

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

func cloneAll[T cloneable[T]](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	for i, node := range in {
		out[i] = node.Clone()
	}
	return out
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
