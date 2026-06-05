package graph

import (
	"selectDb/backend/db/generated"
	"sync"
)

type Graph struct {
	Queries        *generated.Queries
	WorkspaceGraph *WorkspaceNode

	// Optional; app sets to System.LoadAllDatabaseSchemas.
	// Invoked without Graph.mu held after a successful BuildWorkspaceGraph.
	AfterWorkspaceGraphBuild func(ws *WorkspaceNode)

	mu sync.RWMutex
}

func New(Queries *generated.Queries) *Graph {
	return &Graph{
		Queries: Queries,
	}
}

// InvalidateWorkspaceGraph clears the cached graph so the next GetWorkspaceGraph
// rebuilds from the current server's DB and filesystem. Call when switching server.
func (g *Graph) InvalidateWorkspaceGraph() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.WorkspaceGraph = nil
}
