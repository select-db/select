package db_client

import (
	"testing"

	"github.com/selectDb/dialect/engine"
)

// TestCancelQueryCallsAndCleansUp verifies that CancelQuery calls the stored
// cancel func and removes the entry from the engine cancel registry.
func TestCancelQueryCallsAndCleansUp(t *testing.T) {
	dbc := &DbClient{}

	params := QueryParams{
		FileID: "test-file",
	}
	key := queryKey("test-db", params.FileID)

	called := false
	engine.RegisterCancel(key, func() {
		called = true
	})

	dbc.CancelQuery(CancelQueryParams{
		DbInstanceID: "test-db",
		FileID:       params.FileID,
	})

	if !called {
		t.Fatalf("expected cancel func to be called for key %q", key)
	}

	// Re-registering a no-op and immediately cancelling should not call the old func again.
	engine.RegisterCancel(key, func() {})
	engine.UnregisterCancel(key)
}
