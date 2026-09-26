package connect

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/selectDb/dialect/core"
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

// guardedDial is the dialer every guarded open goes through.
var guardedDial core.DialFunc = guardedNetDialer(isBlockedIP).DialContext
