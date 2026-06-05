package auth

import (
	"os"
	"strings"
)

// isEmailAllowed returns true if the email is permitted to sign in.
//
// Production (APP_ENV != "staging") is intentionally open: any GitHub user
// may sign in. The allowlist is a staging-only gate.
//
// In staging the gate fails closed: if ALLOWED_USERS is unset or empty the
// function denies every sign-in, so a misconfigured staging deploy never
// silently becomes open. ALLOWED_USERS is a comma-separated list of emails
// or domain suffixes (e.g. "@acme.com").
func isEmailAllowed(email string) bool {
	if os.Getenv("APP_ENV") != "staging" {
		return true
	}
	raw := os.Getenv("ALLOWED_USERS")
	if raw == "" {
		return false
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	for _, entry := range strings.Split(raw, ",") {
		suffix := strings.ToLower(strings.TrimSpace(entry))
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(email, suffix) {
			return true
		}
	}
	return false
}
