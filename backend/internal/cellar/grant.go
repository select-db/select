package cellar

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"

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
	Slots    int                    `json:"slots"` // statements the workspace may run at once
	Perms    []core.PermissionEntry `json:"perms"` // the caller's entries for DB
}

// GrantHeader carries a Grant as base64url JSON.
const GrantHeader = "X-Cellar-Grant"

// Audience is the audience of the backend's tokens for a cellar, so a user
// token never opens one.
const Audience = "selectdb-cellar"

// Encode is g as the GrantHeader value.
func (g Grant) Encode() (string, error) {
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
			db := r.PathValue("id")
			refuse := func(err error) {
				log.Printf("cellar: refused request for db %s: %v", db, err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			if _, _, err := auth.Verify(auth.ExtractBearerToken(r.Header.Get("Authorization")), pub, Audience); err != nil {
				refuse(err)
				return
			}
			g, err := decodeGrant(r.Header.Get(GrantHeader))
			if err != nil {
				refuse(err)
				return
			}
			if g.CellarID != cellarID || cellarID == "" {
				// The db moved to another cellar: never let two write it.
				refuse(errors.New("grant is for cellar " + g.CellarID))
				return
			}
			g.DB = db
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), grantKey{}, g)))
		})
	}
}
