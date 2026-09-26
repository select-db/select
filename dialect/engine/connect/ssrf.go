package connect

import (
	"fmt"
	"net"
	"strings"
)

// extraBlockedNets: ranges not covered by the net.IP helpers. RFC1918 is NOT
// listed on purpose (real DBs live there).
var extraBlockedNets = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8",         // "this host" / unspecified IPv4 block
		"fd00:ec2::254/128", // AWS IMDSv2 IPv6 endpoint
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// isBlockedIP: loopback, link-local (covers metadata 169.254.169.254), or
// unspecified.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, n := range extraBlockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// isMetadataIP blocks cloud-metadata / link-local / unspecified but NOT
// loopback: a DB reached through an SSH tunnel is very commonly bound to the
// bastion's 127.0.0.1, so loopback must stay allowed for the tunnel target.
func isMetadataIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, n := range extraBlockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// EnforceOutboundGuard gates the checks below. Enable ONLY on the proxy server
// (dials other users' DSNs/bastions = SSRF surface). The desktop app dials the
// user's own machine, so it stays off (default).
var EnforceOutboundGuard bool

// resolveAllowed resolves host and returns its IPs. The one rule for every
// outbound check: the host is refused when it does not resolve or when any of
// its IPs is blocked, since the driver may dial any of them.
func resolveAllowed(host string, blocked func(net.IP) bool) ([]net.IP, error) {
	host = strings.TrimSpace(host)
	host = strings.Trim(host, "[]") // strip IPv6 literal brackets
	if host == "" {
		return nil, fmt.Errorf("connection target is not permitted")
	}
	ips := []net.IP{net.ParseIP(host)}
	if ips[0] == nil {
		var err error
		if ips, err = net.LookupIP(host); err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("connection to %q is not permitted", host)
		}
	}
	for _, ip := range ips {
		if blocked(ip) {
			return nil, fmt.Errorf("connection to %q is not permitted", host)
		}
	}
	return ips, nil
}

// validateOutboundHost: a direct proxy->DB dial. Blocks loopback too (reaching
// the proxy's own services is an attack here). RFC1918 allowed.
func validateOutboundHost(host string) error {
	if !EnforceOutboundGuard {
		return nil
	}
	_, err := resolveAllowed(host, isBlockedIP)
	return err
}

// validateTunnelTarget: the DB host a bastion dials on our behalf. Blocks
// metadata/link-local (bastion-pivot to IMDS) but allows loopback/RFC1918,
// the normal tunnel case.
func validateTunnelTarget(host string) error {
	if !EnforceOutboundGuard {
		return nil
	}
	_, err := resolveAllowed(host, isMetadataIP)
	return err
}
