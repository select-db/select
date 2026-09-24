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

// CellarClaims authorize one request on one managed database. The cellar knows
// no users and reads no Postgres: everything it enforces arrives here.
type CellarClaims struct {
	DB   string `json:"db"`   // datasource id
	WS   string `json:"ws"`   // workspace id
	Cel  string `json:"cel"`  // cellar id; a cellar refuses tokens for another
	Max  int64  `json:"max"`  // size cap of the database, in bytes
	PITR int    `json:"pitr"` // point-in-time window, in days
	Perm string `json:"perm"` // sha256 of the permission entries sent alongside
	jwt.RegisteredClaims
}

// SignCellarToken signs c for ttl with the same key as user tokens.
func SignCellarToken(c CellarClaims, ttl time.Duration) (string, error) {
	signer, err := getSigner()
	if err != nil {
		return "", fmt.Errorf("unable to sign cellar token: %w", err)
	}
	now := time.Now()
	c.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    Issuer,
		Audience:  jwt.ClaimStrings{CellarAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwtSigningMethod, c).SignedString(signer)
}

// ValidateCellarToken checks the signature, issuer, audience and expiry of a
// cellar token against pub, the only key a cellar holds.
func ValidateCellarToken(tokenStr string, pub *rsa.PublicKey) (*CellarClaims, error) {
	if pub == nil {
		return nil, errors.New("cellar token: no public key")
	}
	claims := &CellarClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pub, nil
	}, jwt.WithAudience(CellarAudience), jwt.WithIssuer(Issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// PublicKey is the key user and cellar tokens verify against, for a cellar
// running in the backend's own process.
func PublicKey() (*rsa.PublicKey, error) { return getPublicKey() }
