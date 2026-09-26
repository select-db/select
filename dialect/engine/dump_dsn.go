package engine

import (
	"fmt"
	"net"
	"strings"

	"github.com/selectDb/dialect/sqlite"
)

// ResolveDumpDSN returns a DSN safe for the out-of-process dump tools, which
// resolve/dial the host themselves (the Go guarded dialer can't reach them).
// Pins the target like the driver path.
func ResolveDumpDSN(workspaceID, dbType, dsn string, ssh *ResolvedSSHConfig) (string, error) {
	dialect := GetDialect(dbType)
	if dialect == nil {
		return "", newConfigErrorf("unsupported database type: %s", dbType)
	}

	// Rewritten to the tunnel's local endpoint:
	//   - any datasource with SSH
	// Refused:
	//   - a DSN with no host to tunnel to, such as a sqlite file
	if ssh != nil {
		return tunneledDSN(workspaceID, dialect, dsn, *ssh)
	}

	// Unchanged:
	//   - the desktop app
	//   - a cellar DSN, whose driver dials only the configured cellar
	if !EnforceOutboundGuard || sqlite.IsCellarDSN(dsn) {
		return dsn, nil
	}

	// Rewritten to the resolved and validated literal IP, so the tool cannot re-resolve:
	//   - the server dialing a user's DSN directly
	// Refused:
	//   - a DSN whose host does not parse, incl. a sqlite file (a path on this host)
	host, port, err := dialect.DSNHost(dsn)
	if err != nil {
		return "", fmt.Errorf("connection target is not permitted")
	}
	ip, err := resolveAllowedIP(host)
	if err != nil {
		return "", err
	}
	return dialect.DSNWithHost(dsn, ip, port)
}

// resolveAllowedIP returns the first guard-passing IP for host, failing closed
// if none. A literal IP leaves no second resolution to rebind.
func resolveAllowedIP(host string) (string, error) {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return "", fmt.Errorf("connection target is not permitted")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return "", fmt.Errorf("connection to %q is not permitted", host)
		}
		return ip.String(), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("connection to %q is not permitted", host)
	}
	for _, ip := range ips {
		if !isBlockedIP(ip) {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("connection to %q is not permitted", host)
}
