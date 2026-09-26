package cellar

import (
	"context"
	"crypto/rsa"
	"errors"
	"log"
	"net/http"

	"backend/internal/auth"
)

type grantKey struct{}

// GetGrant returns the grant Authenticated admitted for this request.
func GetGrant(r *http.Request) Grant {
	grant, _ := r.Context().Value(grantKey{}).(Grant)
	return grant
}

// Authenticated admits only the backend, with a grant for this cellar.
// Refusals are 500s; the reason is logged.
func Authenticated(pub *rsa.PublicKey, cellarID string) func(http.Handler) http.Handler {
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
