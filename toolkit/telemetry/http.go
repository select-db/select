package telemetry

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware wraps a handler with OTLP tracing and the standard HTTP server
// metrics (duration, request and response size, active requests).
//
// Spans are named after the ServeMux pattern, not the URL: "GET /workspaces/{id}"
// is one series, while the raw path is one series per workspace and would bury
// the metric backend under UUIDs.
//
// A no-op when telemetry is off, so the handler graph is identical in dev.
func HTTPMiddleware(serviceName string, next http.Handler) http.Handler {
	if !Enabled() {
		return next
	}
	return otelhttp.NewHandler(next, serviceName,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if r.Pattern != "" {
				return r.Pattern
			}
			// No pattern means no route matched; naming it after the path would
			// mint a series per 404 URL.
			return r.Method + " unmatched"
		}),
		// Health and version are polled by the deploy health-check and every
		// client on an interval. They are uptime signals, not work.
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/health" && r.URL.Path != "/version"
		}),
	)
}

// SpanAttributes attaches attributes to the span in ctx. Attributes go on spans,
// never on metric labels: a workspace or user id is exactly the dimension you
// want when reading one trace and exactly the one that must not multiply series.
func SpanAttributes(r *http.Request, attrs ...attribute.KeyValue) {
	trace.SpanFromContext(r.Context()).SetAttributes(attrs...)
}

// TraceID returns the current trace id, or "" when there is no recording span.
// Use it as the request id so a log line and its trace are the same lookup.
func TraceID(r *http.Request) string {
	sc := trace.SpanContextFromContext(r.Context())
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
