package middlewares

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// InFlight lets at most limit requests per key run at once. Others wait for a
// slot rather than fail; a wait that outlives the request's deadline gets 408.
// Keys are kept for the life of the process, so they must be a bounded set,
// such as workspace ids.
func InFlight(limit int, key func(*http.Request) string) func(http.Handler) http.Handler {
	var mu sync.Mutex
	slots := map[string]chan struct{}{}
	slotsFor := func(k string) chan struct{} {
		mu.Lock()
		defer mu.Unlock()
		s, ok := slots[k]
		if !ok {
			s = make(chan struct{}, limit)
			slots[k] = s
		}
		return s
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := slotsFor(key(r))
			select {
			case s <- struct{}{}:
			case <-r.Context().Done():
				if errors.Is(r.Context().Err(), context.DeadlineExceeded) {
					http.Error(w, "timed out waiting for a free slot", http.StatusRequestTimeout)
				}
				return
			}
			defer func() { <-s }()
			next.ServeHTTP(w, r)
		})
	}
}
