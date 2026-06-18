package auth

import (
	"backend/db/db_types"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/sqlc-dev/pqtype"
)

var (
	trustedProxyNets []*net.IPNet
	trustedOnce      sync.Once
	trustedOverride  []*net.IPNet // test-only: when set, used instead of env
)

// extractBearerToken extracts the token from Authorization header "Bearer <token>"
func ExtractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

// refreshTokenHashTag domain-separates refresh-token hashes from API-key hashes.
const refreshTokenHashTag = "sdb-refresh-token-v1"

// HashRefreshToken returns SHA-256 of the token bound to deviceID. The token is
// 64 chars of CSPRNG, so a plain hash is as strong as a keyed MAC here: a DB
// leak yields no usable tokens, and the deviceID binding stops a token stolen
// without its device from validating.
func HashRefreshToken(token, deviceID string) string {
	h := sha256.New()
	h.Write([]byte(refreshTokenHashTag))
	h.Write([]byte{0})
	h.Write([]byte(token))
	h.Write([]byte{0})
	h.Write([]byte(deviceID))
	return hex.EncodeToString(h.Sum(nil))
}

// loadTrustedProxyCIDRs reads TRUSTED_PROXY_CIDRS (comma-separated CIDRs, e.g.
// "10.0.0.0/8,172.16.0.0/12,127.0.0.1/32"). Only when the direct connection
// comes from one of these networks is X-Forwarded-For used.
func loadTrustedProxyCIDRs() {
	s := os.Getenv("TRUSTED_PROXY_CIDRS")
	if s == "" {
		return
	}
	for _, cidrStr := range strings.Split(s, ",") {
		cidrStr = strings.TrimSpace(cidrStr)
		if cidrStr == "" {
			continue
		}
		_, n, err := net.ParseCIDR(cidrStr)
		if err != nil {
			continue // skip invalid entries
		}
		trustedProxyNets = append(trustedProxyNets, n)
	}
}

// isTrustedProxy returns true if the given remote IP is in TRUSTED_PROXY_CIDRS.
// Only when true should X-Forwarded-For be trusted.
func isTrustedProxy(remoteIP net.IP) bool {
	var nets []*net.IPNet
	if trustedOverride != nil {
		nets = trustedOverride
	} else {
		trustedOnce.Do(loadTrustedProxyCIDRs)
		nets = trustedProxyNets
	}
	for _, n := range nets {
		if n.Contains(remoteIP) {
			return true
		}
	}
	return false
}

// remoteIPFromRequest returns the direct connection IP (no port).
func remoteIPFromRequest(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return net.ParseIP(r.RemoteAddr)
	}
	return net.ParseIP(host)
}

// GetIPAddress returns the client IP. X-Forwarded-For is used only when the
// direct connection is from a trusted proxy (see TRUSTED_PROXY_CIDRS).
// Otherwise only r.RemoteAddr is used to avoid spoofing.
func GetIPAddress(r *http.Request) string {
	remoteIP := remoteIPFromRequest(r)
	xf := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))

	if xf != "" && remoteIP != nil && isTrustedProxy(remoteIP) {
		// Leftmost is the original client
		if idx := strings.Index(xf, ","); idx != -1 {
			xf = strings.TrimSpace(xf[:idx])
		}
		if host, _, err := net.SplitHostPort(xf); err == nil {
			xf = host
		}
		return xf
	}

	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return ip
}

// toPgInet converts IP string to pqtype.Inet for Postgres storage
func toPgInet(ipStr string) (pqtype.Inet, error) {
	ip := pqtype.Inet{}
	err := ip.Scan(ipStr)
	return ip, err
}

// sameSubnet checks if ip1 and ip2 are in the same subnet (IPv4 /24 or IPv6 /64)
func SameSubnet(ip1 net.IP, ip2 string) bool {
	parsed := net.ParseIP(ip2)
	if parsed == nil || ip1 == nil {
		return false
	}

	var mask net.IPMask
	if ip1.To4() != nil && parsed.To4() != nil {
		// IPv4 subnet mask /24
		mask = net.CIDRMask(24, 32)
	} else {
		// IPv6 subnet mask /64
		mask = net.CIDRMask(64, 128)
	}

	return ip1.Mask(mask).Equal(parsed.Mask(mask))
}

// SendSecurityAlert sends a security alert to the user about IP change (stub)
func SendSecurityAlert(userID db_types.JSONNullUUID, oldIP net.IP, newIP string) {
	// TODO: implement actual notification (email)
}

// GenerateRandomString returns a cryptographically secure random string of
// length n. Uses rejection sampling so the alphabet is uniform (plain b%62
// over-represents the first 256%62 bytes).
func GenerateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const maxUnbiased = 256 - (256 % len(letters)) // 248 = 62*4; reject >= this
	out := make([]byte, n)
	buf := make([]byte, n)
	bi := len(buf)
	for i := 0; i < n; {
		if bi >= len(buf) {
			if _, err := rand.Read(buf); err != nil {
				panic("failed to generate secure random string: " + err.Error())
			}
			bi = 0
		}
		b := buf[bi]
		bi++
		if int(b) >= maxUnbiased {
			continue
		}
		out[i] = letters[int(b)%len(letters)]
		i++
	}
	return string(out)
}
