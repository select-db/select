package git

import (
	"context"

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

	// Pending state for the two-phase link flow (set by LinkExistingRepo,
	// consumed by CompleteLinkExistingRepo).
	pendingLinkWorkspaceID string
	pendingLinkBranch      string
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

// workspaceRootPath resolves the absolute filesystem path for a given workspace
// ID using the shared graph helpers.
func (s *Git) workspaceRootPath(workspaceID string) (string, error) {
	return graph.WorkspaceRootPath(workspaceID)
}
