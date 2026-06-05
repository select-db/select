package engine

import (
	"context"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	"database/sql"
)

// The ssrf.go pre-dial hostname check can't stop DNS rebinding: the driver
// re-resolves and may dial a different IP. net.Dialer.Control runs after
// resolution, before connect, on every attempt, re-validate the real IP there.

// guardedNetDialer rejects the connect if the resolved IP is blocked; the
// timeout also bounds a black-hole dial.
func guardedNetDialer(blocked func(net.IP) bool) *net.Dialer {
	return &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("connection target is not permitted")
			}
			ip := net.ParseIP(host)
			if ip == nil || blocked(ip) {
				return fmt.Errorf("connection to %q is not permitted", address)
			}
			return nil
		},
	}
}

// pqGuardDialer adapts guardedNetDialer to lib/pq's Dialer/DialerContext.
type pqGuardDialer struct{}

func (pqGuardDialer) Dial(network, address string) (net.Conn, error) {
	return guardedNetDialer(isBlockedIP).Dial(network, address)
}

func (pqGuardDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return guardedNetDialer(isBlockedIP).DialContext(ctx, network, address)
}

func (pqGuardDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return guardedNetDialer(isBlockedIP).DialContext(ctx, network, address)
}

const guardedMySQLNet = "selectdb-guarded-tcp"

var registerMySQLGuardOnce sync.Once

func registerMySQLGuard() {
	registerMySQLGuardOnce.Do(func() {
		mysqldriver.RegisterDialContext(guardedMySQLNet, func(ctx context.Context, addr string) (net.Conn, error) {
			return guardedNetDialer(isBlockedIP).DialContext(ctx, "tcp", addr)
		})
	})
}

// openGuardedDB opens pg/mysql with per-dial IP re-validation. Proxy
// direct-outbound only (no SSH); dbType already constrained to pg/mysql.
func openGuardedDB(dbType, dsn string) (*sql.DB, error) {
	switch dbType {
	case "postgresql":
		connector, err := pq.NewConnector(dsn)
		if err != nil {
			return nil, err
		}
		connector.Dialer(pqGuardDialer{})
		db := sql.OpenDB(connector)
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	case "mysql":
		cfg, err := mysqldriver.ParseDSN(dsn)
		if err != nil {
			return nil, err
		}
		// TCP only: unix/custom nets would open something local on the proxy
		if cfg.Net != "" && cfg.Net != "tcp" {
			return nil, fmt.Errorf("connection target is not permitted")
		}
		registerMySQLGuard()
		cfg.Net = guardedMySQLNet
		connector, err := mysqldriver.NewConnector(cfg)
		if err != nil {
			return nil, err
		}
		return sql.OpenDB(connector), nil
	default:
		return nil, fmt.Errorf("connection target is not permitted")
	}
}
