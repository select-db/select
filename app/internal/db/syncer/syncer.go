package syncer

import (
	"context"
	"sync"
	"time"

	"selectDb/internal/db/generated"
	syncworkspace "selectDb/internal/db/syncer/workspace"
	"selectDb/internal/graph"
)

const syncDebounceDelay = 500 * time.Millisecond

// SwitchOrLogoutHandler is called when the current workspace or workspace_to_user was deleted
// and the syncer has switched to another workspace or must logout.
type SwitchOrLogoutHandler interface {
	// OnAfterWorkspaceSwitch is called after the syncer has set a new current workspace and built the graph.
	// The app should emit workspaceGraphUpdated (e.g. via DebouncedEventsEmit).
	OnAfterWorkspaceSwitch()
	// OnLogout is called when the user has no workspaces left after a delete.
	OnLogout()
}

type Syncer struct {
	ctx     context.Context
	Queries *generated.Queries
	Graph   *graph.Graph

	Workspace syncworkspace.Ensurer

	// SwitchOrLogout is optional. When set, applyDeleteRow uses it after switching workspace or when user has no workspaces.
	SwitchOrLogout SwitchOrLogoutHandler

	// EmitRolesUpdated is optional. When set, called after sync applies role/user_to_role/permission changes.
	EmitRolesUpdated func()

	// FetchFunc overrides api.Fetch (UT).
	FetchFunc func(ctx context.Context, method, endpoint string, payload interface{}, headers map[string]string, response interface{}) error

	debounceMu     sync.Mutex
	debounceTimer  *time.Timer
	debounceUserID string
}

func New(Queries *generated.Queries, Graph *graph.Graph, Workspace syncworkspace.Ensurer) *Syncer {
	return &Syncer{
		Queries:   Queries,
		Graph:     Graph,
		Workspace: Workspace,
	}
}

func (s *Syncer) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// RunSwitchOrLogout lists remaining workspaces for the user and either sets the first as current
// and calls OnAfterWorkspaceSwitch, or calls OnLogout if none remain.
// Returns true if the user was logged out.
func (s *Syncer) RunSwitchOrLogout(ctx context.Context, userID string) bool {
	return s.runSwitchOrLogout(ctx, userID)
}
