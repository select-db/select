package cellar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/selectDb/dialect/engine/membudget"
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

// serve runs one statement through the handler as the backend would.
func serve(t testing.TB, id, statement string, w http.ResponseWriter) {
	t.Helper()
	body, err := json.Marshal(Query{SQL: statement})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/datasources/"+id+"/query", bytes.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), grantKey{}, Grant{DatasourceID: id, WorkspaceID: uuid.NewString(), MaxBytes: 1 << 20}))
	QueryHandler().ServeHTTP(w, r)
}

func streamQuery(t *testing.T, id, statement string) *discardResponse {
	t.Helper()
	w := &discardResponse{header: http.Header{}}
	serve(t, id, statement, w)
	return w
}

func responseBytes(t testing.TB, id, statement string) []byte {
	t.Helper()
	w := httptest.NewRecorder()
	serve(t, id, statement, w)
	return w.Body.Bytes()
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

// useBudget swaps the cellar's memory budget for a test.
func useBudget(t *testing.T, total int64) {
	t.Helper()
	previous := memoryBudget
	memoryBudget = &membudget.Budget{Total: total}
	t.Cleanup(func() { memoryBudget = previous })
}

type outcome struct{ ok, refused, other int }

// burst runs n statements at once, each returning one value of valueBytes, and
// tells what became of them.
func burst(t *testing.T, id string, n, valueBytes int) (outcome, uint64) {
	t.Helper()
	statement := fmt.Sprintf("SELECT zeroblob(%d) AS payload", valueBytes)
	var (
		mu  sync.Mutex
		out outcome
		wg  sync.WaitGroup
	)
	peak := peakHeap(func() {
		for range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				decoded, err := zstdDecode(responseBytes(t, id, statement))
				require.NoError(t, err)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case bytes.Contains(decoded, []byte(CodeUnavailable)):
					out.refused++
				case bytes.Contains(decoded, []byte("executed_ms")):
					out.ok++
				default:
					out.other++
				}
			}()
		}
		wg.Wait()
	})
	return out, peak
}

func zstdDecode(b []byte) ([]byte, error) {
	reader, err := zstd.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// Twenty-four statements at once, each returning a 15 MB value: with room for
// all of them the heap follows their sum; with a budget the cellar refuses the
// ones it has no room for and the rest finish.
func TestMemoryBudgetRefusesWhatItHasNoRoomFor(t *testing.T) {
	debug.SetGCPercent(50)
	t.Cleanup(func() { debug.SetGCPercent(100) })
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	const n, value = 24, 15_000_000

	useBudget(t, 4<<30)
	free, freePeak := burst(t, id, n, value)
	t.Logf("no budget: %+v, peak live heap %d MiB", free, freePeak>>20)

	useBudget(t, 160<<20)
	limited, limitedPeak := burst(t, id, n, value)
	used, total, refused := memoryBudget.Stats()
	t.Logf("160 MiB budget: %+v, peak live heap %d MiB, budget used %d of %d MiB afterwards, %d refused", limited, limitedPeak>>20, used>>20, total>>20, refused)

	require.Equal(t, n, free.ok, "with room for all of them, all of them run")
	require.Positive(t, limited.refused, "a statement with no room is refused, not run")
	require.Positive(t, limited.ok, "the rest finish")
	require.Zero(t, limited.other, "every statement either finishes or is refused with the coded error")
	require.Zero(t, used, "every lease is released")
	// The driver's own copy of each value is made before a statement can ask for
	// room: n values and nothing else, where all of them running would hold several
	// copies each.
	require.Less(t, limitedPeak, uint64(2*n*value), "the heap stays within two copies of each value")
	require.LessOrEqual(t, limited.ok, 6, "only what the budget has room for runs")
}

// Wide statements fill the ceiling; a trivial one still starts.
func TestSmallStatementsStartWhileWideOnesHaveTheBudget(t *testing.T) {
	cellar := newTestCellar(t)
	id := uuid.NewString()
	cellar.createNotes(t, id)
	useBudget(t, 100<<20)
	wide, err := memoryBudget.Begin()
	require.NoError(t, err)
	t.Cleanup(wide.Release)
	require.NoError(t, wide.Grow(90<<20))

	decoded, err := zstdDecode(responseBytes(t, id, "SELECT 1"))
	require.NoError(t, err)
	require.Contains(t, string(decoded), "executed_ms")
	require.NotContains(t, string(decoded), CodeUnavailable)
}
