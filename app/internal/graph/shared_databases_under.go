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
	// The nil check is load-bearing: a nil *WorkspaceNode put in a Node is not
	// == nil, so walk would take it for a real node and dereference it. The
	// graph has no workspace between InvalidateWorkspaceGraph and the rebuild.
	if len(ids) == 0 && g.WorkspaceGraph != nil {
		roots = []Node{g.WorkspaceGraph}
	}

	// A database is reachable more than once -- from its folder, from the
	// workspace's flat list, and from any overlapping id the caller passed --
	// and each one must be revoked once.
	seen := make(map[string]bool)
	found := make([]DatabaseRef, 0)

	for _, root := range roots {
		walkSubtree(root, func(n Node) {
			// Note this does not stop at a database: one can hold folders of its
			// own, and a database inside those still needs revoking.
			db, ok := n.(*DBInstanceNode)
			if !ok || !db.Proxified || seen[db.ID] {
				return
			}
			seen[db.ID] = true
			found = append(found, DatabaseRef{ID: db.ID, Name: db.Name})
		})
	}

	return found
}
