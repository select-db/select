package auth

import (
	"backend/db/db_types"
	"net"
	"net/http"
	"strings"
	"testing"
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

	// Trusted remote: use first X-Forwarded-For value
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

	// Multiple proxies: leftmost is client
	req.Header.Set("X-Forwarded-For", " 192.168.1.1, 10.0.0.1 ")
	if ip := GetIPAddress(req); ip != "192.168.1.1" {
		t.Errorf("multiple X-Forwarded-For: expected 192.168.1.1, got %s", ip)
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
	userID := db_types.JSONNullUUID{}
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
