package graph

import (
	"context"
	"os"
	"selectDb/internal/db/generated"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	originalEmit := eventsEmit
	originalDebounced := debouncedEventsEmit
	eventsEmit = func(ctx context.Context, eventName string, optionalData ...interface{}) {
		// no-op
	}
	debouncedEventsEmit = func(ctx context.Context, eventName string, timeout time.Duration, data ...interface{}) {
		// no-op: avoid scheduling a real wails emit, whose timer would fire
		// after the test returned and crash with an invalid context.
	}
	code := m.Run()
	eventsEmit = originalEmit
	debouncedEventsEmit = originalDebounced
	os.Exit(code)
}

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
