package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CellarAudience is the audience of tokens the backend signs for a cellar. It
// differs from Audience, so neither kind of token passes for the other.
const CellarAudience = "selectdb-cellar"

// SignCellarToken signs a token that proves the backend to a cellar for ttl,
// with the same key as user tokens.
func SignCellarToken(ttl time.Duration) (string, error) {
	signer, err := getSigner()
	if err != nil {
		return "", fmt.Errorf("unable to sign cellar token: %w", err)
	}
	now := time.Now()
	c := jwt.RegisteredClaims{
		Issuer:    Issuer,
		Audience:  jwt.ClaimStrings{CellarAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwtSigningMethod, c).SignedString(signer)
}

// ValidateCellarToken checks a cellar token against pub, the only key a cellar
// holds.
func ValidateCellarToken(tokenStr string, pub *rsa.PublicKey) error {
	if pub == nil {
		return errors.New("cellar token: no public key")
	}
	_, err := parseRS256(tokenStr, &jwt.RegisteredClaims{}, pub, CellarAudience, jwt.WithExpirationRequired())
	return err
}

// PublicKey is the key user and cellar tokens verify against, for a cellar
// running in the backend's own process.
func PublicKey() (*rsa.PublicKey, error) { return getPublicKey() }
