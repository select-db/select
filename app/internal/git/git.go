package git

import (
	"context"
	"fmt"

	"selectDb/internal/db/generated"
	"selectDb/internal/fs_provider"
	"selectDb/internal/graph"
)

// Git encapsulates all Git-related operations for the desktop app.
// It orchestrates local git commands for version control functionality.
type Git struct {
	ctx context.Context

	Queries    *generated.Queries
	FSProvider *fs_provider.FSProvider
	Graph      *graph.Graph
}

func New(
	queries *generated.Queries,
	fsProvider *fs_provider.FSProvider,
	graph *graph.Graph,
) *Git {
	return &Git{
		Queries:    queries,
		FSProvider: fsProvider,
		Graph:      graph,
	}
}

func (s *Git) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// openWorkspaceRoot returns the folder git runs in: the one the app has open.
func openWorkspaceRoot() (string, error) {
	_, root, ok := graph.OpenWorkspace()
	if !ok {
		return "", fmt.Errorf("no workspace folder is open")
	}
	return root, nil
}
