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

func (g *Graph) GetWorkspaceGraph() (*WorkspaceNode, error) {
	g.mu.RLock()
	
	if g.WorkspaceGraph != nil {
		defer g.mu.RUnlock()
		return g.WorkspaceGraph, nil
	}
	
	g.mu.RUnlock()

	g.mu.Lock()

	if g.WorkspaceGraph == nil {
		if err := g.BuildWorkspaceGraph(); err != nil {
			g.mu.Unlock()
			return nil, err
		}
		
		wg := g.WorkspaceGraph
		
		g.mu.Unlock()
		
		onGraphBuilt := g.AfterWorkspaceGraphBuild
		if onGraphBuilt != nil {
			onGraphBuilt(wg)
		}

		return wg, nil
	}
	
	wg := g.WorkspaceGraph

	g.mu.Unlock()
	
	return wg, nil
}
