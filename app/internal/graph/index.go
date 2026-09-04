package graph

// nodeIndex maps every ID a node answers to onto the node, so a lookup is a map
// hit rather than a walk from the root. It holds folders, files, db instances
// and the workspace, but not schema items: a schema load replaces a db
// instance's children without going through this package, so an entry for one
// could outlive its node. FindDbItemNodeById walks a single instance instead.
//
// Guarded by Graph.mu, like the graph it indexes: every helper here assumes the
// caller holds it.
type nodeIndex map[string]Node

func newNodeIndex() nodeIndex {
	return make(nodeIndex)
}

func isSchemaItem(n Node) bool {
	_, is := n.(*DBInstanceItemNode)
	return is
}

// add registers a node under every ID it answers to — a db instance answers to
// both its config ID and its URI.
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

func (ix nodeIndex) addSubtree(n Node) {
	if n == nil || isSchemaItem(n) {
		return
	}
	ix.add(n)
	for _, child := range n.GetChildren() {
		ix.addSubtree(child)
	}
}

func (ix nodeIndex) removeSubtree(n Node) {
	if n == nil || isSchemaItem(n) {
		return
	}
	for _, id := range n.GetIDs() {
		// Only drop an entry still pointing at this node: a replacement has
		// already claimed the ID.
		if ix[id] == n {
			delete(ix, id)
		}
	}
	for _, child := range n.GetChildren() {
		ix.removeSubtree(child)
	}
}

// ensureIndex fills the index from the current graph, after a build replaces
// the tree wholesale.
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

// lookupAll returns the nodes for the given IDs, skipping the ones the graph
// does not know.
func (g *Graph) lookupAll(ids []string) []Node {
	nodes := make([]Node, 0, len(ids))
	for _, id := range ids {
		if n := g.lookup(id); n != nil {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// parentsOf returns the nodes a node hangs from. A db instance has two, its
// folder and the workspace's flat list, and both move together.
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
// "db_instance" — and "" when it holds nothing.
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
