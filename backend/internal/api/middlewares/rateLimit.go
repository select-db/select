package middlewares

import (
	"net/http"
	"os"
	"sync"
	"time"

	auth "backend/internal/api/auth"
)

// Per-minute token bucket. Per-route limit declared in main.go. Keyed by
// authenticated user when known, else client IP.

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type limiterStore struct {
	mu        sync.Mutex
	perMinute int
	buckets   map[string]*tokenBucket
	lastSweep time.Time
}

func newLimiterStore(perMinute int) *limiterStore {
	// lastSweep zero: sweeps anchor to request time, not wall clock.
	return &limiterStore{
		perMinute: perMinute,
		buckets:   make(map[string]*tokenBucket),
	}
}

func (s *limiterStore) allow(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweep(now)

	limit := float64(s.perMinute)
	b, ok := s.buckets[key]
	if !ok {
		s.buckets[key] = &tokenBucket{tokens: limit - 1, last: now}
		return true
	}

	b.tokens += now.Sub(b.last).Minutes() * limit
	if b.tokens > limit {
		b.tokens = limit
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sweep drops idle buckets. Called under s.mu (no per-route goroutine).
func (s *limiterStore) sweep(now time.Time) {
	if now.Sub(s.lastSweep) < 10*time.Minute {
		return
	}
	cutoff := now.Add(-15 * time.Minute)
	for k, b := range s.buckets {
		if b.last.Before(cutoff) {
			delete(s.buckets, k)
		}
	}
	s.lastSweep = now
}

// rateKey: auth context user, else verified-token user, else client IP.
func rateKey(r *http.Request) string {
	if uid, ok := r.Context().Value(userIDKey).(string); ok && uid != "" {
		return "u:" + uid
	}
	if tok := auth.ExtractBearerToken(r.Header.Get("Authorization")); tok != "" {
		if _, claims, err := auth.ValidateJWT(tok); err == nil && claims != nil && claims.UserID != "" {
			return "u:" + claims.UserID
		}
	}
	return "ip:" + auth.GetIPAddress(r)
}

// RateLimit: per-caller perMinute cap, declared per route in main.go.
// RATE_LIMIT_DISABLED=true bypasses (local dev / tests).
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	if os.Getenv("RATE_LIMIT_DISABLED") == "true" {
		return func(next http.Handler) http.Handler { return next }
	}

	store := newLimiterStore(perMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !store.allow(rateKey(r), time.Now()) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
