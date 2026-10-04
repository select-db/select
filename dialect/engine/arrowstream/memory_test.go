package arrowstream

import (
	"io"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
)

// peakHeap runs fn and returns the most live heap it reached above where it
// started, sampled every millisecond.
func peakHeap(fn func()) uint64 {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var (
		mu   sync.Mutex
		peak uint64
		stop = make(chan struct{})
		done = make(chan struct{})
	)
	go func() {
		defer close(done)
		var stats runtime.MemStats
		for {
			runtime.ReadMemStats(&stats)
			mu.Lock()
			if stats.HeapAlloc > peak {
				peak = stats.HeapAlloc
			}
			mu.Unlock()
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
	fn()
	close(stop)
	<-done
	if peak < base.HeapAlloc {
		return 0
	}
	return peak - base.HeapAlloc
}

// A batch is bounded in bytes, not only in rows: 500 rows of 1 MiB would
// otherwise sit in the builders, then in the IPC buffer, then in the encoder,
// before the first byte leaves.
func TestSinkMemoryIsBoundedByBatchBytes(t *testing.T) {
	debug.SetGCPercent(50)
	t.Cleanup(func() { debug.SetGCPercent(100) })
	const rows, cell = 300, 1 << 20
	value := strings.Repeat("x", cell)

	peak := peakHeap(func() {
		sink := NewSink(io.Discard)
		defer sink.Close()
		_ = sink.OnColumns([]string{"b"})
		for range rows {
			if err := sink.OnRow([]any{value}); err != nil {
				t.Fatal(err)
			}
		}
		_ = sink.OnDone(rows, 0, 1)
	})

	t.Logf("%d rows of %d MiB (%d MiB in all): peak live heap %d MiB", rows, cell>>20, rows*cell>>20, peak>>20)
	// A quarter of the stream: a runner's collector paces differently from a
	// laptop's, and the unbounded sink peaked at three times the stream.
	if limit := uint64(rows * cell / 4); peak > limit {
		t.Fatalf("peak heap %d MiB for a stream of %d MiB: want under %d MiB", peak>>20, rows*cell>>20, limit>>20)
	}
}

func BenchmarkSinkNarrowRows(b *testing.B) {
	row := []any{int64(7), "a string of about forty characters, row", 3.5}
	b.ReportAllocs()
	for range b.N {
		sink := NewSink(io.Discard)
		_ = sink.OnColumns([]string{"id", "name", "score"})
		for range 10000 {
			_ = sink.OnRow(row)
		}
		_ = sink.OnDone(10000, 0, 1)
		sink.Close()
	}
}
