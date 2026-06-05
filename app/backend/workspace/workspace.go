package workspace

import (
	"context"

	"selectDb/backend/db/generated"
	"selectDb/backend/fs_provider"
)

// ReloadHooks are optional callbacks for app-level workspace actions (switch, create, delete).
// Set by the app in Startup so the workspace package can trigger graph rebuild and emit.
type ReloadHooks struct {
	BuildWorkspaceGraph       func() error
	EmitWorkspaceGraphUpdated func()
	RunSwitchOrLogout         func(ctx context.Context, userID string) bool
	// ReconcileGitRemote materializes the workspace's git repository to match
	// its configured remote before the graph is rebuilt. Optional.
	ReconcileGitRemote func(workspaceID string)
}

type Workspace struct {
	Queries     *generated.Queries
	FSProvider  *fs_provider.FSProvider
	ReloadHooks *ReloadHooks
	// PullFunc fetches server-side changes locally. Wired by the app layer to avoid circular deps.
	PullFunc func(ctx context.Context, userID string) error
}

func New(Queries *generated.Queries, FSProvider *fs_provider.FSProvider) *Workspace {
	return &Workspace{
		Queries:    Queries,
		FSProvider: FSProvider,
	}
}
