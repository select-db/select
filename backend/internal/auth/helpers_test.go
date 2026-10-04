package auth

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		header   string
		expected string
	}{
		{"Bearer token123", "token123"},
		{"Bearer ", ""},
		{"token123", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := ExtractBearerToken(tt.header)
		if got != tt.expected {
			t.Errorf("ExtractBearerToken(%q) = %q; want %q", tt.header, got, tt.expected)
		}
	}
}

func TestHashRefreshToken(t *testing.T) {
	token := "token123"
	deviceID := "device456"

	hash := HashRefreshToken(token, deviceID)

	if len(hash) != 64 { // SHA-256 hex length
		t.Errorf("expected hash length 64, got %d", len(hash))
	}
}

// setTrustedProxyCIDRsForTest sets trusted proxy CIDRs for tests (same package only).
func setTrustedProxyCIDRsForTest(cidrs []string) {
	var nets []*net.IPNet
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		nets = append(nets, n)
	}
	trustedOverride = nets
}

func clearTrustedProxyCIDRsForTest() {
	trustedOverride = nil
}

func TestGetIPAddress(t *testing.T) {
	defer clearTrustedProxyCIDRsForTest()

	// Untrusted remote: X-Forwarded-For must be ignored
	setTrustedProxyCIDRsForTest([]string{"127.0.0.0/8"})
	req := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4"}},
		RemoteAddr: "5.6.7.8:1234",
	}
	if ip := GetIPAddress(req); ip != "5.6.7.8" {
		t.Errorf("untrusted remote: expected 5.6.7.8 (ignore X-Forwarded-For), got %s", ip)
	}

	// Trusted remote: use the X-Forwarded-For value
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if ip := GetIPAddress(req); ip != "1.2.3.4" {
		t.Errorf("trusted remote: expected 1.2.3.4, got %s", ip)
	}

	// No header: use RemoteAddr
	req.Header.Del("X-Forwarded-For")
	if ip := GetIPAddress(req); ip != "127.0.0.1" {
		t.Errorf("no X-Forwarded-For: expected 127.0.0.1, got %s", ip)
	}

	// nginx appends the peer it saw to what the client sent: the client's own
	// entries are on the left and are not believed.
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 192.168.1.1")
	if ip := GetIPAddress(req); ip != "192.168.1.1" {
		t.Errorf("forged leftmost entry: expected the proxy's 192.168.1.1, got %s", ip)
	}

	// Trusted proxies in a chain are skipped, from the right.
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 192.168.1.1, 127.0.0.5")
	if ip := GetIPAddress(req); ip != "192.168.1.1" {
		t.Errorf("trusted hop on the right: expected 192.168.1.1, got %s", ip)
	}

	// Every hop trusted: the leftmost is the client.
	req.Header.Set("X-Forwarded-For", "127.0.0.9, 127.0.0.5")
	if ip := GetIPAddress(req); ip != "127.0.0.9" {
		t.Errorf("all hops trusted: expected 127.0.0.9, got %s", ip)
	}

	// A port is dropped, and an entry that is not an address stops the walk.
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 192.168.1.1:5555")
	if ip := GetIPAddress(req); ip != "192.168.1.1" {
		t.Errorf("hop with a port: expected 192.168.1.1, got %s", ip)
	}
	req.Header.Set("X-Forwarded-For", "6.6.6.6, not-an-ip")
	if ip := GetIPAddress(req); ip != "127.0.0.1" {
		t.Errorf("garbage hop: expected the connection's 127.0.0.1, got %s", ip)
	}
}

func TestToPgInet(t *testing.T) {
	ipStr := "192.168.0.1"
	inet, err := toPgInet(ipStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := inet.Value()
	if err != nil {
		t.Fatalf("failed to get value from Inet: %v", err)
	}

	// pqtype.Inet always adds /32 for IPv4
	expected := ipStr + "/32"
	if val != expected {
		t.Errorf("expected %s, got %s", expected, val)
	}
}

func TestSameSubnet(t *testing.T) {
	ip1 := net.ParseIP("192.168.1.10")
	ip2 := "192.168.1.20"
	ip3 := "192.168.2.30"

	if !SameSubnet(ip1, ip2) {
		t.Errorf("expected ip1 and ip2 to be same subnet")
	}
	if SameSubnet(ip1, ip3) {
		t.Errorf("expected ip1 and ip3 to NOT be same subnet")
	}
}

func TestSendSecurityAlert(t *testing.T) {
	userID := uuid.New()
	oldIP := net.ParseIP("1.2.3.4")
	newIP := "5.6.7.8"

	// just ensure it doesn't panic (stub)
	SendSecurityAlert(userID, oldIP, newIP)
}

func TestGenerateRandomString(t *testing.T) {
	s := GenerateRandomString(32)
	if len(s) != 32 {
		t.Errorf("expected length 32, got %d", len(s))
	}

	s2 := GenerateRandomString(32)
	if s == s2 {
		t.Errorf("expected two random strings to be different")
	}

	for _, r := range s {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", r) {
			t.Errorf("invalid character in generated string: %c", r)
		}
	}
}
