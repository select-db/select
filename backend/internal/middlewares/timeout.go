package middlewares

import (
	"context"
	"net/http"
	"time"
)

// Timeout cancels the request's context after d, time spent waiting in later
// middlewares included. It streams, unlike http.TimeoutHandler, which buffers
// the whole response.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
