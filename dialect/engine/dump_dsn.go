package engine

import (
	"fmt"
	"net"
	"strings"

	"github.com/selectDb/dialect/core"
)

// ResolveDumpDSN returns a DSN safe for the out-of-process dump tools, which
// resolve/dial the host themselves (the Go guarded dialer can't reach them).
// Pins the target like the driver path:
//   - SSH: rewrite to the existing tunnel's local endpoint
//   - direct + guard on: substitute the resolved+validated literal IP (no re-resolve)
//   - guard off (desktop): unchanged
func ResolveDumpDSN(workspaceID, dbType, dsn string, ssh *ResolvedSSHConfig) (string, error) {
	if ssh != nil {
		remoteHost, remotePort, err := core.ParseDSNRemote(dbType, dsn)
		if err != nil {
			return "", fmt.Errorf("parse DSN for SSH: %w", err)
		}
		if verr := validateTunnelTarget(remoteHost); verr != nil {
			return "", verr
		}
		tunnel, err := GetOrCreateTunnel(workspaceID, *ssh, remoteHost, remotePort)
		if err != nil {
			return "", fmt.Errorf("SSH tunnel: %w", err)
		}
		localPort, err := tunnel.LocalPort()
		if err != nil {
			return "", fmt.Errorf("SSH tunnel local port: %w", err)
		}
		return core.RewriteDSNForLocal(dbType, dsn, "127.0.0.1", localPort)
	}

	if !EnforceOutboundGuard {
		return dsn, nil
	}

	host, port, err := core.ParseDSNRemote(dbType, dsn)
	if err != nil || (dbType != "postgresql" && dbType != "mysql") {
		return "", fmt.Errorf("connection target is not permitted")
	}
	ip, err := resolveAllowedIP(host)
	if err != nil {
		return "", err
	}
	return core.RewriteDSNForLocal(dbType, dsn, ip, port)
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
