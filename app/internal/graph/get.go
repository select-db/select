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

// GetWorkspaceGraph returns the workspace graph, building it on first use.
//
// It also guarantees the graph is indexed: WorkspaceGraph is an exported field,
// so a graph can be assigned rather than built, and every lookup goes through
// the index.
func (g *Graph) GetWorkspaceGraph() (*WorkspaceNode, error) {
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
