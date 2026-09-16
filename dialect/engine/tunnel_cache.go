package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/selectDb/toolkit/cache"
)

// authFingerprint reduces the SSH credential material to a SHA-256 digest so the
// tunnel cache key changes when credentials change, without any raw secret
// entering the key (hashWorkspaceDSN is a non-cryptographic hash).
func authFingerprint(config ResolvedSSHConfig) string {
	h := sha256.New()
	for _, s := range []string{
		config.KeyPath,
		config.Passphrase,
		config.HostKey,
		config.Password,
		config.PrivateKey,
	} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

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
	tunnelCache = cache.New(cache.Options{
		MaxEntries: 20_000,
		TTL:        20 * time.Minute,
		// Expiry and LRU eviction drop an entry without anybody asking, so the
		// index is pruned from here rather than from the callers that delete.
		OnDelete: forgetTunnelWorkspace,
	})

	// secondary index: key → workspace, for CloseWorkspaceTunnels. The key is a
	// hash, so nothing else can tell whose tunnel an entry is.
	//
	// Under its own mutex, not tunnelCacheMu: OnDelete runs from inside a cache
	// call that the functions below make while holding that one.
	tunnelKeyToWorkspace   = make(map[string]string)
	tunnelKeyToWorkspaceMu sync.Mutex

	tunnelCacheMu sync.Mutex
)

func indexTunnel(key, workspaceID string) {
	tunnelKeyToWorkspaceMu.Lock()
	tunnelKeyToWorkspace[key] = workspaceID
	tunnelKeyToWorkspaceMu.Unlock()
}

func forgetTunnelWorkspace(key string, _ any) {
	tunnelKeyToWorkspaceMu.Lock()
	delete(tunnelKeyToWorkspace, key)
	tunnelKeyToWorkspaceMu.Unlock()
}

// closeTunnelLocked drops the entry for key and closes the tunnel it held,
// returning the local address its connections were opened against so the caller
// can flush them once it has let go of tunnelCacheMu. Callers hold that mutex.
func closeTunnelLocked(key string) string {
	tunnel, ok := getTunnel(key)
	if !ok {
		tunnelCache.Delete(key)
		return ""
	}

	addr := tunnel.LocalAddr()
	tunnelCache.Delete(key)
	tunnel.Close()
	return addr
}

// GetOrCreateTunnel returns a live cached tunnel, dialling via StartSSHTunnel on miss.
// Key is hash(workspaceID, full SSH identity + remote address) scoped per workspace, 
// secrets hashed (never stored verbatim). Editing any auth detail changes the key.
// Dead entries are deleted and their DB connections flushed before redialling.
// Serialised: only one tunnel per key established under concurrent callers.
func GetOrCreateTunnel(workspaceID string, config ResolvedSSHConfig, remoteHost string, remotePort int) (Tunnel, error) {
	addrStr := strings.Join([]string{
		config.Host,
		strconv.Itoa(config.Port),
		config.User,
		config.AuthMethod,
		remoteHost,
		strconv.Itoa(remotePort),
		authFingerprint(config),
	}, "\x00")
	key := hashWorkspaceDSN(workspaceID, addrStr)

	tunnelCacheMu.Lock()
	if existing, ok := getTunnel(key); ok {
		if existing.IsAlive() {
			tunnelCacheMu.Unlock()
			return existing, nil
		}
		addr := closeTunnelLocked(key)
		tunnelCacheMu.Unlock()
		DeleteConnsByAddr(addr)
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
		tunnelKeyToWorkspace[key] = workspaceID
		tunnelCacheMu.Unlock()
		DeleteConnsByAddr(addr)
		return tunnel, nil
	}
	tunnelCache.Set(key, tunnel)
	tunnelKeyToWorkspace[key] = workspaceID
	tunnelCacheMu.Unlock()
	return tunnel, nil
}

// DeleteTunnel deletes the entry for key. No-op if absent.
func DeleteTunnel(key string) {
	tunnelCacheMu.Lock()
	tunnelCache.Delete(key)
	tunnelCacheMu.Unlock()
}

// CloseWorkspaceTunnels closes every tunnel a workspace opened. Called when the
// workspace is deleted: the tunnel outlives the cache entry otherwise, and it
// is a live socket into the customer's network.
func CloseWorkspaceTunnels(workspaceID string) {
	// The index is read and released before tunnelCacheMu is taken: a cache
	// deletion prunes the index from OnDelete, and holding both here in the
	// other order is how that meets itself coming back.
	var keys []string
	tunnelKeyToWorkspaceMu.Lock()
	for key, id := range tunnelKeyToWorkspace {
		if id == workspaceID {
			keys = append(keys, key)
		}
	}
	tunnelKeyToWorkspaceMu.Unlock()

	var addrs []string
	tunnelCacheMu.Lock()
	for _, key := range keys {
		if addr := closeTunnelLocked(key); addr != "" {
			addrs = append(addrs, addr)
		}
	}
	tunnelCacheMu.Unlock()

	for _, addr := range addrs {
		DeleteConnsByAddr(addr)
	}
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
