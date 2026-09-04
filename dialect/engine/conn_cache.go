package engine

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/toolkit/cache"
)

// PoolConfig tunes the sql.DB connection pool for a proxified datasource.
// Zero values use Go defaults (unlimited open, 2 idle, no lifetime/idle limits).
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

var (
	// 20k entries × ~30KB = ~600MB max
	connCache = cache.New(cache.Options{
		MaxEntries: 20_000,
		TTL:        20 * time.Minute,
		OnRemove:   closeRemovedPool,
	})

	// secondary index: hash → DSN for EvictConnsByAddr. SSH-rewritten DSNs are 127.0.0.1:port.
	connHashToDSN   = make(map[string]string)
	connHashToDSNMu sync.Mutex
)

// poolCloseGrace is how long closeRemovedPool waits before closing a pool that
// has left the cache, so a caller that took it just before removal can still
// start its query. Close does not interrupt queries already in flight. A var so
// tests can shorten it.
var poolCloseGrace = 60 * time.Second

// closeRemovedPool releases a pool that has left the cache, for any reason:
// idle past the TTL, evicted under LRU pressure, or dropped with the tunnel it
// depended on.
func closeRemovedPool(hash string, value any) {
	connHashToDSNMu.Lock()
	delete(connHashToDSN, hash)
	connHashToDSNMu.Unlock()

	db, ok := value.(*sql.DB)
	if !ok {
		return
	}
	time.AfterFunc(poolCloseGrace, func() { _ = db.Close() })
}

// getConn looks up a cached *sql.DB by (workspaceID, dsn). DSN is hashed, never stored verbatim.
func getConn(workspaceID, dsn string) (*sql.DB, bool) {
	value, ok := connCache.Get(hashWorkspaceDSN(workspaceID, dsn))
	if !ok {
		return nil, false
	}
	return value.(*sql.DB), true
}

// indexConn records hash → dsn so EvictConnsByAddr can find this pool again.
func indexConn(hash, dsn string) {
	connHashToDSNMu.Lock()
	connHashToDSN[hash] = dsn
	connHashToDSNMu.Unlock()
}

// setConn stores db under (workspaceID, dsn). The index write follows the cache
// write, because replacing an entry fires closeRemovedPool for the old value and
// that clears the index entry for this same hash.
func setConn(workspaceID, dsn string, db *sql.DB) {
	hash := hashWorkspaceDSN(workspaceID, dsn)
	connCache.Set(hash, db)
	indexConn(hash, dsn)
}

// applyPoolConfig applies the non-zero fields of cfg; the rest keep Go's defaults.
func applyPoolConfig(db *sql.DB, cfg PoolConfig) {
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}

// EvictConnsByAddr drops all connections whose DSN contains addr. Called when SSH tunnel dies.
func EvictConnsByAddr(addr string) {
	var toDelete []string
	connHashToDSNMu.Lock()
	for hash, dsn := range connHashToDSN {
		if strings.Contains(dsn, addr) {
			toDelete = append(toDelete, hash)
			delete(connHashToDSN, hash)
		}
	}
	connHashToDSNMu.Unlock()
	// Delete fires closeRemovedPool, which closes the pool behind the dead
	// tunnel rather than leaving it to linger on a socket that no longer works.
	for _, hash := range toDelete {
		connCache.Delete(hash)
	}
}

// GetOrOpenConn returns a cached *sql.DB, opening one on miss. dsn must have $variables substituted.
// ssh is optional: when non-nil, establishes/reuses a tunnel and rewrites the DSN before opening.
// Concurrent first queries for one datasource share a single open.
func GetOrOpenConn(workspaceID, dbType, dsn string, ssh *ResolvedSSHConfig, pool ...PoolConfig) (*sql.DB, error) {
	if dbType == "sqlite" && ssh != nil {
		return nil, newConfigError("SSH tunneling is not supported for sqlite")
	}

	if ssh != nil {
		remoteHost, remotePort, err := core.ParseDSNRemote(dbType, dsn)
		if err != nil {
			return nil, fmt.Errorf("parse DSN for SSH: %w", err)
		}
		// The bastion dials remoteHost for us; stop it pivoting to its own
		// cloud-metadata/link-local (loopback stays allowed: common tunnel case).
		if verr := validateTunnelTarget(remoteHost); verr != nil {
			return nil, verr
		}

		tunnel, err := GetOrCreateTunnel(workspaceID, *ssh, remoteHost, remotePort)
		if err != nil {
			return nil, fmt.Errorf("SSH tunnel: %w", err)
		}

		localPort, err := tunnel.LocalPort()
		if err != nil {
			return nil, fmt.Errorf("SSH tunnel local port: %w", err)
		}

		dsn, err = core.RewriteDSNForLocal(dbType, dsn, "127.0.0.1", localPort)
		if err != nil {
			return nil, fmt.Errorf("rewrite DSN for SSH: %w", err)
		}
	}

	guardedDirect := false
	if ssh == nil && EnforceOutboundGuard {
		// Proxy: only validated networked dialects may be dialed. Anything we
		// cannot parse+validate, or any non-networked driver (sqlite and other
		// local-file drivers open a path on THIS host), is refused.
		host, _, perr := core.ParseDSNRemote(dbType, dsn)
		if perr != nil || (dbType != "postgresql" && dbType != "mysql") {
			return nil, fmt.Errorf("connection target is not permitted")
		}
		// Cheap pre-dial reject; fails closed on unresolvable / all-blocked
		if verr := validateOutboundHost(host); verr != nil {
			return nil, verr
		}
		// Authoritative check is the per-dial IP guard below (re-runs on the
		// real resolved IP), so rebinding is caught
		guardedDirect = true
	}
	// Guard off (desktop app): dialing the user's own machine, incl. a local
	// sqlite file, is the intended use and must not be restricted.

	var cfg PoolConfig
	if len(pool) > 0 {
		cfg = pool[0]
	}
	hash := hashWorkspaceDSN(workspaceID, dsn)

	// GetOrCreate opens at most once per key: concurrent first queries for one
	// datasource share the single open instead of each dialing and discarding
	// the losers. Worth serialising because the open is a real round trip on
	// postgres — both paths below Ping — through the SSH tunnel when there is
	// one. A failure is not cached, so the next caller retries.
	value, err := connCache.GetOrCreate(hash, func() (any, error) {
		dialect := GetDialect(dbType)
		if dialect == nil {
			return nil, newConfigErrorf("unsupported database type: %s", dbType)
		}

		var (
			db  *sql.DB
			err error
		)
		if guardedDirect {
			// Per-dial IP guard: re-validates the resolved IP at connect (beats rebinding)
			db, err = openGuardedDB(dbType, dsn)
		} else {
			db, err = dialect.OpenDB(dsn)
		}
		if err != nil {
			return nil, err
		}
		applyPoolConfig(db, cfg)

		// Indexed from in here so only the caller that opened the pool writes
		// the index, rather than every cache hit taking that lock. Ordering is
		// safe even though closeRemovedPool clears this hash: GetOrCreate runs
		// create only on a miss, so the store that follows displaces no entry
		// under this key.
		indexConn(hash, dsn)
		return db, nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*sql.DB), nil
}

// ClearConnCache drops all connection cache entries, closing each pool.
func ClearConnCache() {
	connCache.DeleteFunc(func(string) bool { return true })
	connHashToDSNMu.Lock()
	connHashToDSN = make(map[string]string)
	connHashToDSNMu.Unlock()
}
