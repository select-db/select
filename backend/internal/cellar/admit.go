package cellar

import (
	"context"
	"crypto/rsa"
	"net/http"
	"time"

	"backend/internal/middlewares"
)

// Every statement gets at most statementTimeout, including its wait for one of
// the workspace's slots. The cap is never read from the request.
const statementTimeout = 60 * time.Second

// Admit wraps every cellar route: authenticate, cap the time, take a slot.
func Admit(pub *rsa.PublicKey, cellarID string) func(http.Handler) http.Handler {
	authenticate := Authenticate(pub, cellarID)
	slot := middlewares.InFlight(func(r *http.Request) (string, int) {
		grant := GrantFrom(r.Context())
		return grant.WorkspaceID, grant.MaxInFlight
	})
	return func(next http.Handler) http.Handler {
		limited := slot(next)
		timed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), statementTimeout)
			defer cancel()
			limited.ServeHTTP(w, r.WithContext(ctx))
		})
		return authenticate(timed)
	}
}
