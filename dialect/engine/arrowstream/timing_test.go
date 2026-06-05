package arrowstream

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestStreamingTailLatencyOverHTTP times row-by-row arrival through a real
// local HTTP server using the same wire setup as the production handler.
//
// Run with: go test ./engine/arrowstream/ -run TestStreamingTailLatencyOverHTTP -v
func TestStreamingTailLatencyOverHTTP(t *testing.T) {
	const totalRows = 5000

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
		w.Header().Set("Content-Encoding", "zstd")
		w.WriteHeader(http.StatusOK)
		sink := NewSink(w)
		defer sink.Close()
		if flusher, ok := w.(http.Flusher); ok {
			sink.SetDownstreamFlusher(flusher.Flush)
		}
		_ = sink.OnColumns([]string{"id", "name"})
		sink.OnExecuted(7)
		for i := 0; i < totalRows; i++ {
			_ = sink.OnRow([]any{int64(i), fmt.Sprintf("name-%d", i)})
		}
		_ = sink.OnDone(int64(totalRows), 0, 7)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := NewStream(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	clientStart := time.Now()
	if _, err := stream.Columns(); err != nil {
		t.Fatal(err)
	}
	t.Logf("Columns at +%s, executed=%d ms", time.Since(clientStart), stream.Executed())

	rowTimes := make([]time.Duration, 0, totalRows)
	for {
		_, ok, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		rowTimes = append(rowTimes, time.Since(clientStart))
	}
	doneAt := time.Since(clientStart)
	if _, _, _, err := stream.Summary(); err != nil {
		t.Fatal(err)
	}
	summaryAt := time.Since(clientStart)

	t.Logf("rows=%d, last_row_at=%s, summary_at=%s, done-summary=%s",
		len(rowTimes), doneAt, summaryAt, summaryAt-doneAt)
	for _, idx := range []int{0, totalRows / 2, totalRows - 250, totalRows - 100, totalRows - 1} {
		if idx >= 0 && idx < len(rowTimes) {
			t.Logf("  row %5d at +%s", idx+1, rowTimes[idx])
		}
	}
	if len(rowTimes) >= 251 {
		t.Logf("  last 250 rows arrived in %s", rowTimes[len(rowTimes)-1]-rowTimes[len(rowTimes)-251])
	}
}

// TestStreamingTailWithSlowSource simulates a source DB that delivers rows
// quickly for the bulk and then stalls briefly before delivering the tail.
// Most DB drivers exhibit this when fetching the last server-side cursor page.
func TestStreamingTailWithSlowSource(t *testing.T) {
	if testing.Short() {
		t.Skip("slow-source simulation; skipped under -short")
	}
	const totalRows = 50000
	const stallAfter = totalRows - 250
	const stallDuration = 1500 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
		w.Header().Set("Content-Encoding", "zstd")
		w.WriteHeader(http.StatusOK)
		sink := NewSink(w)
		defer sink.Close()
		if flusher, ok := w.(http.Flusher); ok {
			sink.SetDownstreamFlusher(flusher.Flush)
		}
		_ = sink.OnColumns([]string{"id", "name"})
		sink.OnExecuted(5)

		for i := 0; i < totalRows; i++ {
			if i == stallAfter {
				time.Sleep(stallDuration)
			}
			_ = sink.OnRow([]any{int64(i), fmt.Sprintf("name-%d", i)})
		}
		_ = sink.OnDone(int64(totalRows), 0, 5)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := NewStream(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	clientStart := time.Now()
	if _, err := stream.Columns(); err != nil {
		t.Fatal(err)
	}

	rowTimes := make([]time.Duration, 0, totalRows)
	for {
		_, ok, err := stream.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		rowTimes = append(rowTimes, time.Since(clientStart))
	}
	if len(rowTimes) != totalRows {
		t.Fatalf("rows: want %d got %d", totalRows, len(rowTimes))
	}

	bulkGap := rowTimes[stallAfter-1] - rowTimes[0]
	tailGap := rowTimes[totalRows-1] - rowTimes[stallAfter-1]
	t.Logf("Bulk %d rows in %s; tail %d rows in %s (server-side stall=%s)",
		stallAfter, bulkGap, totalRows-stallAfter, tailGap, stallDuration)
	t.Logf("First row at +%s, stall transition at +%s, last row at +%s",
		rowTimes[0], rowTimes[stallAfter-1], rowTimes[totalRows-1])
}
