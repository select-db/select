package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	"github.com/selectDb/dialect/sqlite"
)

// A guarded open must refuse a blocked address when it dials, not only in the
// pre-dial check, which a rebinding host passes.
func TestOpenGuardedDBRefusesABlockedAddressAtDial(t *testing.T) {
	_, err := postgresql.NewDialect().OpenGuardedDB("postgres://u:p@127.0.0.1:5432/x?sslmode=disable", guardedDial)
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("postgresql: err = %v, want not permitted", err)
	}

	db, err := mysql.NewDialect().OpenGuardedDB("u:p@tcp(127.0.0.1:3306)/x", guardedDial)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("mysql: err = %v, want not permitted", err)
	}

	if _, err := sqlite.NewDialect().OpenGuardedDB("file:/etc/passwd", guardedDial); err == nil {
		t.Fatal("sqlite: a guarded open of a file succeeded")
	}
}

// sqlite has no host to tunnel to; the guard side is TestProxyRefusesNonNetworkedDialect.
func TestGetOrOpenConnRefusesASqliteFileOverSSH(t *testing.T) {
	var cfgErr *ConfigError
	_, err := GetOrOpenConn("ws", "sqlite", "file:/etc/passwd", &ResolvedSSHConfig{Host: "bastion"})
	if !errors.As(err, &cfgErr) {
		t.Fatalf("err = %v, want a ConfigError", err)
	}
}

// The dump tools dial for themselves, so the server must refuse a sqlite file
// before handing its DSN over.
func TestResolveDumpDSNRefusesASqliteFileOnTheServer(t *testing.T) {
	EnforceOutboundGuard = true
	defer func() { EnforceOutboundGuard = false }()
	if _, err := ResolveDumpDSN("ws", "sqlite", "file:/etc/passwd", nil); err == nil {
		t.Fatal("a sqlite file was handed to the dump tools")
	}
}
