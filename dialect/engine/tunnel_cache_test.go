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

// A TTL expiry and an LRU eviction drop an entry with nobody on the call stack
// to close what it held, and an unclosed tunnel is a socket into the customer's
// network that outlives the entry naming it.
func TestATunnelLeavingTheCacheIsClosed(t *testing.T) {
	ClearTunnelCache()
	defer ClearTunnelCache()

	tunnel := &stubTunnel{}
	tunnelCache.Set(workspaceCacheKey("ws-1", "tunnel"), tunnel)

	// What expiry and eviction do, without waiting twenty minutes for it.
	tunnelCache.Delete(workspaceCacheKey("ws-1", "tunnel"))

	if !tunnel.closed {
		t.Error("a tunnel dropped from the cache was left open")
	}
}

func TestCloseWorkspaceTunnelsClosesOnlyThatWorkspace(t *testing.T) {
	ClearTunnelCache()
	defer ClearTunnelCache()

	deleted, kept := &stubTunnel{}, &stubTunnel{}
	deletedKey := workspaceCacheKey("ws-deleted", "tunnel")
	keptKey := workspaceCacheKey("ws-kept", "tunnel")
	tunnelCache.Set(deletedKey, deleted)
	tunnelCache.Set(keptKey, kept)

	CloseWorkspaceTunnels("ws-deleted")

	if !deleted.closed {
		t.Error("the deleted workspace's tunnel is still open")
	}
	if kept.closed {
		t.Error("another workspace's tunnel was closed with it")
	}
	if _, ok := getTunnel(keptKey); !ok {
		t.Error("another workspace's tunnel was dropped from the cache")
	}
	if _, ok := getTunnel(deletedKey); ok {
		t.Error("the deleted workspace's tunnel is still cached")
	}
}

// The cache is written by callers holding tunnelCacheMu and swept by its own
// goroutine holding none of it, so everything it keeps has to be the cache's
// own state and nothing beside it.
func TestTunnelCacheTakesConcurrentWritesAndSweeps(t *testing.T) {
	ClearTunnelCache()
	defer ClearTunnelCache()

	var writers sync.WaitGroup
	for worker := range 8 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := range 200 {
				key := workspaceCacheKey("ws-1", fmt.Sprintf("tunnel-%d-%d", worker, i))
				tunnelCacheMu.Lock()
				tunnelCache.Set(key, &stubTunnel{})
				tunnelCacheMu.Unlock()

				// What expiry does, from a goroutine that holds neither lock.
				tunnelCache.Delete(key)
			}
		}()
	}
	writers.Wait()

	if _, ok := getTunnel(workspaceCacheKey("ws-1", "tunnel-0-0")); ok {
		t.Error("the cache kept a key it was told to drop")
	}
}
