package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The disabled path is the one every unconfigured box takes, so it has to be
// provably inert: no error, a usable shutdown, and the caller's own handler back.
func TestDisabledWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	if Enabled() {
		t.Fatal("Enabled() is true with no endpoint set")
	}

	shutdown, err := Start(context.Background(), Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if shutdown == nil {
		t.Fatal("Start returned a nil shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestHTTPMiddlewareDisabledReturnsSameHandler(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	var called bool
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	HTTPMiddleware("test", next).ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/health", nil),
	)
	if !called {
		t.Fatal("wrapped handler was not called")
	}
}

func TestObserveSQLPoolDisabledIsNoOp(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	// nil pool: the callback must never be registered, let alone run.
	if err := ObserveSQLPool("app", nil); err != nil {
		t.Fatalf("ObserveSQLPool: %v", err)
	}
}

func TestExportIntervalDefaultsAndParses(t *testing.T) {
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "")
	if got := exportInterval(); got.Seconds() != 30 {
		t.Fatalf("default interval = %v, want 30s", got)
	}

	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "5000")
	if got := exportInterval(); got.Seconds() != 5 {
		t.Fatalf("interval = %v, want 5s", got)
	}

	// Garbage falls back rather than exporting on a zero interval.
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "soon")
	if got := exportInterval(); got.Seconds() != 30 {
		t.Fatalf("invalid interval = %v, want the 30s default", got)
	}
}
