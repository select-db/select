package apikey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend/internal/middlewares"
)

func ptr(s string) *string { return &s }

func TestResolveExpiry(t *testing.T) {
	if v, err := resolveExpiry(nil); v != nil || err != nil {
		t.Fatalf("nil = (%v,%v), want (nil,nil)", v, err)
	}
	if v, err := resolveExpiry(ptr("")); v != nil || err != nil {
		t.Fatalf("empty = (%v,%v), want (nil,nil)", v, err)
	}
	if _, err := resolveExpiry(ptr("not-a-date")); err != errBadExpiry {
		t.Fatalf("garbage err = %v, want errBadExpiry", err)
	}
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	if _, err := resolveExpiry(&past); err != errBadExpiry {
		t.Fatalf("past err = %v, want errBadExpiry", err)
	}
	tooFar := time.Now().Add(maxExpiry + 48*time.Hour).Format(time.RFC3339)
	if _, err := resolveExpiry(&tooFar); err != errExpiryTooFar {
		t.Fatalf("too-far err = %v, want errExpiryTooFar", err)
	}
	ok := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	v, err := resolveExpiry(&ok)
	if err != nil || v == nil {
		t.Fatalf("valid = (%v,%v), want a time", v, err)
	}
}

func TestGuardRejectsAPIKeyPrincipal(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/apikey/list", strings.NewReader(`{"workspace_id":"w"}`))
	req = req.WithContext(middlewares.ContextWithAPIKeyPrincipal(context.Background(), "key-1", "ws-1", nil))
	rr := httptest.NewRecorder()

	ListHandler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "api keys cannot manage api keys") {
		t.Fatalf("body = %q", rr.Body.String())
	}
}

func TestGuardRejectsGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/apikey/list", nil)
	rr := httptest.NewRecorder()
	ListHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}
