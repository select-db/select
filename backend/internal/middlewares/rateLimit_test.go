package middlewares

import (
	"testing"
	"time"
)

func TestTokenBucketAllowsBurstThenBlocksThenRefills(t *testing.T) {
	store := newLimiterStore(3) // 3 requests/minute
	now := time.Unix(0, 0)

	for i := 0; i < 3; i++ {
		if !store.allow("k", now) {
			t.Fatalf("request %d within burst should be allowed", i)
		}
	}
	if store.allow("k", now) {
		t.Fatal("4th request in same instant should be blocked")
	}
	// One minute later the bucket has refilled to capacity.
	if !store.allow("k", now.Add(time.Minute)) {
		t.Fatal("request after a minute should be allowed")
	}
}

func TestKeysAreIsolated(t *testing.T) {
	store := newLimiterStore(1)
	now := time.Unix(0, 0)

	if !store.allow("a", now) {
		t.Fatal("first request for key a should pass")
	}
	if store.allow("a", now) {
		t.Fatal("second request for key a should be blocked")
	}
	if !store.allow("b", now) {
		t.Fatal("key b must not be affected by key a")
	}
}

func TestIdleBucketsAreSwept(t *testing.T) {
	store := newLimiterStore(1)
	start := time.Unix(0, 0)

	store.allow("stale", start)
	if _, ok := store.buckets["stale"]; !ok {
		t.Fatal("bucket should exist after first request")
	}

	// A later request triggers a sweep; the stale bucket is past the cutoff.
	store.allow("fresh", start.Add(30*time.Minute))
	if _, ok := store.buckets["stale"]; ok {
		t.Fatal("stale bucket should have been swept")
	}
}
