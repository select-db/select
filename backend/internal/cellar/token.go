package cellar

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"backend/internal/auth"

	"github.com/selectDb/toolkit/cache"
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

// Tokens signs cellar tokens for the backend, reusing each for reuseFor.
type Tokens struct {
	cache *cache.Cache
	sign  func(auth.CellarGrant, time.Duration) (string, error)
}

func NewTokens() *Tokens {
	return &Tokens{
		cache: cache.New(cache.Options{TTL: reuseFor, MaxEntries: 10_000}),
		sign:  auth.SignCellarToken,
	}
}

// Token returns a signed token for g, reusing a recent one for the same grant.
func (t *Tokens) Token(g auth.CellarGrant) (string, error) {
	key, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	tok, err := t.cache.GetOrCreate(string(key), func() (any, error) { return t.sign(g, tokenTTL) })
	if err != nil {
		return "", err
	}
	return tok.(string), nil
}

type grantKey struct{}

// GrantFrom returns the grant Authenticate verified for this request.
func GrantFrom(ctx context.Context) auth.CellarGrant {
	g, _ := ctx.Value(grantKey{}).(auth.CellarGrant)
	return g
}

// Authenticate admits a request only with a valid token for this cellar and the
// database in the path, signed over the PermHeader bytes it carries. A refusal
// means a backend bug or an attack, so the caller learns nothing and the reason
// goes to the log.
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
			case g.Cel != cellarID:
				reason = "token is for cellar " + g.Cel
			case g.Perm != PermHash([]byte(r.Header.Get(PermHeader))):
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
