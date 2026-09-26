package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_CancelsTheRequestContext(t *testing.T) {
	var deadline time.Time
	h := Timeout(time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, _ = r.Context().Deadline()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if left := time.Until(deadline); left <= 0 || left > time.Minute {
		t.Fatalf("deadline in %s, want within a minute", left)
	}
}
