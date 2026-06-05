package system

import (
	"context"
	"sync"

	"selectDb/backend/db/generated"
	"selectDb/backend/db_client"
	"selectDb/backend/fs_provider"
	"selectDb/backend/graph"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
	wailsruntime.Quit(s.ctx)
}

func (s *System) WindowMinimise() {
	wailsruntime.WindowMinimise(s.ctx)
}

func (s *System) WindowToggleFullscreen() {
	wailsruntime.WindowToggleMaximise(s.ctx)

}
