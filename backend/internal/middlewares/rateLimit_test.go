package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestRateKeyPrefersTheWorkspace(t *testing.T) {
	request := func(workspaceID, userID, forwardedFor string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "5.6.7.8:1234"
		if forwardedFor != "" {
			r.Header.Set("X-Forwarded-For", forwardedFor)
		}
		ctx := r.Context()
		if workspaceID != "" {
			ctx = context.WithValue(ctx, ctxWorkspaceID, workspaceID)
		}
		if userID != "" {
			ctx = context.WithValue(ctx, userIDKey, userID)
		}
		return r.WithContext(ctx)
	}

	inWorkspace := rateKey(request("ws-1", "alice", ""))
	if inWorkspace != "w:ws-1" {
		t.Fatalf("a request for a workspace is counted against it, got %q", inWorkspace)
	}
	for name, other := range map[string]*http.Request{
		"another user":    request("ws-1", "bob", ""),
		"another key":     request("ws-1", "api-key-7", ""),
		"another address": request("ws-1", "alice", "9.9.9.9"),
	} {
		if got := rateKey(other); got != inWorkspace {
			t.Errorf("%s must share the workspace's allowance, got %q", name, got)
		}
	}
	if got := rateKey(request("ws-2", "alice", "")); got == inWorkspace {
		t.Error("another workspace has an allowance of its own")
	}
	if got := rateKey(request("", "alice", "")); got != "u:alice" {
		t.Errorf("without a workspace the user is counted, got %q", got)
	}
	if got := rateKey(request("", "", "")); got != "ip:5.6.7.8" {
		t.Errorf("without a workspace or a user the address is counted, got %q", got)
	}
}
