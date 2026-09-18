package engine

import "testing"

// The result cache is keyed by (db, file), so re-running a file replaces the
// entry. Readers hold the execution id they started with, and every page has
// to say which execution it came from — otherwise a caller is handed the newer
// rows while still showing the earlier columns.
func TestPageCarriesOwningExecutionID(t *testing.T) {
	first := NewStreamingResult("exec-first")
	first.SetColumns([]string{"id", "workspace_id", "occurred_at"})
	first.AppendRow([]any{"row-from-first", "ws", "t0"})
	first.Finalize(1, 0, 1)

	page, _ := first.Page(0, 75)
	if page.ID != "exec-first" {
		t.Fatalf("page should name the execution that produced it, got %q", page.ID)
	}

	// A re-run of the same file takes over the same cache slot.
	const key = "db:d:file:f"
	SetStreamingResult(key, first)
	second := NewStreamingResult("exec-second")
	second.SetColumns([]string{"workspace_id", "occurred_at"})
	second.AppendRow([]any{"row-from-second", "t0"})
	second.Finalize(1, 0, 1)
	SetStreamingResult(key, second)

	t.Cleanup(func() { DeleteResult(key) })

	cached, ok := GetStreamingResult(key)
	if !ok {
		t.Fatal("expected the slot to hold a result")
	}
	got, _ := cached.Page(0, 75)
	if got.ID != "exec-second" {
		t.Fatalf("the slot holds the newer execution, so its page must say so, got %q", got.ID)
	}
	if got.ID == "exec-first" {
		t.Fatal("a caller holding exec-first would have been handed exec-second's rows")
	}
}
