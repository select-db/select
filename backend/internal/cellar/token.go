package cellar

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"backend/internal/auth"
)

// PermHeader carries the caller's permission entries for the database, as the
// JSON bytes whose hash the token's Perm claim holds.
const PermHeader = "X-Cellar-Perms"

// A token lives tokenTTL and is reused for reuseFor: each KMS sign is a remote
// call, and every reused token still has 10s left when it reaches the cellar.
const (
	tokenTTL = 60 * time.Second
	reuseFor = 50 * time.Second
)

// PermHash is the Perm claim for a permission payload.
func PermHash(perms []byte) string {
	sum := sha256.Sum256(perms)
	return hex.EncodeToString(sum[:])
}

type tokenKey struct {
	db, ws, cel, perm string
	max               int64
	pitr              int
}

type cachedToken struct {
	token   string
	expires time.Time
}

// Tokens signs cellar tokens for the backend and reuses each one for reuseFor.
type Tokens struct {
	mu    sync.Mutex
	cache map[tokenKey]cachedToken
	sign  func(auth.CellarClaims, time.Duration) (string, error)
}

func NewTokens() *Tokens {
	return &Tokens{cache: map[tokenKey]cachedToken{}, sign: auth.SignCellarToken}
}

// Token returns a signed token for c, reusing a recent one when nothing differs.
func (t *Tokens) Token(c auth.CellarClaims) (string, error) {
	k := tokenKey{db: c.DB, ws: c.WS, cel: c.Cel, perm: c.Perm, max: c.Max, pitr: c.PITR}
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()
	if hit, ok := t.cache[k]; ok && now.Before(hit.expires) {
		return hit.token, nil
	}
	tok, err := t.sign(c, tokenTTL)
	if err != nil {
		return "", err
	}
	for key, v := range t.cache {
		if !now.Before(v.expires) {
			delete(t.cache, key)
		}
	}
	t.cache[k] = cachedToken{token: tok, expires: now.Add(reuseFor)}
	return tok, nil
}

// errBadToken is all a caller learns. A refusal means a backend bug or an
// attack, never a user mistake, so the reason goes to the log.
var errBadToken = errors.New("cellar: request not authorized")

// Authorize checks that r carries a valid token for database db on cellar
// cellarID, and that the permission bytes it sent are the ones that were signed.
func Authorize(r *http.Request, pub *rsa.PublicKey, cellarID, db string, perms []byte) (*auth.CellarClaims, error) {
	c, err := auth.ValidateCellarToken(auth.ExtractBearerToken(r.Header.Get("Authorization")), pub)
	switch {
	case err != nil:
		log.Printf("cellar: refused request for db %s: %v", db, err)
	case c.DB != db:
		log.Printf("cellar: refused request for db %s: token is for db %s", db, c.DB)
	case c.Cel != cellarID:
		log.Printf("cellar: refused request for db %s: token is for cellar %s", db, c.Cel)
	case c.Perm != PermHash(perms):
		log.Printf("cellar: refused request for db %s: permissions do not match the token", db)
	default:
		return c, nil
	}
	return nil, errBadToken
}
