package system

import (
	"context"
	"sync"

	"selectDb/internal/db/generated"
	"selectDb/internal/db_client"
	"selectDb/internal/fs_provider"
	"selectDb/internal/graph"

	"selectDb/internal/desktop"
)

type System struct {
	ctx               context.Context
	mu                sync.Mutex
	fileWatcherCancel context.CancelFunc
	dbWatcherCancel   context.CancelFunc
	Queries           *generated.Queries
	Graph             *graph.Graph
	DbClient          *db_client.DbClient
	FSProvider        *fs_provider.FSProvider
	// emitHook is an optional test hook that, when set, is called instead of
	// invoking Graph.Mutate from emitMutation. It is nil in production.
	emitHook func(generated.MutationCommit)
}

func New(Queries *generated.Queries, Graph *graph.Graph, DbClient *db_client.DbClient, FSProvider *fs_provider.FSProvider) *System {
	return &System{
		Queries:    Queries,
		Graph:      Graph,
		DbClient:   DbClient,
		FSProvider: FSProvider,
	}
}

func (s *System) SetContext(ctx context.Context) {
	s.ctx = ctx
}

func (s *System) WindowClose() {
	desktop.Quit()
}

func (s *System) WindowMinimise() {
	desktop.MinimiseWindow()
}

func (s *System) WindowToggleFullscreen() {
	desktop.ToggleMaximiseWindow()

}

// SetZoom applies a page zoom factor to the window and returns the factor that
// was actually applied, which the frontend uses to keep its zoom level in step
// with the webview (Windows cannot go below 100%).
func (s *System) SetZoom(factor float64) (float64, error) {
	return desktop.SetZoom(factor)
}

// GetZoom returns the window's current page zoom factor.
func (s *System) GetZoom() float64 {
	return desktop.GetZoom()
}
