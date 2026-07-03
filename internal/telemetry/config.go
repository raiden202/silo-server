// Package telemetry provides the OpenTelemetry SDK bootstrap for Silo: shared
// resource, tracer provider, logger provider, and W3C propagation. It
// deliberately does NOT build or register a MeterProvider — metrics stay on the
// existing Prometheus rail. Leaving the global MeterProvider as the built-in
// no-op is what prevents the trace instrumentation libraries from double-emitting
// metrics. See docs/superpowers/plans/2026-07-02-opentelemetry-observability.md.
package telemetry

import (
	"math"
	"os"
	"strconv"
	"strings"
)

// Protocol identifies the OTLP exporter wire protocol.
type Protocol string

const (
	// ProtocolGRPC is the OTLP/gRPC protocol (default).
	ProtocolGRPC Protocol = "grpc"
	// ProtocolHTTP is the OTLP/HTTP+protobuf protocol.
	ProtocolHTTP Protocol = "http/protobuf"
)

// defaultServiceName is used when OTEL_SERVICE_NAME is unset.
const defaultServiceName = "silo-server"

// defaultSamplerRatio is the parent-based trace-id ratio applied when
// OTEL_TRACES_SAMPLER_ARG is unset or unparseable.
const defaultSamplerRatio = 1.0

// Config is the fully-defaulted telemetry configuration parsed from the
// environment. It is cheap to construct and safe to build even when telemetry
// is disabled.
type Config struct {
	// Enabled gates the entire feature. True when SILO_OTEL_ENABLED is truthy
	// OR OTEL_EXPORTER_OTLP_ENDPOINT is set.
	Enabled bool

	// Endpoint is the OTLP collector endpoint (OTEL_EXPORTER_OTLP_ENDPOINT). It
	// is used ONLY to decide Enabled; the exporters themselves read the endpoint
	// (and all other OTEL_EXPORTER_OTLP_* knobs) directly from the environment,
	// which remains the single source of truth for exporter wiring.
	Endpoint string
	// Protocol selects the OTLP wire protocol.
	Protocol Protocol

	// ServiceName populates the service.name resource attribute.
	ServiceName string
	// ServiceVersion populates the service.version resource attribute.
	ServiceVersion string
	// NodeID populates the node.name resource attribute.
	NodeID string

	// SamplerRatio is the parent-based trace-id-ratio sampling probability.
	SamplerRatio float64
}

// LoadConfig parses the telemetry configuration from the environment. nodeID is
// the resolved node identity used for the node.name resource attribute.
func LoadConfig(nodeID string) Config {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	enabled := truthy(os.Getenv("SILO_OTEL_ENABLED")) || endpoint != ""

	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	protocol := ProtocolGRPC
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))) {
	case "http/protobuf", "http":
		protocol = ProtocolHTTP
	case "grpc", "":
		protocol = ProtocolGRPC
	}

	ratio := defaultSamplerRatio
	if raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")); raw != "" {
		// Accept only finite, non-negative values; clamp above 1 to 1.0 so a
		// typo'd or +Inf arg means "sample everything" rather than silently
		// falling through. NaN and -Inf fail the v >= 0 / IsInf checks.
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 && !math.IsInf(v, 1) {
			ratio = math.Min(v, 1.0)
		}
	}

	return Config{
		Enabled:        enabled,
		Endpoint:       endpoint,
		Protocol:       protocol,
		ServiceName:    serviceName,
		ServiceVersion: strings.TrimSpace(os.Getenv("OTEL_SERVICE_VERSION")),
		NodeID:         nodeID,
		SamplerRatio:   ratio,
	}
}

// truthy reports whether an env value should be treated as a boolean true.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
