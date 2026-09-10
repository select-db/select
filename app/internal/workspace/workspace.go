package workspace

import (
	"context"

	"selectDb/internal/db/generated"
	"selectDb/internal/graph"
)

// ReloadHooks lets this package reach the frontend without importing it. Set by
// the app in Startup.
type ReloadHooks struct {
	BuildWorkspaceGraph       func() error
	EmitWorkspaceGraphUpdated func()
	EmitWorkspaceClosed       func()
}

type Workspace struct {
	Queries     *generated.Queries
	Graph       *graph.Graph
	ReloadHooks *ReloadHooks
	// PullFunc fetches server-side changes locally. Wired by the app layer to avoid circular deps.
	PullFunc func(ctx context.Context, userID string) error
}

func New(Queries *generated.Queries, Graph *graph.Graph) *Workspace {
	return &Workspace{
		Queries: Queries,
		Graph:   Graph,
	}
}
