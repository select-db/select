package cellar

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"backend/internal/auth"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/toolkit/cache"
)

// PermHeader carries the caller's permission entries for the database, as
// base64url JSON; the token's Perm claim is the hash of that exact value.
const PermHeader = "X-Cellar-Perms"

// EncodePerms is the PermHeader value for entries.
func EncodePerms(entries []core.PermissionEntry) (string, error) {
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PermsFrom decodes the PermHeader of a request Authenticate admitted.
func PermsFrom(r *http.Request) ([]core.PermissionEntry, error) {
	b, err := base64.RawURLEncoding.DecodeString(r.Header.Get(PermHeader))
	if err != nil {
		return nil, err
	}
	var entries []core.PermissionEntry
	return entries, json.Unmarshal(b, &entries)
}

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

// Tokens signs cellar tokens for the backend, reusing each for reuseFor.
type Tokens struct {
	cache *cache.Cache
	sign  func(auth.CellarGrant, time.Duration) (string, error)
	now   func() time.Time
}

func NewTokens() *Tokens {
	return &Tokens{
		cache: cache.New(cache.Options{TTL: reuseFor, MaxEntries: 10_000}),
		sign:  auth.SignCellarToken,
		now:   time.Now,
	}
}

// signedToken carries its own deadline: the cache's TTL restarts on every hit,
// so on its own it would keep serving a token past its expiry.
type signedToken struct {
	token      string
	reuseUntil time.Time
}

// Token returns a signed token for g, reusing a recent one for the same grant.
func (t *Tokens) Token(g auth.CellarGrant) (string, error) {
	b, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	key := string(b)
	create := func() (any, error) {
		reuseUntil := t.now().Add(reuseFor) // taken before signing, so a slow sign only shortens reuse
		tok, err := t.sign(g, tokenTTL)
		return signedToken{token: tok, reuseUntil: reuseUntil}, err
	}
	v, err := t.cache.GetOrCreate(key, create)
	if err == nil && !t.now().Before(v.(signedToken).reuseUntil) {
		t.cache.Delete(key)
		v, err = t.cache.GetOrCreate(key, create)
	}
	if err != nil {
		return "", err
	}
	return v.(signedToken).token, nil
}

type grantKey struct{}

// GrantFrom returns the grant Authenticate verified for this request.
func GrantFrom(ctx context.Context) auth.CellarGrant {
	g, _ := ctx.Value(grantKey{}).(auth.CellarGrant)
	return g
}

// Authenticate admits only a valid token for this cellar and the path's db,
// signed over the PermHeader it carries. Refusals are 500s; the reason is logged.
func Authenticate(pub *rsa.PublicKey, cellarID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			db := r.PathValue("id")
			g, err := auth.ValidateCellarToken(auth.ExtractBearerToken(r.Header.Get("Authorization")), pub)
			var reason string
			switch {
			case err != nil:
				reason = err.Error()
			case g.DB != db:
				reason = "token is for db " + g.DB
			case g.CellarID != cellarID:
				reason = "token is for cellar " + g.CellarID
			case g.PermSHA256 != PermHash([]byte(r.Header.Get(PermHeader))):
				reason = "permissions do not match the token"
			}
			if reason != "" {
				log.Printf("cellar: refused request for db %s: %s", db, reason)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), grantKey{}, g)))
		})
	}
}
