package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func keyHeader(r *http.Request) (string, int) { return r.Header.Get("K"), 1 }

func TestInFlight_WaitsForASlot(t *testing.T) {
	release := make(chan struct{})
	var running atomic.Int32
	h := InFlight(keyHeader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if running.Add(1) > 1 {
			t.Error("two requests ran at once, limit is 1")
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
}

func TestInFlight_DeadlineWhileWaiting(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	h := InFlight(keyHeader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))

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
	h := InFlight(keyHeader)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestInFlight_LimitComesFromTheRequest(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	h := InFlight(func(*http.Request) (string, int) { return "ws", 2 })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	for range 2 {
		go h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if w.Code != http.StatusRequestTimeout {
		t.Fatalf("status %d: a third request ran with a limit of 2", w.Code)
	}
}
