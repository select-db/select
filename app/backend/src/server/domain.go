package server

import (
	"strings"
)

// DomainToBaseURL returns the full API base URL for the given domain.
// Uses https for all domains except localhost (http).
func DomainToBaseURL(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	if isLocalhost(domain) {
		return "http://" + domain
	}
	return "https://" + domain
}

func isLocalhost(domain string) bool {
	lower := strings.ToLower(domain)
	return lower == "localhost" || strings.HasPrefix(lower, "localhost:")
}
