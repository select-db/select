package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

// API keys authenticate automated clients to the same principal pipeline as a
// user JWT. Only an HMAC of the plaintext is stored. No dedicated secret: keys
// reuse the signing secret under a domain tag, disjoint from refresh-token
// hashing, so they cannot be cross-replayed
const (
	APIKeyScheme   = "sdb_"           // bearer prefix; branches the auth path before any DB hit
	apiKeyPrefixLn = 12               // stored in auth.api_key.prefix, used for lookup
	apiKeySecretLn = 40               // unguessable tail
	apiKeyHMACTag  = "sdb-api-key-v1" // HMAC domain separation from refresh tokens
)

// ValidateSigningSecret fails at boot if the shared signing secret is unset,
// so CI catches it instead of a first-request panic
func ValidateSigningSecret() error {
	if os.Getenv("REFRESH_TOKEN_SECRET") == "" {
		return errors.New("REFRESH_TOKEN_SECRET is not set")
	}
	return nil
}

func IsAPIKey(token string) bool {
	return strings.HasPrefix(token, APIKeyScheme)
}

// GenerateAPIKey returns the plaintext (shown once, never stored), its prefix,
// and its hash
func GenerateAPIKey() (plaintext, prefix, hash string) {
	prefix = GenerateRandomString(apiKeyPrefixLn)
	secret := GenerateRandomString(apiKeySecretLn)
	plaintext = APIKeyScheme + prefix + "_" + secret
	return plaintext, prefix, HashAPIKey(plaintext)
}

func HashAPIKey(plaintext string) string {
	h := hmac.New(sha256.New, GetRefreshTokenSecret())
	h.Write([]byte(apiKeyHMACTag))
	h.Write([]byte{0})
	h.Write([]byte(plaintext))
	return hex.EncodeToString(h.Sum(nil))
}

// ParseAPIKeyPrefix returns the lookup handle from an sdb_<prefix>_<secret>
// token; ok is false for any other shape
func ParseAPIKeyPrefix(token string) (prefix string, ok bool) {
	if !IsAPIKey(token) {
		return "", false
	}
	rest := strings.TrimPrefix(token, APIKeyScheme)
	i := strings.IndexByte(rest, '_')
	if i <= 0 || i == len(rest)-1 {
		return "", false
	}
	return rest[:i], true
}

func VerifyAPIKey(plaintext, storedHash string) bool {
	return hmac.Equal([]byte(HashAPIKey(plaintext)), []byte(storedHash))
}
