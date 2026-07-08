package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// API keys authenticate automated clients to the same principal pipeline as a
// user JWT. Only a SHA-256 of the plaintext is stored: the secret tail is 40
// chars of CSPRNG, so a plain hash is as strong as a keyed MAC and a DB leak
// yields no usable keys. A domain tag separates these hashes from refresh-token
// hashes so neither can be replayed as the other.
const (
	APIKeyScheme   = "slct_"           // bearer prefix; branches the auth path before any DB hit
	apiKeyPrefixLn = 12               // stored in auth.api_key.prefix, used for lookup
	apiKeySecretLn = 40               // unguessable tail
	apiKeyHashTag  = "sdb-api-key-v1" // domain separation from refresh tokens
)

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
	h := sha256.New()
	h.Write([]byte(apiKeyHashTag))
	h.Write([]byte{0})
	h.Write([]byte(plaintext))
	return hex.EncodeToString(h.Sum(nil))
}

// ParseAPIKeyPrefix returns the lookup handle from an slct_<prefix>_<secret>
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
