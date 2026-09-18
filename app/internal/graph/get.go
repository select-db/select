package graph

// FindDbItemNodeById walks the schema item tree of a DB instance and returns the
// node with the given ID, or nil if not found.
func (g *Graph) FindDbItemNodeById(dbInstanceID, nodeID string) *DBInstanceItemNode {
	dbNode := g.GetDBInstanceNodeByID(dbInstanceID)
	if dbNode == nil {
		return nil
	}
	stack := make([]*DBInstanceItemNode, len(dbNode.Children))
	copy(stack, dbNode.Children)
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.ID == nodeID {
			return node
		}
		stack = append(stack, node.Children...)
	}
	return nil
}

// GetWorkspaceGraph returns a copy of the workspace graph for the frontend.
// See clone.go: what crosses the bridge is never the tree the watcher writes to.
func (g *Graph) GetWorkspaceGraph() (*WorkspaceNode, error) {
	if _, err := EnsureWorkspaceGraph(g); err != nil {
		return nil, err
	}
	return SnapshotWorkspaceGraph(g), nil
}

// EnsureWorkspaceGraph returns the tree itself, building it on first use.
//
// A function rather than a method: every method on Graph is frontend API, and
// this hands out the live tree, which is the app's to hold and no one else's.
//
// It also guarantees the graph is indexed: WorkspaceGraph is an exported field,
// so a graph can be assigned rather than built, and every lookup goes through
// the index.
func EnsureWorkspaceGraph(g *Graph) (*WorkspaceNode, error) {
	g.mu.RLock()
	if g.WorkspaceGraph != nil && g.index != nil {
		defer g.mu.RUnlock()
		return g.WorkspaceGraph, nil
	}
	g.mu.RUnlock()

	g.mu.Lock()

	built := false
	switch {
	case g.WorkspaceGraph == nil:
		if err := g.BuildWorkspaceGraph(); err != nil {
			g.mu.Unlock()
			return nil, err
		}
		built = true
	case g.index == nil:
		g.ensureIndex()
	}

	wg := g.WorkspaceGraph
	onGraphBuilt := g.AfterWorkspaceGraphBuild

	g.mu.Unlock()

	if built && onGraphBuilt != nil {
		onGraphBuilt(wg)
	}

	return wg, nil
}
