package cellar

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"backend/internal/auth"

	"github.com/selectDb/dialect/core"
)

// Grant is what one cellar request may do. The cellar knows no users and reads
// no Postgres, so the backend sends all of it in GrantHeader.
type Grant struct {
	DB       string                 `json:"-"` // the path's id
	WS       string                 `json:"ws"`
	CellarID string                 `json:"cel"`
	MaxBytes int64                  `json:"max"`
	PITRDays int                    `json:"pitr"`
	Perms    []core.PermissionEntry `json:"perms"` // the caller's entries for DB
}

// GrantHeader carries a Grant as base64url JSON.
const GrantHeader = "X-Cellar-Grant"

func encodeGrant(g Grant) (string, error) {
	b, err := json.Marshal(g)
	return base64.RawURLEncoding.EncodeToString(b), err
}

func decodeGrant(v string) (Grant, error) {
	var g Grant
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err == nil {
		err = json.Unmarshal(b, &g)
	}
	return g, err
}

// A token lives tokenTTL and is reused for reuseFor: each KMS sign is a remote
// call, and every reused token still has 10s left when it reaches the cellar.
const (
	tokenTTL = 60 * time.Second
	reuseFor = 50 * time.Second
)

// Tokens signs the backend's cellar token, reusing it for reuseFor.
type Tokens struct {
	mu         sync.Mutex
	token      string
	reuseUntil time.Time
	sign       func(time.Duration) (string, error)
	now        func() time.Time
}

func NewTokens() *Tokens {
	return &Tokens{sign: auth.SignCellarToken, now: time.Now}
}

// Token returns a signed token. Callers wait on one sign rather than each
// making their own.
func (t *Tokens) Token() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.now().Before(t.reuseUntil) {
		return t.token, nil
	}
	reuseUntil := t.now().Add(reuseFor) // taken before signing, so a slow sign only shortens reuse
	tok, err := t.sign(tokenTTL)
	if err != nil {
		return "", err
	}
	t.token, t.reuseUntil = tok, reuseUntil
	return tok, nil
}

type grantKey struct{}

// GrantFrom returns the grant Authenticate admitted for this request.
func GrantFrom(ctx context.Context) Grant {
	g, _ := ctx.Value(grantKey{}).(Grant)
	return g
}

// Authenticate admits only the backend, with a grant for this cellar. Refusals
// are 500s; the reason is logged.
func Authenticate(pub *rsa.PublicKey, cellarID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var g Grant
			err := auth.ValidateCellarToken(auth.ExtractBearerToken(r.Header.Get("Authorization")), pub)
			if err == nil {
				g, err = decodeGrant(r.Header.Get(GrantHeader))
			}
			if err == nil && g.CellarID != cellarID {
				// The db moved to another cellar: never let two write it.
				err = errors.New("grant is for cellar " + g.CellarID)
			}
			g.DB = r.PathValue("id")
			if err != nil {
				log.Printf("cellar: refused request for db %s: %v", g.DB, err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), grantKey{}, g)))
		})
	}
}
