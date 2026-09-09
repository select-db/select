package syncer

import (
	"context"
	"sync"
	"time"

	"selectDb/internal/db/generated"
	"selectDb/internal/graph"
)

const syncDebounceDelay = 500 * time.Millisecond

// CurrentWorkspaceGoneHandler runs when a server delete takes away the open
// workspace, or this user's membership of it. There is nothing to switch to:
// picking a different folder is the user's call.
type CurrentWorkspaceGoneHandler interface {
	OnCurrentWorkspaceGone()
}

type Syncer struct {
	ctx     context.Context
	Queries *generated.Queries
	Graph   *graph.Graph

	// Optional; set by app wiring.
	CurrentWorkspaceGone CurrentWorkspaceGoneHandler

	// EmitRolesUpdated is optional. When set, called after sync applies role/user_to_role/permission changes.
	EmitRolesUpdated func()

	// FetchFunc overrides api.Fetch (UT).
	FetchFunc func(ctx context.Context, method, endpoint string, payload interface{}, headers map[string]string, response interface{}) error

	debounceMu     sync.Mutex
	debounceTimer  *time.Timer
	debounceUserID string
}

func New(Queries *generated.Queries, Graph *graph.Graph) *Syncer {
	return &Syncer{
		Queries: Queries,
		Graph:   Graph,
	}
}

func (s *Syncer) SetContext(ctx context.Context) {
	s.ctx = ctx
}
