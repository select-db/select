package engine

import (
	"strconv"
	"sync"
	"time"

	"github.com/selectDb/toolkit/cache"
)

// Tunnel is implemented by sshTunnel. Cache and locking live in the engine
// so neither DbClient nor the remote backend hold tunnel state.
type Tunnel interface {
	IsAlive() bool
	LocalAddr() string
	LocalPort() (int, error)
	Close()
}

var (
	// 20k entries × ~100 KB = ~2GB max
	tunnelCache   = cache.New(cache.Options{MaxEntries: 20_000, TTL: 20 * time.Minute})
	tunnelCacheMu sync.Mutex
)

// GetOrCreateTunnel returns a live cached tunnel, dialling via StartSSHTunnel on miss.
// Key is hash(workspaceID, ssh+remote address), scoped per workspace, secrets never stored.
// Dead entries are evicted and their DB connections flushed before redialling.
// Serialised: only one tunnel per key established under concurrent callers.
func GetOrCreateTunnel(workspaceID string, config ResolvedSSHConfig, remoteHost string, remotePort int) (Tunnel, error) {
	addrStr := config.Host + ":" + strconv.Itoa(config.Port) + ":" + remoteHost + ":" + strconv.Itoa(remotePort)
	key := hashWorkspaceDSN(workspaceID, addrStr)

	tunnelCacheMu.Lock()
	if existing, ok := getTunnel(key); ok {
		if existing.IsAlive() {
			tunnelCacheMu.Unlock()
			return existing, nil
		}
		addr := existing.LocalAddr()
		tunnelCache.Delete(key)
		existing.Close()
		tunnelCacheMu.Unlock()
		EvictConnsByAddr(addr)
	} else {
		tunnelCacheMu.Unlock()
	}

	tunnel, err := StartSSHTunnel(config, remoteHost, remotePort)
	if err != nil {
		return nil, err
	}

	// re-check: another goroutine may have raced the dial
	tunnelCacheMu.Lock()
	if existing, ok := getTunnel(key); ok {
		if existing.IsAlive() {
			tunnelCacheMu.Unlock()
			tunnel.Close()
			return existing, nil
		}
		addr := existing.LocalAddr()
		tunnelCache.Delete(key)
		existing.Close()
		tunnelCache.Set(key, tunnel)
		tunnelCacheMu.Unlock()
		EvictConnsByAddr(addr)
		return tunnel, nil
	}
	tunnelCache.Set(key, tunnel)
	tunnelCacheMu.Unlock()
	return tunnel, nil
}

// DeleteTunnel removes the entry for key. No-op if absent.
func DeleteTunnel(key string) {
	tunnelCacheMu.Lock()
	tunnelCache.Delete(key)
	tunnelCacheMu.Unlock()
}

// ClearTunnelCache drops all entries.
func ClearTunnelCache() {
	tunnelCacheMu.Lock()
	tunnelCache.DeleteFunc(func(string) bool { return true })
	tunnelCacheMu.Unlock()
}

func getTunnel(key string) (Tunnel, bool) {
	value, ok := tunnelCache.Get(key)
	if !ok {
		return nil, false
	}
	return value.(Tunnel), true
}
