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

func TestFetchWithRetry(t *testing.T) {
	// Backup original funcs
	origLoadAccess := loadAccessTokenFunc
	origLoadRefresh := loadRefreshTokenFunc
	origLoadDevice := loadDeviceIDFunc
	origSaveAccess := saveAccessTokenFunc
	origSaveRefresh := saveRefreshTokenFunc
	origClearAccess := clearAccessTokenFunc
	origClearRefresh := clearRefreshTokenFunc
	defer func() {
		loadAccessTokenFunc = origLoadAccess
		loadRefreshTokenFunc = origLoadRefresh
		loadDeviceIDFunc = origLoadDevice
		saveAccessTokenFunc = origSaveAccess
		saveRefreshTokenFunc = origSaveRefresh
		clearAccessTokenFunc = origClearAccess
		clearRefreshTokenFunc = origClearRefresh
	}()

	resetTestState := func() {
		loadAccessTokenFunc = func() (string, error) { return "accessX", nil }
		loadRefreshTokenFunc = func() (string, error) { return "refreshX", nil }
		loadDeviceIDFunc = func() (string, error) { return "deviceX", nil }
		saveAccessTokenFunc = func(string) error { return nil }
		saveRefreshTokenFunc = func(string) error { return nil }
		clearAccessTokenFunc = func() error { return nil }
		clearRefreshTokenFunc = func() error { return nil }
		refreshedAt = time.Time{}
		refreshFailed = false
	}
	resetTestState()

	t.Run("missing base URL", func(t *testing.T) {
		os.Unsetenv("API_URL")
		err := Fetch(context.Background(), "GET", "/test", nil, nil, nil)
		if err == nil || (!strings.Contains(err.Error(), "API_URL") && !strings.Contains(err.Error(), "no server")) {
			t.Fatalf("expected base URL missing error, got %v", err)
		}
	})

	t.Run("payload marshal error", func(t *testing.T) {
		os.Setenv("API_URL", "http://example.com")
		// Payload that cannot be marshaled (function type)
		err := Fetch(context.Background(), "POST", "/test", func() {}, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "failed to marshal request body") {
			t.Fatalf("expected marshal error, got %v", err)
		}
	})

	t.Run("success case", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]string{"hello": "world"})
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		var resp map[string]string
		err := Fetch(context.Background(), "GET", "/", nil, nil, &resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp["hello"] != "world" {
			t.Fatalf("unexpected response: %v", resp)
		}
	})

	t.Run("unauthorized then retry", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "retried"})
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		var resp map[string]string
		err := Fetch(context.Background(), "GET", "/", nil, nil, &resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp["status"] != "retried" {
			t.Fatalf("expected retried status, got %v", resp)
		}
	})

	t.Run("unauthorized clears tokens after retry", func(t *testing.T) {
		resetTestState()
		clearedAccess := false
		clearedRefresh := false
		clearAccessTokenFunc = func() error { clearedAccess = true; return nil }
		clearRefreshTokenFunc = func() error { clearedRefresh = true; return nil }

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected session expired error, got %v", err)
		}
		if !clearedAccess || !clearedRefresh {
			t.Fatalf("expected tokens to be cleared")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		var resp map[string]string
		err := Fetch(context.Background(), "GET", "/", nil, nil, &resp)
		if err == nil || !strings.Contains(err.Error(), "failed to parse JSON") {
			t.Fatalf("expected JSON parse error, got %v", err)
		}
	})

	t.Run("API error without retry", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad stuff", http.StatusBadRequest)
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "API error 400") {
			t.Fatalf("expected API error, got %v", err)
		}
	})

	t.Run("X-Refresh-Token and X-Device-ID loaded only on retry", func(t *testing.T) {
		resetTestState()
		loadedRefresh, _ := loadRefreshTokenFunc()
		loadedDevice, _ := loadDeviceIDFunc()
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				// On first call, headers should NOT be set
				if r.Header.Get("X-Refresh-Token") != "" || r.Header.Get("X-Device-ID") != "" {
					t.Fatalf("headers should not be set on first call")
				}
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// On retry, headers should be set
			if r.Header.Get("X-Refresh-Token") != loadedRefresh || r.Header.Get("X-Device-ID") != loadedDevice {
				t.Fatalf("headers not set correctly on retry: got %v / %v", r.Header.Get("X-Refresh-Token"), r.Header.Get("X-Device-ID"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("new access and refresh tokens are saved", func(t *testing.T) {
		resetTestState()
		var savedAccess, savedRefresh string
		saveAccessTokenFunc = func(token string) error {
			savedAccess = token
			return nil
		}
		saveRefreshTokenFunc = func(token string) error {
			savedRefresh = token
			return nil
		}

		// HTTP server that returns new tokens
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-New-Access-Token", "newAccess123")
			w.Header().Set("X-New-Refresh-Token", "newRefresh456")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Assert tokens were saved
		if savedAccess != "newAccess123" {
			t.Fatalf("expected access token saved as 'newAccess123', got '%s'", savedAccess)
		}
		if savedRefresh != "newRefresh456" {
			t.Fatalf("expected refresh token saved as 'newRefresh456', got '%s'", savedRefresh)
		}
	})

	t.Run("no access token still retries and expires", func(t *testing.T) {
		resetTestState()
		loadAccessTokenFunc = func() (string, error) { return "", errors.New("no token") }
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

		err := Fetch(context.Background(), "GET", "/", nil, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "session expired") {
			t.Fatalf("expected session expired error, got %v", err)
		}
	})

	t.Run("concurrent 401s only refresh once", func(t *testing.T) {
		resetTestState()

		var refreshCount atomic.Int32
		var refreshed atomic.Bool
		loadAccessTokenFunc = func() (string, error) { return "accessX", nil }
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Refresh-Token") != "" {
				refreshCount.Add(1)
				refreshed.Store(true)
				time.Sleep(50 * time.Millisecond)
				w.Header().Set("X-New-Access-Token", "freshAccess")
				w.Header().Set("X-New-Refresh-Token", "freshRefresh")
				w.WriteHeader(http.StatusOK)
				return
			}
			// After refresh, accept requests with the new access token
			if refreshed.Load() && r.Header.Get("Authorization") != "" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		os.Setenv("API_URL", srv.URL)

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
