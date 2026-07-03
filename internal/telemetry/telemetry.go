package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlplog "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	otlploghttp "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	logapi "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Providers holds the constructed OpenTelemetry providers. When telemetry is
// disabled, LoggerProvider is a no-op so callers can build an slog handler
// unconditionally without branching.
type Providers struct {
	// LoggerProvider is the OTel logs provider. Never nil (no-op when disabled).
	LoggerProvider logapi.LoggerProvider
}

// noopShutdown is an inert shutdown used when telemetry is disabled.
func noopShutdown(context.Context) error { return nil }

// Setup initializes the OpenTelemetry SDK from cfg. When cfg.Enabled is false it
// installs nothing (leaving all otel globals as their built-in no-ops) and
// returns a Providers holding no-op providers plus a no-op shutdown.
//
// When enabled it builds one shared resource, a composite W3C propagator, a
// TracerProvider, and a LoggerProvider, registering each as the corresponding
// otel global. It deliberately does NOT build or register a MeterProvider:
// metrics stay on Prometheus, and the built-in no-op global MeterProvider keeps
// the trace instrumentation libraries from double-emitting metrics.
//
// The returned shutdown flushes and closes all providers; it is idempotent and
// joins any errors from the underlying shutdown funcs.
func Setup(ctx context.Context, cfg Config) (*Providers, func(context.Context) error, error) {
	if !cfg.Enabled {
		return &Providers{LoggerProvider: lognoop.NewLoggerProvider()}, noopShutdown, nil
	}

	// resource.New can return a usable, fully-merged resource together with a
	// non-fatal error (e.g. ErrSchemaURLConflict when a detector's schema differs
	// from ours). Per OTel guidance, use the returned resource and treat the error
	// as a warning; only abort if no resource came back. This preserves the
	// "telemetry is best-effort, must never brick the server" contract.
	res, err := buildResource(ctx, cfg)
	if err != nil {
		if res == nil {
			return nil, noopShutdown, err
		}
		slog.WarnContext(ctx, "telemetry: resource built with warnings", "component", "telemetry", "error", err)
	}

	var shutdownFuncs []func(context.Context) error

	// TracerProvider. Build the exporter/provider before mutating any process
	// globals so a failure leaves nothing installed.
	traceExp, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return nil, noopShutdown, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerRatio))),
	)
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)

	// LoggerProvider.
	logExp, err := newLogExporter(ctx, cfg)
	if err != nil {
		// Best-effort cleanup of what we already built before failing. Nothing
		// is installed as a global yet, so this only drains the trace batcher.
		_ = tp.Shutdown(ctx)
		return nil, noopShutdown, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	)
	shutdownFuncs = append(shutdownFuncs, lp.Shutdown)

	// Both exporters constructed successfully: now install the globals. Setting
	// them last preserves the "install nothing on failure" invariant.
	otel.SetTracerProvider(tp)
	global.SetLoggerProvider(lp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := newIdempotentShutdown(shutdownFuncs)

	return &Providers{LoggerProvider: lp}, shutdown, nil
}

// buildResource constructs the single shared resource describing this process.
func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{semconv.ServiceName(cfg.ServiceName)}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.ServiceVersion))
	}
	if cfg.NodeID != "" {
		attrs = append(attrs, attribute.String("node.name", cfg.NodeID))
	}
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithAttributes(attrs...),
	)
}

// newTraceExporter builds the OTLP trace exporter for the configured protocol.
func newTraceExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	if cfg.TracesProtocol == ProtocolHTTP {
		return otlptracehttp.New(ctx)
	}
	return otlptracegrpc.New(ctx)
}

// newLogExporter builds the OTLP log exporter for the configured protocol.
func newLogExporter(ctx context.Context, cfg Config) (sdklog.Exporter, error) {
	if cfg.LogsProtocol == ProtocolHTTP {
		return otlploghttp.New(ctx)
	}
	return otlplog.New(ctx)
}

// newIdempotentShutdown returns a shutdown func that runs the accumulated
// shutdown funcs exactly once, joining their errors.
func newIdempotentShutdown(funcs []func(context.Context) error) func(context.Context) error {
	var once sync.Once
	var err error
	return func(ctx context.Context) error {
		once.Do(func() {
			var errs []error
			for _, fn := range funcs {
				if fn == nil {
					continue
				}
				if e := fn(ctx); e != nil {
					errs = append(errs, e)
				}
			}
			err = errors.Join(errs...)
		})
		return err
	}
}
