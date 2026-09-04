package engine

import (
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/selectDb/dialect/core"
	_ "modernc.org/sqlite"
)

// countingDialect wraps a real dialect and counts opens, so a test can assert
// how many pools were actually dialed rather than how many survived.
type countingDialect struct {
	core.SQLDialect
	opens atomic.Int64
	delay time.Duration
}

func (d *countingDialect) OpenDB(dsn string) (*sql.DB, error) {
	d.opens.Add(1)
	time.Sleep(d.delay)
	return d.SQLDialect.OpenDB(dsn)
}

func openTestPool(t *testing.T, name string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("create table if not exists t(x int)"); err != nil {
		t.Fatal(err)
	}
	return db
}

// getConn and setConn reach the connection cache the way GetOrOpenConn does,
// without opening anything. Production goes through connCache.GetOrCreate, so
// these live here rather than in the package: the tests need to place a pool
// under a key and read it back to exercise the deletion paths.
func getConn(workspaceID, dsn string) (*sql.DB, bool) {
	value, ok := connCache.Get(hashWorkspaceDSN(workspaceID, dsn))
	if !ok {
		return nil, false
	}
	return value.(*sql.DB), true
}

// setConn writes the index after the cache, because replacing an entry fires
// closeDeletedPool for the old value and that clears the index for this hash.
func setConn(workspaceID, dsn string, db *sql.DB) {
	hash := hashWorkspaceDSN(workspaceID, dsn)
	connCache.Set(hash, db)
	indexConn(hash, dsn)
}

func poolIsClosed(db *sql.DB) bool {
	_, err := db.Exec("select 1")
	return err == sql.ErrConnDone || (err != nil && err.Error() == "sql: database is closed")
}

// TestDeletedPoolIsClosed is the leak regression: a pool dropped from the cache
// must actually be closed, not just dereferenced. An unclosed *sql.DB is rooted
// by its own connectionOpener goroutine, so it is never collected and its
// sockets to the customer's database stay open for the life of the process.
func TestDeletedPoolIsClosed(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = 10 * time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	db := openTestPool(t, "deleted")
	setConn("ws1", "dsn-deleted", db)

	if poolIsClosed(db) {
		t.Fatal("pool closed while still cached")
	}

	connCache.Delete(hashWorkspaceDSN("ws1", "dsn-deleted"))

	deadline := time.Now().Add(2 * time.Second)
	for !poolIsClosed(db) {
		if time.Now().After(deadline) {
			t.Fatal("deleted pool was never closed: this is the leak")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestDeletionClearsDSNIndex: the hash → DSN index is only pruned by
// DeleteConnsByAddr today, so TTL and LRU deletions grew it without bound.
func TestDeletionClearsDSNIndex(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = 10 * time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	db := openTestPool(t, "indexed")
	setConn("ws1", "dsn-indexed", db)
	hash := hashWorkspaceDSN("ws1", "dsn-indexed")

	connHashToDSNMu.Lock()
	_, present := connHashToDSN[hash]
	connHashToDSNMu.Unlock()
	if !present {
		t.Fatal("index entry missing while cached")
	}

	connCache.Delete(hash)

	connHashToDSNMu.Lock()
	_, stillThere := connHashToDSN[hash]
	connHashToDSNMu.Unlock()
	if stillThere {
		t.Fatal("index entry survived deletion")
	}
}

// TestReplacingAPoolClosesTheOldOne: storing a second pool under a live key
// must close the one it displaces rather than drop the reference. GetOrOpenConn
// no longer produces that case itself — see TestConcurrentFirstQueriesOpenOnePool
// — but Set is public and a redial after a tunnel drop lands here.
func TestReplacingAPoolClosesTheOldOne(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = 10 * time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	first := openTestPool(t, "race-a")
	second := openTestPool(t, "race-b")

	setConn("ws1", "dsn-race", first)
	setConn("ws1", "dsn-race", second)

	deadline := time.Now().Add(2 * time.Second)
	for !poolIsClosed(first) {
		if time.Now().After(deadline) {
			t.Fatal("the replaced pool was never closed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if poolIsClosed(second) {
		t.Fatal("the live pool was closed")
	}
}

// TestGracePeriodProtectsAnInFlightCaller: a request that took the pool out of
// the cache just before deletion must still be able to start its query.
func TestGracePeriodProtectsAnInFlightCaller(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = 750 * time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	db := openTestPool(t, "inflight")
	setConn("ws1", "dsn-inflight", db)

	handed, _ := getConn("ws1", "dsn-inflight")
	connCache.Delete(hashWorkspaceDSN("ws1", "dsn-inflight"))

	// The caller starts its query after the deletion, inside the grace window.
	time.Sleep(100 * time.Millisecond)
	if _, err := handed.Exec("select 1"); err != nil {
		t.Fatalf("in-flight caller was cut off by deletion: %v", err)
	}
}

// TestNoGoroutineLeakAcrossDeletions is the capacity claim: churning pools
// through the cache must not accumulate goroutines.
func TestNoGoroutineLeakAcrossDeletions(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	const pools = 150
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	before := runtime.NumGoroutine()

	for i := 0; i < pools; i++ {
		db := openTestPool(t, fmt.Sprintf("churn%d", i))
		key := fmt.Sprintf("dsn-churn%d", i)
		setConn("ws1", key, db)
		connCache.Delete(hashWorkspaceDSN("ws1", key))
	}

	deadline := time.Now().Add(10 * time.Second)
	var after int
	for {
		runtime.GC()
		time.Sleep(150 * time.Millisecond)
		after = runtime.NumGoroutine()
		if after-before < pools/4 || time.Now().After(deadline) {
			break
		}
	}
	t.Logf("goroutines %d -> %d after churning %d pools", before, after, pools)
	if after-before >= pools/4 {
		t.Fatalf("pools leaked: %d goroutines retained for %d deleted pools", after-before, pools)
	}
}

func TestConcurrentDeletionIsSafe(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				key := fmt.Sprintf("dsn-%d-%d", i, j)
				db := openTestPool(t, fmt.Sprintf("conc%d_%d", i, j))
				setConn("ws1", key, db)
				getConn("ws1", key)
				connCache.Delete(hashWorkspaceDSN("ws1", key))
			}
		}(i)
	}
	wg.Wait()
}

// TestConcurrentFirstQueriesOpenOnePool is the singleflight claim: callers that
// miss together share one open instead of each dialing and discarding the
// losers. Before, N concurrent first queries meant N dials — N round trips
// against the customer's database — with N-1 pools closed straight after.
func TestConcurrentFirstQueriesOpenOnePool(t *testing.T) {
	restoreGuard := EnforceOutboundGuard
	restoreGrace := poolCloseGrace
	EnforceOutboundGuard = false
	poolCloseGrace = 10 * time.Millisecond
	defer func() {
		EnforceOutboundGuard = restoreGuard
		poolCloseGrace = restoreGrace
		ClearConnCache()
	}()
	ClearConnCache()

	base := GetDialect("sqlite")
	if base == nil {
		t.Fatal("sqlite dialect not available")
	}
	// Slow enough that every goroutine is inside GetOrOpenConn before the first
	// open finishes; without singleflight they all dial.
	counting := &countingDialect{SQLDialect: base, delay: 50 * time.Millisecond}
	RegisterDialect("sqlite-counting", counting)

	const n = 32
	dsn := "file:concurrent-open?mode=memory&cache=shared"
	pools := make([]*sql.DB, n)
	errs := make([]error, n)

	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(n)
	for i := range n {
		go func() {
			defer done.Done()
			start.Wait()
			pools[i], errs[i] = GetOrOpenConn("ws1", "sqlite-counting", dsn, nil)
		}()
	}
	start.Done()
	done.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		if pools[i] != pools[0] {
			t.Fatalf("caller %d got a different pool; all callers must share one", i)
		}
	}
	if opens := counting.opens.Load(); opens != 1 {
		t.Fatalf("opened %d pools for one datasource, want 1", opens)
	}

	// The pool that was opened is the one in the cache, and it is indexed for
	// DeleteConnsByAddr.
	cached, ok := getConn("ws1", dsn)
	if !ok || cached != pools[0] {
		t.Fatal("the shared pool is not the one left in the cache")
	}
	connHashToDSNMu.Lock()
	_, indexed := connHashToDSN[hashWorkspaceDSN("ws1", dsn)]
	connHashToDSNMu.Unlock()
	if !indexed {
		t.Fatal("the opened pool was not added to the hash → DSN index")
	}
}

// TestFailedOpenIsNotCached: a dial that fails must leave nothing behind, so the
// next query retries rather than inheriting the failure.
func TestFailedOpenIsNotCached(t *testing.T) {
	restoreGuard := EnforceOutboundGuard
	EnforceOutboundGuard = false
	defer func() { EnforceOutboundGuard = restoreGuard; ClearConnCache() }()
	ClearConnCache()

	if _, err := GetOrOpenConn("ws1", "no-such-dialect", "dsn-unopenable", nil); err == nil {
		t.Fatal("expected an error for an unsupported database type")
	}
	if _, ok := getConn("ws1", "dsn-unopenable"); ok {
		t.Fatal("a failed open was cached")
	}
}
