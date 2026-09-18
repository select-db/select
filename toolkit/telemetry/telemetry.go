// Package telemetry wires OpenTelemetry traces and metrics for the backend
// server and the desktop app. Both export OTLP over HTTP to a collector on
// localhost, which owns batching, retry and the trip to the backend store.
//
// It is off unless OTEL_EXPORTER_OTLP_ENDPOINT is set, so an unconfigured
// process (dev, or a box provisioned before the collector) pays nothing and
// cannot fail to boot because telemetry is unreachable.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// Config names the emitting process. Everything else (endpoint, headers,
// sampling) comes from the standard OTEL_* environment variables, so an
// operator retunes a box without a rebuild.
type Config struct {
	// ServiceName is the service.name resource attribute, e.g. "select-backend".
	ServiceName string
	// ServiceVersion is the build version; leave empty to read APP_VERSION.
	ServiceVersion string
	// Environment is dev, staging or production; leave empty to read APP_ENV.
	Environment string
	// Region groups a deployment, e.g. "us-east". Empty outside the backend.
	Region string
}

// Enabled reports whether an OTLP endpoint is configured. Callers use it to
// skip instrumentation setup that would otherwise cost them nothing but noise.
func Enabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
}

// ShutdownFunc flushes pending spans and metrics. Always non-nil, so callers
// can defer it without a nil check even when telemetry is off.
type ShutdownFunc func(context.Context) error

// Start installs the global tracer and meter providers and begins collecting Go
// runtime metrics. The returned shutdown flushes what is buffered; call it after
// the HTTP server has stopped so in-flight spans are not dropped.
//
// A missing endpoint is not an error: it returns a no-op shutdown and leaves the
// global providers alone, so every otel.Tracer call in the tree stays valid.
func Start(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	if !Enabled() {
		return func(context.Context) error { return nil }, nil
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}

	traceExp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		// ParentBased+AlwaysSample: the collector owns sampling policy, so it can
		// be retuned without shipping a new binary to every node and desktop.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	metricExp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("telemetry: metric exporter: %w", err),
			tracerProvider.Shutdown(ctx),
		)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(exportInterval()),
		)),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	// W3C trace context so a trace survives nginx, the workspace router and the
	// desktop app's own requests.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if err := runtime.Start(runtime.WithMeterProvider(meterProvider)); err != nil {
		return nil, fmt.Errorf("telemetry: runtime metrics: %w", err)
	}

	return func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}, nil
}

// Tracer returns a tracer under the calling component's name. Safe before
// Start: the global provider is a no-op until then.
func Tracer(name string) trace.Tracer { return otel.Tracer(name) }

// Meter returns a meter under the calling component's name.
func Meter(name string) metric.Meter { return otel.Meter(name) }

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	version := cfg.ServiceVersion
	if version == "" {
		version = os.Getenv("APP_VERSION")
	}
	env := cfg.Environment
	if env == "" {
		env = os.Getenv("APP_ENV")
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(version),
		semconv.DeploymentEnvironmentNameKey.String(env),
	}
	if cfg.Region != "" {
		attrs = append(attrs, semconv.CloudRegion(cfg.Region))
	}

	// WithHost/WithProcess fill host.name and process.*; the collector's
	// resourcedetection processor adds the rest and wins on conflict.
	return resource.New(ctx,
		resource.WithHost(),
		resource.WithProcessRuntimeDescription(),
		resource.WithAttributes(attrs...),
	)
}

// exportInterval reads OTEL_METRIC_EXPORT_INTERVAL (milliseconds), defaulting to
// 30s. Shorter than the 60s SDK default so a launch graph moves while you watch.
func exportInterval() time.Duration {
	if v := os.Getenv("OTEL_METRIC_EXPORT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v + "ms"); err == nil && d > 0 {
			return d
		}
	}
	return 30 * time.Second
}
