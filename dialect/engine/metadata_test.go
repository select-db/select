package engine

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/sqlite"
	sqlitedriver "modernc.org/sqlite"
)

func init() {
	// Register the pure-Go SQLite driver under the name our dialect expects.
	sql.Register("sqlite3", &sqlitedriver.Driver{})
}

func TestGetOrFetchMetadata_HitAndMiss(t *testing.T) {
	t.Cleanup(ClearMetadataCache)
	ClearMetadataCache()

	dialect := sqlite.NewDialect()
	db, err := dialect.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec("CREATE TABLE t1 (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	ctx := context.Background()
	const ws = "ws-1"
	const dsn = "dsn-1"

	m1, err := GetOrFetchMetadata(ctx, ws, dsn, db, dialect, "test_db", false)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	m2, err := GetOrFetchMetadata(ctx, ws, dsn, db, dialect, "test_db", false)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if m1 != m2 {
		t.Fatal("expected same cached *Metadata pointer on hit")
	}

	// Different DSN, same workspace → different entry.
	m3, err := GetOrFetchMetadata(ctx, ws, "dsn-2", db, dialect, "test_db", false)
	if err != nil {
		t.Fatalf("third call: %v", err)
	}
	if m1 == m3 {
		t.Fatal("expected separate entries for different DSNs")
	}

	// Same DSN, different workspace → different entry. This is the
	// multi-tenant isolation guarantee: two users in two workspaces
	// pointing at the same DB never share a cache row.
	m4, err := GetOrFetchMetadata(ctx, "ws-2", dsn, db, dialect, "test_db", false)
	if err != nil {
		t.Fatalf("fourth call: %v", err)
	}
	if m1 == m4 {
		t.Fatal("expected separate entries when workspace differs")
	}
}

func TestGetOrFetchMetadata_RefreshForcesFresh(t *testing.T) {
	t.Cleanup(ClearMetadataCache)
	ClearMetadataCache()

	dialect := sqlite.NewDialect()
	db, err := dialect.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()

	m1, err := GetOrFetchMetadata(ctx, "ws", "dsn", db, dialect, "test_db", false)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := GetOrFetchMetadata(ctx, "ws", "dsn", db, dialect, "test_db", true)
	if err != nil {
		t.Fatal(err)
	}
	if m1 == m2 {
		t.Fatal("expected refresh=true to bypass cache")
	}
	// And the new entry replaced the old one.
	m3, err := GetOrFetchMetadata(ctx, "ws", "dsn", db, dialect, "test_db", false)
	if err != nil {
		t.Fatal(err)
	}
	if m3 != m2 {
		t.Fatal("expected the refreshed entry to be cached")
	}
}

func TestInvalidateMetadata(t *testing.T) {
	t.Cleanup(ClearMetadataCache)
	ClearMetadataCache()

	dialect := sqlite.NewDialect()
	db, err := dialect.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()

	m1a, _ := GetOrFetchMetadata(ctx, "ws", "dsn-A", db, dialect, "db", false)
	m2a, _ := GetOrFetchMetadata(ctx, "ws", "dsn-B", db, dialect, "db", false)

	InvalidateMetadata("ws", "dsn-A")

	m1b, _ := GetOrFetchMetadata(ctx, "ws", "dsn-A", db, dialect, "db", false)
	m2b, _ := GetOrFetchMetadata(ctx, "ws", "dsn-B", db, dialect, "db", false)
	if m1a == m1b {
		t.Fatal("dsn-A should have been refetched after Invalidate")
	}
	if m2a != m2b {
		t.Fatal("dsn-B should still be cached")
	}
}

func TestClearMetadataCache(t *testing.T) {
	t.Cleanup(ClearMetadataCache)
	ClearMetadataCache()

	dialect := sqlite.NewDialect()
	db, err := dialect.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()

	m1a, _ := GetOrFetchMetadata(ctx, "ws", "dsn-A", db, dialect, "db", false)
	m2a, _ := GetOrFetchMetadata(ctx, "ws", "dsn-B", db, dialect, "db", false)

	ClearMetadataCache()

	m1b, _ := GetOrFetchMetadata(ctx, "ws", "dsn-A", db, dialect, "db", false)
	m2b, _ := GetOrFetchMetadata(ctx, "ws", "dsn-B", db, dialect, "db", false)
	if m1a == m1b || m2a == m2b {
		t.Fatal("expected fresh metadata after Clear")
	}
}

func TestGetOrFetchMetadata_Concurrent(t *testing.T) {
	t.Cleanup(ClearMetadataCache)
	ClearMetadataCache()

	dialect := sqlite.NewDialect()
	db, err := dialect.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()

	const n = 10
	var wg sync.WaitGroup
	results := make([]*core.Metadata, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m, err := GetOrFetchMetadata(ctx, "ws", "dsn", db, dialect, "db", false)
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
				return
			}
			results[idx] = m
		}(i)
	}
	wg.Wait()

	for i := 1; i < n; i++ {
		if results[i] != results[0] {
			t.Fatalf("goroutine %d got a different *Metadata than goroutine 0", i)
		}
	}
}

func TestGetOrFetchMetadata_InvalidateForcesFresh(t *testing.T) {
	t.Cleanup(ClearMetadataCache)
	ClearMetadataCache()

	dialect := sqlite.NewDialect()
	db, err := dialect.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()

	m1, err := GetOrFetchMetadata(ctx, "ws", "dsn", db, dialect, "db", false)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := GetOrFetchMetadata(ctx, "ws", "dsn", db, dialect, "db", false)
	if err != nil {
		t.Fatal(err)
	}
	if m1 != m2 {
		t.Fatal("expected cache hit before invalidation")
	}

	InvalidateMetadata("ws", "dsn")

	m3, err := GetOrFetchMetadata(ctx, "ws", "dsn", db, dialect, "db", false)
	if err != nil {
		t.Fatal(err)
	}
	if m3 == m1 {
		t.Fatal("expected fresh metadata after invalidation")
	}
}

func TestHashWorkspaceDSN_StableAndScoped(t *testing.T) {
	if hashWorkspaceDSN("ws-1", "dsn-1") != hashWorkspaceDSN("ws-1", "dsn-1") {
		t.Fatal("key must be stable for the same inputs")
	}
	if hashWorkspaceDSN("ws-1", "dsn-1") == hashWorkspaceDSN("ws-2", "dsn-1") {
		t.Fatal("workspace must be part of the key")
	}
	if hashWorkspaceDSN("ws-1", "dsn-1") == hashWorkspaceDSN("ws-1", "dsn-2") {
		t.Fatal("dsn must be part of the key")
	}
	// Defence-in-depth: the literal DSN must not appear in the key.
	const sneakyDSN = "postgres://user:secret@host/db"
	k := hashWorkspaceDSN("ws", sneakyDSN)
	if k == sneakyDSN {
		t.Fatal("DSN leaked verbatim into cache key")
	}
}
