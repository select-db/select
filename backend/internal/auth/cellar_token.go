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

// CellarGrant is what one cellar request may do. The cellar knows no users and
// reads no Postgres, so everything it enforces is here.
type CellarGrant struct {
	DB         string `json:"db"`
	WS         string `json:"ws"`
	CellarID   string `json:"cel"`
	MaxBytes   int64  `json:"max"`
	PITRDays   int    `json:"pitr"`
	PermSHA256 string `json:"perm"` // over the PermHeader bytes sent alongside
}

type CellarClaims struct {
	CellarGrant
	jwt.RegisteredClaims
}

// SignCellarToken signs g for ttl with the same key as user tokens.
func SignCellarToken(g CellarGrant, ttl time.Duration) (string, error) {
	signer, err := getSigner()
	if err != nil {
		return "", fmt.Errorf("unable to sign cellar token: %w", err)
	}
	now := time.Now()
	c := CellarClaims{CellarGrant: g, RegisteredClaims: jwt.RegisteredClaims{
		Issuer:    Issuer,
		Audience:  jwt.ClaimStrings{CellarAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}}
	return jwt.NewWithClaims(jwtSigningMethod, c).SignedString(signer)
}

// ValidateCellarToken checks a cellar token against pub, the only key a cellar
// holds, and returns what it grants.
func ValidateCellarToken(tokenStr string, pub *rsa.PublicKey) (CellarGrant, error) {
	if pub == nil {
		return CellarGrant{}, errors.New("cellar token: no public key")
	}
	claims := &CellarClaims{}
	if _, err := parseRS256(tokenStr, claims, pub, CellarAudience, jwt.WithExpirationRequired()); err != nil {
		return CellarGrant{}, err
	}
	return claims.CellarGrant, nil
}

// PublicKey is the key user and cellar tokens verify against, for a cellar
// running in the backend's own process.
func PublicKey() (*rsa.PublicKey, error) { return getPublicKey() }
