package cellar

import (
	"context"
	"crypto/rsa"
	"log"
	"net/http"
	"time"

	"backend/internal/middlewares"
)

// Every statement gets at most statementTimeout, including its wait for one of
// the workspace's inFlightPerWorkspace slots. Neither is read from the request.
const (
	statementTimeout     = 60 * time.Second
	inFlightPerWorkspace = 10
)

// Admit wraps every cellar route: authenticate, cap the time, take a slot, and
// log the time spent per workspace.
func Admit(pub *rsa.PublicKey, cellarID string) func(http.Handler) http.Handler {
	authenticate := Authenticate(pub, cellarID)
	slot := middlewares.InFlight(inFlightPerWorkspace, func(r *http.Request) string {
		return GrantFrom(r.Context()).WS
	})
	return func(next http.Handler) http.Handler {
		limited := slot(next)
		timed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), statementTimeout)
			defer cancel()
			start := time.Now()
			limited.ServeHTTP(w, r.WithContext(ctx))
			log.Printf("cellar: %s %s ws=%s took=%s", r.Method, r.URL.Path, GrantFrom(ctx).WS, time.Since(start))
		})
		return authenticate(timed)
	}
}
