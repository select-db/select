package engine

import "testing"

func TestStreamingResultPagePending(t *testing.T) {
	sr := NewStreamingResult("test-exec")
	sr.SetColumns([]string{"id", "name"})

	page, status := sr.Page(0, 10)
	if status != PagePending {
		t.Fatalf("status: want pending, got %v", status)
	}
	if page == nil || len(page.Rows) != 0 {
		t.Fatalf("expected empty rows, got %v", page)
	}
	if len(page.Columns) != 2 {
		t.Fatalf("expected columns to be carried even when pending, got %v", page.Columns)
	}
}

func TestStreamingResultPagePartial(t *testing.T) {
	sr := NewStreamingResult("test-exec")
	sr.SetColumns([]string{"id"})

	for i := 0; i < 5; i++ {
		sr.AppendRow([]any{i})
	}

	page, status := sr.Page(0, 10)
	if status != PagePartial {
		t.Fatalf("status: want partial, got %v", status)
	}
	if len(page.Rows) != 5 {
		t.Fatalf("expected 5 rows in partial page, got %d", len(page.Rows))
	}
	if page.Available != 5 {
		t.Fatalf("expected available=5, got %d", page.Available)
	}
}

func TestStreamingResultPageReady(t *testing.T) {
	sr := NewStreamingResult("test-exec")
	sr.SetColumns([]string{"id"})

	for i := 0; i < 25; i++ {
		sr.AppendRow([]any{i})
	}
	sr.Finalize(25, 0, 7)

	page, status := sr.Page(0, 10)
	if status != PageReady {
		t.Fatalf("page 0 status: want ready, got %v", status)
	}
	if len(page.Rows) != 10 {
		t.Fatalf("page 0: want 10 rows, got %d", len(page.Rows))
	}

	page, status = sr.Page(2, 10)
	if status != PageReady {
		t.Fatalf("page 2 status: want ready (final 5), got %v", status)
	}
	if len(page.Rows) != 5 {
		t.Fatalf("page 2: want 5 rows, got %d", len(page.Rows))
	}
	if page.RowCount != 25 {
		t.Fatalf("page 2 rowCount: want 25, got %d", page.RowCount)
	}

	// Past the end of a finished stream
	page, status = sr.Page(3, 10)
	if status != PageReady {
		t.Fatalf("page past end status: want ready, got %v", status)
	}
	if len(page.Rows) != 0 {
		t.Fatalf("page past end: want 0 rows, got %d", len(page.Rows))
	}
}

func TestStreamingResultFailCarriesError(t *testing.T) {
	sr := NewStreamingResult("test-exec")
	sr.SetColumns([]string{"id"})
	sr.AppendRow([]any{1})
	sr.Fail("boom", nil)

	page, status := sr.Page(0, 10)
	if status != PageReady {
		t.Fatalf("status after Fail: want ready (terminal), got %v", status)
	}
	if len(page.Errors) != 1 || page.Errors[0] != "boom" {
		t.Fatalf("expected errors=[boom], got %v", page.Errors)
	}
	if len(page.Rows) != 1 {
		t.Fatalf("partial rows before fail should be returned, got %d", len(page.Rows))
	}
}
