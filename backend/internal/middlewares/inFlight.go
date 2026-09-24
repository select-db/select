package middlewares

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// InFlight lets at most limit requests per key run at once. Others wait for a
// slot rather than fail; a wait that outlives the request's deadline gets 408.
func InFlight(limit int, key func(*http.Request) string) func(http.Handler) http.Handler {
	g := &slotGroups{groups: map[string]*slotGroup{}}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k := key(r)
			slots := g.join(k, limit)
			defer g.leave(k)

			select {
			case slots <- struct{}{}:
			case <-r.Context().Done():
				if errors.Is(r.Context().Err(), context.DeadlineExceeded) {
					http.Error(w, "timed out waiting for a free slot", http.StatusRequestTimeout)
				}
				return
			}
			defer func() { <-slots }()
			next.ServeHTTP(w, r)
		})
	}
}

// slotGroups keeps one semaphore per key, dropped once no request holds or
// waits on it, so idle keys cost nothing.
type slotGroups struct {
	mu     sync.Mutex
	groups map[string]*slotGroup
}

type slotGroup struct {
	slots chan struct{}
	users int
}

func (g *slotGroups) join(k string, limit int) chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.groups[k]
	if !ok {
		s = &slotGroup{slots: make(chan struct{}, limit)}
		g.groups[k] = s
	}
	s.users++
	return s.slots
}

func (g *slotGroups) leave(k string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if s := g.groups[k]; s != nil {
		s.users--
		if s.users == 0 {
			delete(g.groups, k)
		}
	}
}
