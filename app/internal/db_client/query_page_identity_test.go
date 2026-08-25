package db_client

import (
	"strings"
	"testing"

	"github.com/selectDb/dialect/engine"
)

// Regression: the result cache is keyed by (db, file), and GetResultPage used
// to ignore the ResultID it was handed. A second run of the same file took over
// the slot, and a page request for the first execution was answered with the
// second's rows — which the frontend then rendered under the first execution's
// column headers, putting every value one column out.
func TestGetResultPageRefusesASupersededExecution(t *testing.T) {
	const dbID, fileID = "db-1", "file-1"
	key := queryKey(dbID, fileID)
	t.Cleanup(func() { engine.DeleteResult(key) })

	// First run: SELECT * — the columns the frontend is showing.
	first := engine.NewStreamingResult("exec-first")
	first.SetColumns([]string{"id", "workspace_id", "occurred_at", "recorded_at"})
	first.AppendRow([]any{"id-1", "ws-1", "t0", "t1"})
	first.Finalize(1, 0, 1)
	engine.SetStreamingResult(key, first)

	// Second run of the same file, one column narrower, takes over the slot.
	second := engine.NewStreamingResult("exec-second")
	second.SetColumns([]string{"workspace_id", "occurred_at", "recorded_at"})
	second.AppendRow([]any{"ws-1", "t0", "t1"})
	second.Finalize(1, 0, 1)
	engine.SetStreamingResult(key, second)

	dbc := &DbClient{}

	stale := dbc.GetResultPage(GetResultPageParams{
		DbInstanceID: dbID,
		FileID:       fileID,
		ResultID:     "exec-first",
		Page:         0,
	})

	if len(stale.Rows) > 0 {
		t.Errorf("a superseded execution must not be handed another run's rows, got %v", stale.Rows)
	}
	if len(stale.Errors) == 0 || !strings.Contains(stale.Errors[0], "re-run") {
		t.Errorf("the caller should be told to re-run, got errors %v", stale.Errors)
	}

	// The execution that owns the slot still reads normally.
	current := dbc.GetResultPage(GetResultPageParams{
		DbInstanceID: dbID,
		FileID:       fileID,
		ResultID:     "exec-second",
		Page:         0,
	})

	if len(current.Errors) != 0 {
		t.Fatalf("the current execution should read cleanly, got %v", current.Errors)
	}
	if len(current.Rows) != 1 || len(current.Columns) != 3 {
		t.Fatalf("expected 1 row of 3 columns, got %d rows / %d columns", len(current.Rows), len(current.Columns))
	}
	if len(current.Rows[0]) != len(current.Columns) {
		t.Errorf("row width must match the column count: %d values, %d columns",
			len(current.Rows[0]), len(current.Columns))
	}
}
