package graph

// The workspace graph is a tree of pointers, so every lookup by ID was a
// depth-first walk from the root — one per mutation, per filesystem event, per
// file opened — and each visit allocated a []Node through GetChildren. At a few
// thousand files that is milliseconds of walking to answer what is a map
// lookup, and it is paid under Graph.mu while the UI waits for it.
//
// nodeIndex keeps the tree addressable by ID. It covers the filesystem tree:
// folders, files, db instances and the workspace itself. Schema items
// (DBInstanceItemNode) are deliberately left out — a schema load replaces a db
// instance's children wholesale without going through the graph package, so an
// index over them would go stale; the one lookup they need walks a single db
// instance in FindDbItemNodeById.
//
// The index lives beside the tree and is maintained by the same operations:
// filled by a build, updated by the inserts and deletes Mutate applies, and by
// the files ResolveFolder materializes. It is guarded by Graph.mu, so the
// unexported helpers below all assume the caller holds it.
type nodeIndex map[string]Node

func newNodeIndex() nodeIndex {
	return make(nodeIndex)
}

// add registers a node under every ID it answers to. A db instance answers to
// both its config ID and its URI, and callers look it up by either.
func (ix nodeIndex) add(n Node) {
	if n == nil {
		return
	}
	for _, id := range n.GetIDs() {
		if id != "" {
			ix[id] = n
		}
	}
}

// addSubtree registers a node and everything below it, stopping at schema
// items, which the index does not cover.
func (ix nodeIndex) addSubtree(n Node) {
	if n == nil {
		return
	}
	if _, isItem := n.(*DBInstanceItemNode); isItem {
		return
	}
	ix.add(n)
	for _, child := range n.GetChildren() {
		ix.addSubtree(child)
	}
}

// removeSubtree unregisters a node and everything below it.
func (ix nodeIndex) removeSubtree(n Node) {
	if n == nil {
		return
	}
	if _, isItem := n.(*DBInstanceItemNode); isItem {
		return
	}
	for _, id := range n.GetIDs() {
		// Only drop the entry when it still points at this node: a replaced
		// node has already claimed the ID.
		if ix[id] == n {
			delete(ix, id)
		}
	}
	for _, child := range n.GetChildren() {
		ix.removeSubtree(child)
	}
}

// ensureIndex fills the index from the current graph. Called after a build,
// which replaces the tree wholesale.
func (g *Graph) ensureIndex() {
	g.index = newNodeIndex()
	g.index.addSubtree(g.WorkspaceGraph)
}

// lookup returns the node with the given ID, or nil.
func (g *Graph) lookup(id string) Node {
	if g.index == nil {
		return nil
	}
	return g.index[id]
}

// lookupAll returns the nodes for the given IDs, skipping IDs the graph does
// not know. It replaces FindNodesByIds on the hot paths: same result, without
// the walk.
func (g *Graph) lookupAll(ids []string) []Node {
	nodes := make([]Node, 0, len(ids))
	for _, id := range ids {
		if n := g.lookup(id); n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// parentsOf returns the nodes a node hangs from. A db instance has two — its
// folder and the workspace, which keeps a flat list of every instance — and
// both must be updated when it is added or removed.
func (g *Graph) parentsOf(n Node) []Node {
	return g.lookupAll(n.GetParentIDs())
}

// attach adds a node to each of its parents and indexes it.
func (g *Graph) attach(n Node) {
	for _, parent := range g.parentsOf(n) {
		parent.AddChild(n)
	}
	g.index.addSubtree(n)
}

// detachByIDs removes the nodes with the given IDs from their parents and
// unindexes them, reporting whether anything was removed.
func (g *Graph) detachByIDs(ids []string) bool {
	removed := false
	for _, n := range g.lookupAll(ids) {
		for _, parent := range g.parentsOf(n) {
			if parent.RemoveChildByIDs(n.GetIDs()) {
				removed = true
			}
		}
		g.index.removeSubtree(n)
	}
	return removed
}

// GetFolderNodeByID returns the FolderNode with the given ID from the current
// graph, or nil when the ID is unknown or names something else.
func (g *Graph) GetFolderNodeByID(id string) *FolderNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	folder, _ := g.lookup(id).(*FolderNode)
	return folder
}

// NodeKind returns what the graph holds under an ID — "file", "folder" or
// "db_instance" — and "" when it holds nothing. It answers the question a
// filesystem event asks about a path that has just disappeared, where the node
// is all that is left to say what it was.
func (g *Graph) NodeKind(id string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	switch g.lookup(id).(type) {
	case *FileNode:
		return "file"
	case *FolderNode:
		return "folder"
	case *DBInstanceNode:
		return "db_instance"
	default:
		return ""
	}
}
