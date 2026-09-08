package graph

// SharedDatabasesUnder returns the shared -- proxified -- databases at or under
// the given nodes, naming each one by id and name. Passing no ids asks about
// the whole workspace.
//
// Containment is the graph's question to answer, not the frontend's. The
// frontend holds a serialized snapshot: it does not know that a node answers to
// both its config id and its URI, that the workspace keeps a flat list of every
// database alongside the folder tree, or that a folder can be laid out before
// its contents are read (see FolderNode.Resolved). Asking here means one set of
// containment rules rather than a second copy that agrees until it does not.
//
// It matters most where the answer decides whether something has to be revoked
// before it is deleted. A database missed there is a credential left working
// with nothing left pointing at it.
//
// Ids the graph does not know are skipped, and a database reached twice is
// returned once, so callers can pass a whole selection without deduplicating it
// first.
func (g *Graph) SharedDatabasesUnder(ids []string) []DatabaseRef {
	g.mu.RLock()
	defer g.mu.RUnlock()

	roots := g.lookupAll(ids)
	if len(ids) == 0 && g.WorkspaceGraph != nil {
		roots = []Node{g.WorkspaceGraph}
	}

	seen := make(map[Node]bool)
	found := make([]DatabaseRef, 0)

	var walk func(n Node)
	walk = func(n Node) {
		// A schema item is the one thing not worth descending into: it holds no
		// database, and a loaded schema is large. index.go leaves them out for
		// the same reason.
		if n == nil || seen[n] || isSchemaItem(n) {
			return
		}
		seen[n] = true

		// A database directory can hold folders of its own, so finding one is
		// not a reason to stop descending.
		if db, ok := n.(*DBInstanceNode); ok && db.Proxified {
			found = append(found, DatabaseRef{ID: db.ID, Name: db.Name})
		}

		for _, child := range n.GetChildren() {
			walk(child)
		}
	}

	for _, root := range roots {
		walk(root)
	}

	return found
}
