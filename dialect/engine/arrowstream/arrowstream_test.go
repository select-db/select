package arrowstream

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	var buf bytes.Buffer

	// Encode
	sink := NewSink(&buf)
	if err := sink.OnColumns([]string{"id", "name", "score"}); err != nil {
		t.Fatal(err)
	}
	rows := [][]any{
		{int64(1), "alice", 9.5},
		{int64(2), "bob", nil},
		{int64(3), "charlie", 7.2},
	}
	for _, row := range rows {
		if err := sink.OnRow(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := sink.OnDone(3, 0, 42); err != nil {
		t.Fatal(err)
	}
	sink.Close()

	// Decode
	stream, err := NewStream(io.NopCloser(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	cols, err := stream.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 || cols[0] != "id" || cols[1] != "name" || cols[2] != "score" {
		t.Fatalf("unexpected columns: %v", cols)
	}

	var decoded [][]any
	for {
		row, ok, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		decoded = append(decoded, row)
	}

	if len(decoded) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(decoded))
	}

	// Values come back as strings (Arrow stores all as string columns)
	if decoded[0][0] != "1" || decoded[0][1] != "alice" || decoded[0][2] != "9.5" {
		t.Fatalf("row 0: %v", decoded[0])
	}
	if decoded[1][2] != nil {
		t.Fatalf("row 1 col 2 should be nil, got %v", decoded[1][2])
	}

	rowCount, affected, durationMs, err := stream.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if rowCount != 3 || affected != 0 || durationMs != 42 {
		t.Fatalf("summary: rows=%d affected=%d ms=%d", rowCount, affected, durationMs)
	}
}

func TestRoundtripError(t *testing.T) {
	var buf bytes.Buffer

	sink := NewSink(&buf)
	if err := sink.OnColumns([]string{"id"}); err != nil {
		t.Fatal(err)
	}
	sink.OnRow([]any{"1"})
	sink.OnError(fmt.Errorf("syntax error at position 42"))
	sink.Close()

	stream, err := NewStream(io.NopCloser(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	_, err = stream.Columns()
	if err != nil {
		t.Fatal(err)
	}

	// Should get the one row then error
	row, ok, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || row[0] != "1" {
		t.Fatalf("expected first row, got ok=%v row=%v", ok, row)
	}

	_, ok, err = stream.Next()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "syntax error at position 42" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoundtripExecutedSegment(t *testing.T) {
	var buf bytes.Buffer

	sink := NewSink(&buf)
	sink.OnExecuted(42)
	if err := sink.OnColumns([]string{"id"}); err != nil {
		t.Fatal(err)
	}
	if err := sink.OnRow([]any{int64(1)}); err != nil {
		t.Fatal(err)
	}
	if err := sink.OnDone(1, 0, 50); err != nil {
		t.Fatal(err)
	}
	sink.Close()

	stream, err := NewStream(io.NopCloser(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	cols, err := stream.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 1 || cols[0] != "id" {
		t.Fatalf("cols: want [id], got %v", cols)
	}
	if got := stream.Executed(); got != 42 {
		t.Fatalf("Executed(): want 42, got %d", got)
	}

	row, ok, err := stream.Next()
	if err != nil || !ok || row[0] != "1" {
		t.Fatalf("first row: ok=%v err=%v row=%v", ok, err, row)
	}
	_, ok, err = stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no more rows")
	}
	rowCount, _, durationMs, err := stream.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 || durationMs != 50 {
		t.Fatalf("summary: rows=%d ms=%d", rowCount, durationMs)
	}
}

// TestSinkFlushesDownstreamPerBatch verifies that when a flusher is wired,
// each batch boundary produces bytes downstream, not held until Close.
func TestSinkFlushesDownstreamPerBatch(t *testing.T) {
	var buf bytes.Buffer
	flushes := 0
	sink := NewSink(&buf)
	sink.SetDownstreamFlusher(func() { flushes++ })

	if err := sink.OnColumns([]string{"id"}); err != nil {
		t.Fatal(err)
	}

	// One full batch (defaultBatchSize rows) → exactly one batch flush.
	for i := 0; i < defaultBatchSize; i++ {
		if err := sink.OnRow([]any{int64(i)}); err != nil {
			t.Fatal(err)
		}
	}

	if flushes != 1 {
		t.Fatalf("flushes after one full batch: want 1, got %d", flushes)
	}
	if buf.Len() == 0 {
		t.Fatal("expected bytes to be pushed downstream after a batch flush, got empty buffer")
	}
	bytesAfterBatch := buf.Len()

	// A trailing partial batch. The remaining rows shouldn't reach the
	// downstream until OnDone fires the final flushBatch.
	trailingRows := defaultBatchSize / 2
	for i := defaultBatchSize; i < defaultBatchSize+trailingRows; i++ {
		if err := sink.OnRow([]any{int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if buf.Len() != bytesAfterBatch {
		t.Fatalf("partial batch should not flush; bytes grew %d → %d", bytesAfterBatch, buf.Len())
	}

	totalRows := defaultBatchSize + trailingRows
	if err := sink.OnDone(int64(totalRows), 0, 1); err != nil {
		t.Fatal(err)
	}
	if flushes < 2 {
		t.Fatalf("expected an additional flush from OnDone's final batch, got %d total", flushes)
	}
	sink.Close()

	// Sanity: the resulting stream must still decode end-to-end.
	stream, err := NewStream(io.NopCloser(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Columns(); err != nil {
		t.Fatal(err)
	}
	rows := 0
	for {
		_, ok, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		rows++
	}
	if rows != totalRows {
		t.Fatalf("rows: want %d, got %d", totalRows, rows)
	}
}

func TestRoundtripErrorBeforeRows(t *testing.T) {
	var buf bytes.Buffer

	sink := NewSink(&buf)
	sink.OnError(fmt.Errorf("permission denied"))
	sink.Close()

	stream, err := NewStream(io.NopCloser(&buf))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	_, err = stream.Columns()
	if err == nil {
		t.Fatal("expected error from Columns()")
	}
	if err.Error() != "permission denied" {
		t.Fatalf("unexpected error: %v", err)
	}
}
