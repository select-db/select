package engine

import (
	"fmt"
	"sync"
	"testing"
)

// stubTunnel stands in for a dialled tunnel: the cache only ever hands its
// value back, and these tests are about what the cache does around it.
type stubTunnel struct{ closed bool }

func (t *stubTunnel) IsAlive() bool           { return !t.closed }
func (t *stubTunnel) LocalAddr() string       { return "127.0.0.1:1" }
func (t *stubTunnel) LocalPort() (int, error) { return 1, nil }
func (t *stubTunnel) Close()                  { t.closed = true }

func indexedWorkspace(key string) (string, bool) {
	tunnelKeyToWorkspaceMu.Lock()
	defer tunnelKeyToWorkspaceMu.Unlock()
	id, ok := tunnelKeyToWorkspace[key]
	return id, ok
}

// Nothing asks before a TTL expiry or an LRU eviction, so an index pruned only
// by the callers that delete deliberately grows for the life of the process.
func TestTunnelWorkspaceIndexIsPrunedByTheCache(t *testing.T) {
	ClearTunnelCache()
	defer ClearTunnelCache()

	const key = "tunnel-key"
	tunnelCache.Set(key, &stubTunnel{})
	indexTunnel(key, "ws-1")

	if _, ok := indexedWorkspace(key); !ok {
		t.Fatal("the index does not hold an entry that was just added")
	}

	// What expiry and eviction do, without waiting twenty minutes for it.
	tunnelCache.Delete(key)

	if id, ok := indexedWorkspace(key); ok {
		t.Errorf("the index kept %q => %q after the cache dropped the entry", key, id)
	}
}

func TestCloseWorkspaceTunnelsClosesOnlyThatWorkspace(t *testing.T) {
	ClearTunnelCache()
	defer ClearTunnelCache()

	deleted, kept := &stubTunnel{}, &stubTunnel{}
	tunnelCache.Set("deleted-key", deleted)
	indexTunnel("deleted-key", "ws-deleted")
	tunnelCache.Set("kept-key", kept)
	indexTunnel("kept-key", "ws-kept")

	CloseWorkspaceTunnels("ws-deleted")

	if !deleted.closed {
		t.Error("the deleted workspace's tunnel is still open")
	}
	if kept.closed {
		t.Error("another workspace's tunnel was closed with it")
	}
	if _, ok := getTunnel("kept-key"); !ok {
		t.Error("another workspace's tunnel was dropped from the cache")
	}
	if _, ok := indexedWorkspace("deleted-key"); ok {
		t.Error("the index kept the closed tunnel")
	}
}

// The index has its own mutex because the cache prunes it from a goroutine of
// its own. A write to it under any other lock is a concurrent map write, which
// is not an error to handle but the end of the process.
func TestTunnelWorkspaceIndexIsWrittenUnderOneLock(t *testing.T) {
	ClearTunnelCache()
	defer ClearTunnelCache()

	var writers sync.WaitGroup
	for worker := range 8 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := range 200 {
				key := fmt.Sprintf("key-%d-%d", worker, i)
				tunnelCacheMu.Lock()
				tunnelCache.Set(key, &stubTunnel{})
				indexTunnel(key, "ws-1")
				tunnelCacheMu.Unlock()

				// What expiry does, from a goroutine that holds neither lock.
				tunnelCache.Delete(key)
			}
		}()
	}
	writers.Wait()

	if _, ok := indexedWorkspace("key-0-0"); ok {
		t.Error("the index kept a key the cache dropped")
	}
}
