package toolkit

import "testing"

func TestRequireLoopbackOnlyAcceptsAddressesTheNetworkCannotReach(t *testing.T) {
	for _, addr := range []string{"localhost:6060", "127.0.0.1:6060", "127.1.2.3:1", "[::1]:6060"} {
		if err := requireLoopback(addr); err != nil {
			t.Errorf("%s should be accepted: %v", addr, err)
		}
	}
	for _, addr := range []string{":6060", "0.0.0.0:6060", "[::]:6060", "10.0.0.5:6060", "40.160.11.126:6060", "example.com:6060", "6060", ""} {
		if err := requireLoopback(addr); err == nil {
			t.Errorf("%s must be refused", addr)
		}
	}
}
