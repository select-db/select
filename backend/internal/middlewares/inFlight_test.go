package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func keyHeader(r *http.Request) string { return r.Header.Get("K") }

func TestInFlight_WaitsForASlot(t *testing.T) {
	release := make(chan struct{})
	var running, peak atomic.Int32
	h := InFlight(1, keyHeader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := running.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		<-release
		running.Add(-1)
	}))

	done := make(chan int, 2)
	for range 2 {
		go func() {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("K", "ws-1")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			done <- w.Code
		}()
	}
	time.Sleep(50 * time.Millisecond)
	release <- struct{}{}
	release <- struct{}{}
	for range 2 {
		if code := <-done; code != http.StatusOK {
			t.Fatalf("status %d, want the waiting request to run, not fail", code)
		}
	}
	if peak.Load() != 1 {
		t.Fatalf("%d requests ran at once, limit is 1", peak.Load())
	}
}

func TestInFlight_DeadlineWhileWaiting(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	h := InFlight(1, keyHeader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))

	go h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if w.Code != http.StatusRequestTimeout {
		t.Fatalf("status %d, want 408", w.Code)
	}
}

func TestInFlight_KeysDoNotShareSlots(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	h := InFlight(1, keyHeader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("K") == "busy" {
			<-release
		}
	}))
	busy := httptest.NewRequest("GET", "/", nil)
	busy.Header.Set("K", "busy")
	go h.ServeHTTP(httptest.NewRecorder(), busy)
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	other := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	other.Header.Set("K", "other")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, other)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: one key's load blocked another", w.Code)
	}
}
