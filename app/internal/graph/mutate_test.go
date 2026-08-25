package graph

import (
	"selectDb/internal/db/generated"
)

func setupGraph() *Graph {
	g := New(&generated.Queries{})
	g.WorkspaceGraph = &WorkspaceNode{
		ID:          "ws-1",
		Type:        "workspace",
		Name:        "workspace",
		Folders:     []*FolderNode{},
		DBInstances: []*DBInstanceNode{},
	}
	g.WorkspaceGraph.AddChild(&FolderNode{
		ID:   "root",
		Name: "root",
	})
	return g
}
