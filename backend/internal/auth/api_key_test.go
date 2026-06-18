package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKeyRoundtrip(t *testing.T) {
	plaintext, prefix, hash := GenerateAPIKey()

	if !IsAPIKey(plaintext) {
		t.Fatalf("generated key %q lacks scheme", plaintext)
	}
	if !strings.Contains(plaintext, prefix) {
		t.Fatalf("plaintext %q does not contain prefix %q", plaintext, prefix)
	}

	gotPrefix, ok := ParseAPIKeyPrefix(plaintext)
	if !ok || gotPrefix != prefix {
		t.Fatalf("ParseAPIKeyPrefix = (%q, %v), want (%q, true)", gotPrefix, ok, prefix)
	}
	if !VerifyAPIKey(plaintext, hash) {
		t.Fatal("VerifyAPIKey rejected a valid key")
	}
	if VerifyAPIKey(plaintext+"x", hash) {
		t.Fatal("VerifyAPIKey accepted a tampered key")
	}
}

// TestHashDomainSeparation ensures an API-key hash and a refresh-token hash of
// the same input differ, so a hash captured in one context cannot be replayed
// as the other.
func TestHashDomainSeparation(t *testing.T) {
	const in = "same-input"
	if HashAPIKey(in) == HashRefreshToken(in, "") {
		t.Fatal("API-key and refresh-token hashes collide; domain tags not applied")
	}
}

func TestGenerateAPIKeyUnique(t *testing.T) {
	a, _, _ := GenerateAPIKey()
	b, _, _ := GenerateAPIKey()
	if a == b {
		t.Fatal("two generated keys collided")
	}
}

func TestParseAPIKeyPrefixRejectsMalformed(t *testing.T) {
	for _, tok := range []string{"", "nope", "sdb_", "sdb_only", "sdb_pfx_", "Bearer sdb_a_b"} {
		if _, ok := ParseAPIKeyPrefix(tok); ok {
			t.Fatalf("ParseAPIKeyPrefix(%q) = true, want false", tok)
		}
	}
}

func TestIsAPIKey(t *testing.T) {
	if IsAPIKey("eyJhbGciOi...") {
		t.Fatal("JWT misclassified as API key")
	}
	if !IsAPIKey("sdb_abc_def") {
		t.Fatal("API key not recognised")
	}
}
