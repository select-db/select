package cellar

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// discardResponse is a response writer that keeps nothing, so a test measures
// the cellar and not the recorder behind it.
type discardResponse struct {
	header http.Header
	bytes  int
}

func (d *discardResponse) Header() http.Header         { return d.header }
func (d *discardResponse) WriteHeader(int)             {}
func (d *discardResponse) Flush()                      {}
func (d *discardResponse) Write(p []byte) (int, error) { d.bytes += len(p); return len(p), nil }

// peakHeap runs fn and returns the most live heap it reached above its start.
func peakHeap(fn func()) uint64 {
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var (
		mu         sync.Mutex
		peak       uint64
		stop, done = make(chan struct{}), make(chan struct{})
	)
	go func() {
		defer close(done)
		var stats runtime.MemStats
		for {
			runtime.ReadMemStats(&stats)
			mu.Lock()
			peak = max(peak, stats.HeapAlloc)
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
	return peak - min(peak, base.HeapAlloc)
}

func streamQuery(t *testing.T, id, statement string) *discardResponse {
	t.Helper()
	body, err := json.Marshal(Query{SQL: statement})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/datasources/"+id+"/query", bytes.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), grantKey{}, Grant{DatasourceID: id, WorkspaceID: uuid.NewString(), CellarID: "local", MaxBytes: 1 << 20}))
	w := &discardResponse{header: http.Header{}}
	QueryHandler().ServeHTTP(w, r)
	return w
}

// A result streams in memory bounded by its batches, not its size: 300 rows of
// 1 MiB reached 900 MiB of live heap when a batch was 500 rows whatever their width.
func TestWideRowsStreamInBoundedMemory(t *testing.T) {
	debug.SetGCPercent(50)
	t.Cleanup(func() { debug.SetGCPercent(100) })
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	const rows, cell = 300, 1 << 20

	var response *discardResponse
	peak := peakHeap(func() {
		response = streamQuery(t, id, "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c LIMIT 300) SELECT zeroblob(1048576) AS payload FROM c")
	})

	t.Logf("%d rows of %d MiB: peak live heap %d MiB, %d KiB on the wire", rows, cell>>20, peak>>20, response.bytes>>10)
	require.Less(t, peak, uint64(96<<20), "the heap must not follow the size of the result")
	require.Positive(t, response.bytes)
}

func TestAValueOverTheLimitIsRefusedBeforeItIsCopied(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)

	response := streamQuery(t, id, "SELECT length(zeroblob(100000000))")
	require.Positive(t, response.bytes)
	wide := streamQuery(t, id, "SELECT zeroblob(20000000) AS payload")
	require.Positive(t, wide.bytes)

	peak := peakHeap(func() { streamQuery(t, id, "SELECT zeroblob(20000000) AS payload") })
	require.Less(t, peak, uint64(48<<20), "a refused value is not held several times over")
}
