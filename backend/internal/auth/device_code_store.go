package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/selectDb/toolkit/cache"
)

// issuedDeviceCodes records codes this server issued so /auth/access-token
// can't be an open proxy forwarding arbitrary codes to GitHub. In-memory:
// single backend process behind nginx. 15m TTL covers GitHub's device-code
// lifetime; MaxEntries bounds memory.
var issuedDeviceCodes = cache.New(cache.Options{MaxEntries: 10_000, TTL: 15 * time.Minute})

func deviceCodeKey(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func rememberDeviceCode(code string) {
	if code != "" {
		issuedDeviceCodes.Set(deviceCodeKey(code), struct{}{})
	}
}

func deviceCodeKnown(code string) bool {
	if code == "" {
		return false
	}
	_, ok := issuedDeviceCodes.Get(deviceCodeKey(code))
	return ok
}
