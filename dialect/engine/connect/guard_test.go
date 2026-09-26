package connect

import (
	"errors"
	"strings"
	"testing"

	"github.com/selectDb/dialect/dialects"
	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	"github.com/selectDb/dialect/sqlite"
)

func TestValidateOutboundHostDisabledByDefault(t *testing.T) {
	if EnforceOutboundGuard {
		t.Fatal("EnforceOutboundGuard must default to false so the desktop app can reach localhost")
	}
	// Guard off: the desktop app's local connections must be allowed.
	for _, h := range []string{"localhost", "127.0.0.1", "::1", "169.254.169.254"} {
		if err := validateOutboundHost(h); err != nil {
			t.Errorf("guard disabled: validateOutboundHost(%q) = %v, want nil", h, err)
		}
	}
}

func TestValidateOutboundHostEnforced(t *testing.T) {
	EnforceOutboundGuard = true
	defer func() { EnforceOutboundGuard = false }()

	blocked := []string{"127.0.0.1", "::1", "169.254.169.254", "0.0.0.0"}
	for _, h := range blocked {
		if err := validateOutboundHost(h); err == nil {
			t.Errorf("guard enabled: validateOutboundHost(%q) = nil, want blocked", h)
		}
	}

	// Private RFC1918 stays allowed: real databases live there.
	allowed := []string{"10.0.0.5", "192.168.1.10", "172.16.3.4", "8.8.8.8"}
	for _, h := range allowed {
		if err := validateOutboundHost(h); err != nil {
			t.Errorf("guard enabled: validateOutboundHost(%q) = %v, want nil", h, err)
		}
	}

	// Fail closed: a host that cannot be parsed as an IP or resolved must be
	// rejected, not allowed through (the old behaviour let a quoted
	// "'169.254.169.254'" slip past because LookupIP errored).
	failClosed := []string{"", "'169.254.169.254'", "no-such-host.invalid"}
	for _, h := range failClosed {
		if err := validateOutboundHost(h); err == nil {
			t.Errorf("guard enabled: validateOutboundHost(%q) = nil, want rejected (fail closed)", h)
		}
	}
}

// TestSSRFParserDifferentialClosed is the regression test for the SSRF guard
// bypass: a libpq-quoted host must be unquoted by DSNHost (so it equals
// what lib/pq dials) and then blocked by the guard.
func TestSSRFParserDifferentialClosed(t *testing.T) {
	EnforceOutboundGuard = true
	defer func() { EnforceOutboundGuard = false }()

	dsn := "host='169.254.169.254' port=80 sslmode=disable dbname=x user=x"
	host, port, err := dialects.Get("postgresql").DSNHost(dsn)
	if err != nil {
		t.Fatalf("DSNHost error: %v", err)
	}
	if host != "169.254.169.254" {
		t.Fatalf("host = %q, want unquoted 169.254.169.254 (must match lib/pq)", host)
	}
	if port != 80 {
		t.Fatalf("port = %d, want 80", port)
	}
	if err := validateOutboundHost(host); err == nil {
		t.Fatal("guard must block the cloud-metadata host after unquoting")
	}
}

func TestValidateTunnelTarget(t *testing.T) {
	if err := validateTunnelTarget("169.254.169.254"); err != nil {
		t.Error("guard off: tunnel target must be a no-op")
	}

	EnforceOutboundGuard = true
	defer func() { EnforceOutboundGuard = false }()

	// Metadata / link-local / unspecified blocked.
	for _, h := range []string{"169.254.169.254", "0.0.0.0", "", "no-such.invalid"} {
		if err := validateTunnelTarget(h); err == nil {
			t.Errorf("tunnel target %q must be blocked", h)
		}
	}
	// Loopback + RFC1918 allowed (the normal SSH-tunnel case).
	for _, h := range []string{"127.0.0.1", "::1", "10.0.0.5", "192.168.1.9"} {
		if err := validateTunnelTarget(h); err != nil {
			t.Errorf("tunnel target %q must be allowed, got %v", h, err)
		}
	}
}

// TestProxyRefusesNonNetworkedDialect is the C1 regression: with the guard on,
// the proxy must refuse sqlite / any non-pg-mysql type before dialing, so a
// "datasource" cannot open an arbitrary file on the server host.
func TestProxyRefusesNonNetworkedDialect(t *testing.T) {
	EnforceOutboundGuard = true
	defer func() { EnforceOutboundGuard = false }()

	for _, tc := range []struct{ dbType, dsn string }{
		{"sqlite", "file:/opt/selectdb/secrets/jwt_private.pem"},
		{"sqlite", "/etc/passwd"},
		{"clickhouse", "tcp://attacker/db"},
		{"postgresql", "host=169.254.169.254 port=80 user=x"}, // metadata still blocked
	} {
		if _, err := GetOrOpen("ws", tc.dbType, tc.dsn, nil); err == nil {
			t.Errorf("GetOrOpen(%q,%q) = nil err, want refused", tc.dbType, tc.dsn)
		}
	}
}

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
func TestGetOrOpenRefusesASqliteFileOverSSH(t *testing.T) {
	var cfgErr *ConfigError
	_, err := GetOrOpen("ws", "sqlite", "file:/etc/passwd", &ResolvedSSHConfig{Host: "bastion"})
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

// The dump path follows the driver path's rule and pins an allowed host to its IP.
func TestResolveDumpDSNFollowsTheOutboundRule(t *testing.T) {
	EnforceOutboundGuard = true
	defer func() { EnforceOutboundGuard = false }()
	if _, err := ResolveDumpDSN("ws", "postgresql", "host=169.254.169.254 dbname=x", nil); err == nil {
		t.Fatal("a metadata host was handed to the dump tools")
	}
	got, err := ResolveDumpDSN("ws", "postgresql", "host=8.8.8.8 port=5432 dbname=x", nil)
	if err != nil || !strings.Contains(got, "host=8.8.8.8") {
		t.Fatalf("got %q, %v; want the literal IP kept", got, err)
	}
}
