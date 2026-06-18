package search

import (
	"context"

	"selectDb/internal/graph"
)

// Search encapsulates all search-related operations using ripgrep.
type Search struct {
	ctx context.Context

	Graph *graph.Graph
}

func New(graph *graph.Graph) *Search {
	return &Search{
		Graph: graph,
	}
}

func (s *Search) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// workspaceRootPath resolves the absolute filesystem path for a given workspace
// ID using the shared graph helpers.
func (s *Search) workspaceRootPath(workspaceID string) (string, error) {
	return graph.WorkspaceRootPath(workspaceID)
}
