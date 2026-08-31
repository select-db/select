package engine

import (
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

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

func poolIsClosed(db *sql.DB) bool {
	_, err := db.Exec("select 1")
	return err == sql.ErrConnDone || (err != nil && err.Error() == "sql: database is closed")
}

// TestEvictedPoolIsClosed is the leak regression: a pool dropped from the cache
// must actually be closed, not just dereferenced. An unclosed *sql.DB is rooted
// by its own connectionOpener goroutine, so it is never collected and its
// sockets to the customer's database stay open for the life of the process.
func TestEvictedPoolIsClosed(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = 10 * time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	db := openTestPool(t, "evicted")
	setConn("ws1", "dsn-evicted", db)

	if poolIsClosed(db) {
		t.Fatal("pool closed while still cached")
	}

	connCache.Delete(hashWorkspaceDSN("ws1", "dsn-evicted"))

	deadline := time.Now().Add(2 * time.Second)
	for !poolIsClosed(db) {
		if time.Now().After(deadline) {
			t.Fatal("evicted pool was never closed: this is the leak")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestEvictionClearsDSNIndex: the hash → DSN index is only pruned by
// EvictConnsByAddr today, so TTL and LRU evictions grew it without bound.
func TestEvictionClearsDSNIndex(t *testing.T) {
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
		t.Fatal("index entry survived eviction")
	}
}

// TestReplacingAPoolClosesTheOldOne covers the double-open race: two first
// queries for one datasource both dial, and the loser must be closed rather
// than overwritten in the cache.
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
// the cache just before eviction must still be able to start its query.
func TestGracePeriodProtectsAnInFlightCaller(t *testing.T) {
	restore := poolCloseGrace
	poolCloseGrace = 750 * time.Millisecond
	defer func() { poolCloseGrace = restore; ClearConnCache() }()
	ClearConnCache()

	db := openTestPool(t, "inflight")
	setConn("ws1", "dsn-inflight", db)

	handed, _ := getConn("ws1", "dsn-inflight")
	connCache.Delete(hashWorkspaceDSN("ws1", "dsn-inflight"))

	// The caller starts its query after the eviction, inside the grace window.
	time.Sleep(100 * time.Millisecond)
	if _, err := handed.Exec("select 1"); err != nil {
		t.Fatalf("in-flight caller was cut off by eviction: %v", err)
	}
}

// TestNoGoroutineLeakAcrossEvictions is the capacity claim: churning pools
// through the cache must not accumulate goroutines.
func TestNoGoroutineLeakAcrossEvictions(t *testing.T) {
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
		t.Fatalf("pools leaked: %d goroutines retained for %d evicted pools", after-before, pools)
	}
}

func TestConcurrentEvictionIsSafe(t *testing.T) {
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
