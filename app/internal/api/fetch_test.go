package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// sessionProbe swaps the credential accessors for in-memory ones and records
// whether the session was wiped, which is what the app reads as a logout.
type sessionProbe struct {
	access  string
	refresh string
	device  string

	accessErr  error
	refreshErr error
	deviceErr  error

	// Whether the session was wiped, per token.
	clearedAccess  bool
	clearedRefresh bool

	// When set, the keyring refuses to persist the rotated refresh token.
	saveRefreshErr error
}

// cleared reports whether the session was wiped, which is what the app reads
// as a logout.
func (p *sessionProbe) cleared() bool { return p.clearedAccess || p.clearedRefresh }

func (p *sessionProbe) install(t *testing.T) {
	t.Helper()

	origLoadAccess, origLoadRefresh, origLoadDevice := loadAccessTokenFunc, loadRefreshTokenFunc, loadDeviceIDFunc
	origSaveAccess, origSaveRefresh := saveAccessTokenFunc, saveRefreshTokenFunc
	origClearAccess, origClearRefresh := clearAccessTokenFunc, clearRefreshTokenFunc
	t.Cleanup(func() {
		loadAccessTokenFunc, loadRefreshTokenFunc, loadDeviceIDFunc = origLoadAccess, origLoadRefresh, origLoadDevice
		saveAccessTokenFunc, saveRefreshTokenFunc = origSaveAccess, origSaveRefresh
		clearAccessTokenFunc, clearRefreshTokenFunc = origClearAccess, origClearRefresh
	})

	loadAccessTokenFunc = func() (string, error) { return p.access, p.accessErr }
	loadRefreshTokenFunc = func() (string, error) { return p.refresh, p.refreshErr }
	loadDeviceIDFunc = func() (string, error) { return p.device, p.deviceErr }
	saveAccessTokenFunc = func(tok string) error { p.access = tok; return nil }
	saveRefreshTokenFunc = func(tok string) error {
		if p.saveRefreshErr != nil {
			return p.saveRefreshErr
		}
		p.refresh = tok
		return nil
	}
	clearAccessTokenFunc = func() error { p.clearedAccess = true; p.access = ""; return nil }
	clearRefreshTokenFunc = func() error {
		p.clearedRefresh = true
		p.refresh = ""
		forgetRefreshToken() // what the real ClearRefreshToken does
		return nil
	}

	refreshMu.Lock()
	refreshedAt = time.Time{}
	refreshErr = nil
	refreshMu.Unlock()
	forgetRefreshToken()
}

// newProbe returns a probe holding a complete, working credential set.
func newProbe(t *testing.T) *sessionProbe {
	p := &sessionProbe{access: "accessX", refresh: "refreshX", device: "deviceX"}
	p.install(t)
	return p
}

// serve points the client at a test server for the duration of the test.
func serve(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	os.Setenv("API_URL", srv.URL)
}

// rotatingAuthServer models the backend's refresh-token rotation: every refresh
// (request carrying X-Refresh-Token) consumes the presented token, deletes it,
// and issues a brand new access+refresh pair via X-New-* headers. A refresh that
// presents a token other than the current one returns 401 "Failed to refresh token",
// exactly the server behaviour in middlewares.Authenticated / auth.TryRefreshToken.
//
// To force a refresh on every Fetch (mirroring an already-expired 5-min access
// token), normal requests (no X-Refresh-Token) always return 401.
type rotatingAuthServer struct {
	mu             sync.Mutex
	currentRefresh string
	gen            int
}

func (s *rotatingAuthServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presented := r.Header.Get("X-Refresh-Token")
		if presented == "" {
			// Access token treated as expired -> client must refresh.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		if presented != s.currentRefresh {
			// Stale/already-rotated token. This is the "Failed to refresh token" path.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Rotate: old token is now dead, issue a fresh pair.
		s.gen++
		s.currentRefresh = "refresh-v" + string(rune('0'+s.gen))
		w.Header().Set("X-New-Access-Token", "access-v"+string(rune('0'+s.gen)))
		w.Header().Set("X-New-Refresh-Token", s.currentRefresh)
		w.WriteHeader(http.StatusOK)
	}
}

// TestFetchRefreshRotationDesync pins the fix for the production "Failed to refresh
// token" lockout: the server rotates the refresh token on every refresh, so a lost
// persist of the rotated token would strand the client on a dead token and brick the
// session forever. The client must survive a keyring save failure by keeping the
// rotated refresh token in-process, so subsequent refresh cycles still present the
// live token.
func TestFetchRefreshRotationDesync(t *testing.T) {
	// setup wires the probe around a rotating server. saveRefreshOK toggles
	// whether persisting the rotated refresh token succeeds.
	setup := func(t *testing.T, saveRefreshOK bool) {
		p := newProbe(t)
		p.access, p.refresh = "stale-access", "refresh-v0"
		if !saveRefreshOK {
			p.saveRefreshErr = errors.New("keyring write denied")
		}
		serve(t, (&rotatingAuthServer{currentRefresh: "refresh-v0"}).handler())
	}

	t.Run("refresh save failure does not brick session", func(t *testing.T) {
		setup(t, false) // keyring fails to persist rotated refresh token

		// Every cycle: access expired -> refresh. The server rotates the refresh token
		// each time and the keyring write fails, so without an in-process fallback the
		// client would replay a dead token and 401 forever from cycle 2 on. With the
		// fix, the rotated token survives in memory and every cycle refreshes cleanly.
		for cycle := 1; cycle <= 3; cycle++ {
			if err := Fetch(context.Background(), "GET", "/", nil, nil, nil); err != nil {
				t.Fatalf("cycle %d should survive keyring save failure, got: %v", cycle, err)
			}
		}
	})

	t.Run("control: refresh save success keeps session alive", func(t *testing.T) {
		setup(t, true) // keyring persists rotated token correctly

		// Same scenario, but the rotated refresh token is persisted. Every cycle
		// should refresh cleanly and never brick.
		for cycle := 1; cycle <= 3; cycle++ {
			if err := Fetch(context.Background(), "GET", "/", nil, nil, nil); err != nil {
				t.Fatalf("cycle %d expected success, got: %v", cycle, err)
			}
		}
	})
}

func TestFetchWithRetry(t *testing.T) {
	t.Run("missing base URL", func(t *testing.T) {
		newProbe(t)
		os.Unsetenv("API_URL")

		err := Fetch(context.Background(), "GET", "/test", nil, nil, nil)
		if err == nil || (!strings.Contains(err.Error(), "API_URL") && !strings.Contains(err.Error(), "no server")) {
			t.Fatalf("expected base URL missing error, got %v", err)
		}
	})

	t.Run("payload marshal error", func(t *testing.T) {
		newProbe(t)
		os.Setenv("API_URL", "http://example.com")

		// A function value cannot be marshaled.
		err := Fetch(context.Background(), "POST", "/test", func() {}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to marshal request body") {
			t.Fatalf("expected marshal error, got %v", err)
		}
	})

	t.Run("success case", func(t *testing.T) {
		newProbe(t)
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"hello": "world"})
		})

		var resp map[string]string
		if err := Fetch(context.Background(), "GET", "/", nil, nil, &resp); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp["hello"] != "world" {
			t.Fatalf("unexpected response: %v", resp)
		}
	})

	t.Run("unauthorized then retry", func(t *testing.T) {
		newProbe(t)
		callCount := 0
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "retried"})
		})

		var resp map[string]string
		if err := Fetch(context.Background(), "GET", "/", nil, nil, &resp); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp["status"] != "retried" {
			t.Fatalf("expected retried status, got %v", resp)
		}
	})

	t.Run("unauthorized clears tokens after retry", func(t *testing.T) {
		p := newProbe(t)
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected session expired error, got %v", err)
		}
		if !p.clearedAccess || !p.clearedRefresh {
			t.Fatalf("expected both tokens to be cleared")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		newProbe(t)
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		})

		var resp map[string]string
		err := Fetch(context.Background(), "GET", "/", nil, nil, &resp)
		if err == nil || !strings.Contains(err.Error(), "failed to parse JSON") {
			t.Fatalf("expected JSON parse error, got %v", err)
		}
	})

	t.Run("API error without retry", func(t *testing.T) {
		newProbe(t)
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad stuff", http.StatusBadRequest)
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "API error 400") {
			t.Fatalf("expected API error, got %v", err)
		}
	})

	t.Run("X-Refresh-Token and X-Device-ID sent only on retry", func(t *testing.T) {
		p := newProbe(t)
		callCount := 0
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				if r.Header.Get("X-Refresh-Token") != "" || r.Header.Get("X-Device-ID") != "" {
					t.Errorf("headers should not be set on the first call")
				}
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Header.Get("X-Refresh-Token") != p.refresh || r.Header.Get("X-Device-ID") != p.device {
				t.Errorf("headers not set correctly on retry: got %v / %v",
					r.Header.Get("X-Refresh-Token"), r.Header.Get("X-Device-ID"))
			}
			w.WriteHeader(http.StatusOK)
		})

		if err := Fetch(context.Background(), "GET", "/", nil, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("new access and refresh tokens are saved", func(t *testing.T) {
		p := newProbe(t)
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-New-Access-Token", "newAccess123")
			w.Header().Set("X-New-Refresh-Token", "newRefresh456")
			w.WriteHeader(http.StatusOK)
		})

		if err := Fetch(context.Background(), "GET", "/", nil, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.access != "newAccess123" {
			t.Fatalf("expected access token saved as 'newAccess123', got %q", p.access)
		}
		if p.refresh != "newRefresh456" {
			t.Fatalf("expected refresh token saved as 'newRefresh456', got %q", p.refresh)
		}
	})

	t.Run("no access token surfaces the server's 401", func(t *testing.T) {
		p := newProbe(t)
		// Nothing to identify the session with, so the refresh is never attempted
		// and the 401 stands on its own. See TestFetchKeepsSessionOnUnprovenFailure.
		p.access, p.accessErr = "", errors.New("no token")
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "API error 401") {
			t.Fatalf("expected the API 401 to surface, got %v", err)
		}
	})

	t.Run("concurrent 401s only refresh once", func(t *testing.T) {
		newProbe(t)
		var refreshCount atomic.Int32
		var refreshed atomic.Bool
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Refresh-Token") != "" {
				refreshCount.Add(1)
				refreshed.Store(true)
				time.Sleep(50 * time.Millisecond)
				w.Header().Set("X-New-Access-Token", "freshAccess")
				w.Header().Set("X-New-Refresh-Token", "freshRefresh")
				w.WriteHeader(http.StatusOK)
				return
			}
			// After the refresh, the new access token is accepted.
			if refreshed.Load() && r.Header.Get("Authorization") != "" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		})

		const n = 10
		var wg sync.WaitGroup
		errs := make([]error, n)
		for i := range n {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				errs[idx] = Fetch(context.Background(), "GET", "/", nil, nil, nil)
			}(i)
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Errorf("goroutine %d got unexpected error: %v", i, err)
			}
		}
		if got := refreshCount.Load(); got != 1 {
			t.Fatalf("expected exactly 1 refresh request, got %d", got)
		}
	})
}

// TestFetchKeepsSessionOnUnprovenFailure pins the fix for the production
// "logged out every few minutes" report. A 401 is only proof that the stored
// session is dead when the server was actually given the material to refresh it
// and rejected it. Every other 401 -- one we could not attach credentials to,
// and one the endpoint itself returned after the server had refreshed -- used to
// wipe the keyring and drop the user on the login screen.
func TestFetchKeepsSessionOnUnprovenFailure(t *testing.T) {
	t.Run("unreadable access token is not a dead session", func(t *testing.T) {
		p := newProbe(t)
		p.access, p.accessErr = "", errors.New("keyring read failed")

		// The server never sees a bearer token, so it can only answer 401: it has
		// no idea which session is asking, let alone that it has expired.
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected the server's 401 to surface as-is, got %v", err)
		}
		if p.cleared() {
			t.Fatal("a keyring that could not be read must not cost the user the session")
		}
	})

	t.Run("unreadable device id is not a dead session", func(t *testing.T) {
		p := newProbe(t)
		p.device, p.deviceErr = "", errors.New("keyring read failed")

		// Without X-Device-ID the server cannot hash the refresh token, so the
		// retry is doomed before it is sent.
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected the server's 401 to surface as-is, got %v", err)
		}
		if p.cleared() {
			t.Fatal("a missing device id must not cost the user the session")
		}
	})

	t.Run("endpoint's own 401 after a successful refresh is not a dead session", func(t *testing.T) {
		p := newProbe(t)

		// The server refreshes -- it hands back a new pair -- and the handler
		// behind it answers 401 for its own reasons.
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Refresh-Token") != "" {
				w.Header().Set("X-New-Access-Token", "freshAccess")
				w.Header().Set("X-New-Refresh-Token", "freshRefresh")
			}
			w.WriteHeader(http.StatusUnauthorized)
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected the endpoint's 401 to surface as-is, got %v", err)
		}
		if p.cleared() {
			t.Fatal("an endpoint's 401 must not cost the user the session")
		}
		if p.access != "freshAccess" {
			t.Fatalf("the refreshed access token should still be kept, got %q", p.access)
		}
	})

	t.Run("a refresh that never reached the server is not a dead session", func(t *testing.T) {
		p := newProbe(t)

		// 401 first, then drop the connection on the refresh: a network blip in
		// the one window where the client is most exposed.
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Refresh-Token") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			conn, _, hijackErr := w.(http.Hijacker).Hijack()
			if hijackErr != nil {
				t.Errorf("hijack: %v", hijackErr)
				return
			}
			_ = conn.Close()
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected the transport error to surface, got %v", err)
		}
		if p.cleared() {
			t.Fatal("a network failure must not cost the user the session")
		}

		// Requests that queued behind this refresh are told the same thing, so a
		// blip never reads as a logout anywhere.
		refreshMu.Lock()
		latched := refreshErr
		refreshMu.Unlock()
		if latched == nil || strings.Contains(latched.Error(), "session expired") {
			t.Fatalf("waiters should inherit the transport error, got %v", latched)
		}
	})

	t.Run("control: a rejected refresh token ends the session", func(t *testing.T) {
		p := newProbe(t)

		// The server is given everything it needs and still says no: the refresh
		// token is gone, and there is nothing left to log in with.
		serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected session expired, got %v", err)
		}
		if !p.cleared() {
			t.Fatal("a rejected refresh token should clear the stored session")
		}
	})
}
