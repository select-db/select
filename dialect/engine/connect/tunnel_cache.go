package connect

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/toolkit/cache"
)

// authFingerprint reduces the SSH credential material to a SHA-256 digest so the
// tunnel cache key changes when credentials change, without any raw secret
// entering the key (workspaceCacheKey is a non-cryptographic hash).
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
		// Every way out of the cache closes the tunnel: expiry and eviction drop
		// an entry with nobody on the call stack to do it, and a tunnel dropped
		// without being closed is a socket into the customer's network that
		// outlives the entry that named it.
		OnDelete: closeDeletedTunnel,
	})
	tunnelCacheMu sync.Mutex
)

// closeDeletedTunnel closes a tunnel that has left the cache and flushes the
// pools opened through it, which are now pointing at a local port nothing
// answers on. Runs for every deletion, via the cache's OnDelete.
func closeDeletedTunnel(_ string, value any) {
	tunnel, ok := value.(Tunnel)
	if !ok {
		return
	}

	addr := tunnel.LocalAddr()
	tunnel.Close()
	DeleteConnsByAddr(addr)
}

// tunneledDSN opens, or reuses, the tunnel to the DSN's host and returns the
// DSN pointed at its local end.
func tunneledDSN(workspaceID string, dialect core.SQLDialect, dsn string, ssh ResolvedSSHConfig) (string, error) {
	remoteHost, remotePort, err := dialect.DSNHost(dsn)
	if err != nil {
		return "", newConfigErrorf("parse DSN for SSH: %v", err)
	}
	// The bastion dials remoteHost for us; stop it pivoting to its own
	// cloud-metadata/link-local (loopback stays allowed: common tunnel case).
	if err := validateTunnelTarget(remoteHost); err != nil {
		return "", err
	}
	tunnel, err := GetOrCreateTunnel(workspaceID, ssh, remoteHost, remotePort)
	if err != nil {
		return "", fmt.Errorf("SSH tunnel: %w", err)
	}
	localPort, err := tunnel.LocalPort()
	if err != nil {
		return "", fmt.Errorf("SSH tunnel local port: %w", err)
	}
	return dialect.DSNWithHost(dsn, "127.0.0.1", localPort)
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
	key := WorkspaceCacheKey(workspaceID, addrStr)

	tunnelCacheMu.Lock()
	if existing, ok := getTunnel(key); ok && existing.IsAlive() {
		tunnelCacheMu.Unlock()
		return existing, nil
	}
	// Dead or absent: the delete closes it and flushes its connections.
	tunnelCache.Delete(key)
	tunnelCacheMu.Unlock()

	tunnel, err := StartSSHTunnel(config, remoteHost, remotePort)
	if err != nil {
		return nil, err
	}

	// re-check: another goroutine may have raced the dial
	tunnelCacheMu.Lock()
	if existing, ok := getTunnel(key); ok && existing.IsAlive() {
		tunnelCacheMu.Unlock()
		tunnel.Close()
		return existing, nil
	}

	// Set replaces whatever was there, and a replacement is a deletion: the one
	// it displaces is closed and flushed by OnDelete.
	tunnelCache.Set(key, tunnel)
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
// workspace is deleted: a tunnel that outlives it is a live socket into the
// customer's network.
func CloseWorkspaceTunnels(workspaceID string) {
	prefix := WorkspaceKeyPrefix(workspaceID)

	tunnelCacheMu.Lock()
	tunnelCache.DeleteFunc(func(key string) bool { return strings.HasPrefix(key, prefix) })
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
