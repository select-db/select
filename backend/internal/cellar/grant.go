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
	DatasourceID string                 `json:"-"` // the path's id
	WorkspaceID  string                 `json:"workspace_id"`
	CellarID     string                 `json:"cellar_id"`
	MaxBytes     int64                  `json:"max_bytes"`
	MaxInFlight  int                    `json:"max_in_flight"` // statements the workspace may run at once
	Permissions  []core.PermissionEntry `json:"permissions"`   // the caller's entries for the datasource
}

// GrantHeader carries a Grant as base64url JSON.
const GrantHeader = "X-Cellar-Grant"

// Audience is the audience of the backend's tokens for a cellar, so a user
// token never opens one.
const Audience = "selectdb-cellar"

// Encode is the grant as the GrantHeader value.
func (grant Grant) Encode() (string, error) {
	b, err := json.Marshal(grant)
	return base64.RawURLEncoding.EncodeToString(b), err
}

func decodeGrant(v string) (Grant, error) {
	var grant Grant
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err == nil {
		err = json.Unmarshal(b, &grant)
	}
	return grant, err
}

type grantKey struct{}

// GrantFrom returns the grant Authenticate admitted for this request.
func GrantFrom(ctx context.Context) Grant {
	grant, _ := ctx.Value(grantKey{}).(Grant)
	return grant
}

// Authenticate admits only the backend, with a grant for this cellar. Refusals
// are 500s; the reason is logged.
func Authenticate(pub *rsa.PublicKey, cellarID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			datasourceID := r.PathValue("id")
			refuse := func(err error) {
				log.Printf("cellar: refused request for datasource %s: %v", datasourceID, err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			if _, _, err := auth.Verify(auth.ExtractBearerToken(r.Header.Get("Authorization")), pub, Audience); err != nil {
				refuse(err)
				return
			}
			grant, err := decodeGrant(r.Header.Get(GrantHeader))
			if err != nil {
				refuse(err)
				return
			}
			if grant.CellarID != cellarID || cellarID == "" {
				// The datasource moved to another cellar: never let two write it.
				refuse(errors.New("grant is for cellar " + grant.CellarID))
				return
			}
			grant.DatasourceID = datasourceID
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), grantKey{}, grant)))
		})
	}
}
