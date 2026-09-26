package middlewares

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// InFlight runs at most limit requests per key, both from key(r); the rest
// wait, and get 408 if their deadline passes first. A key whose limit changes
// gets new slots while its old ones drain. Keys live as long as the process:
// keep them bounded.
func InFlight(key func(*http.Request) (string, int)) func(http.Handler) http.Handler {
	type slotKey struct {
		key   string
		limit int
	}
	var mu sync.Mutex
	slots := map[slotKey]chan struct{}{}
	slotsFor := func(k string, limit int) chan struct{} {
		mu.Lock()
		defer mu.Unlock()
		sk := slotKey{k, limit}
		s, ok := slots[sk]
		if !ok {
			s = make(chan struct{}, limit)
			slots[sk] = s
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
