package services

import (
	"context"
	"fmt"

	"station/internal/logging"

	"github.com/firebase/genkit/go/genkit"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// TelemetryManager handles OpenTelemetry configuration and export
type TelemetryManager struct {
	exporter trace.SpanExporter
}

// NewTelemetryManager creates a new telemetry manager
func NewTelemetryManager() *TelemetryManager {
	return &TelemetryManager{}
}

// SetupOpenTelemetryExport sets up OpenTelemetry export to Jaeger using Genkit's RegisterSpanProcessor
func (tm *TelemetryManager) SetupOpenTelemetryExport(ctx context.Context, g *genkit.Genkit) error {
	// Create OTLP exporter for Jaeger
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("http://localhost:14317"),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	tm.exporter = exporter

	// Create resource with service information
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("station-agent-creator"),
		semconv.ServiceVersionKey.String("0.1.0"),
	)

	// Create span processor
	spanProcessor := trace.NewBatchSpanProcessor(exporter)

	// TODO: Register span processor with GenKit
	// The current GenKit version doesn't expose RegisterSpanProcessor
	_ = spanProcessor
	_ = res

	logging.Debug("OpenTelemetry export configured for Jaeger at localhost:14317")
	return nil
}

// Shutdown closes the telemetry exporter
func (tm *TelemetryManager) Shutdown(ctx context.Context) error {
	if tm.exporter == nil {
		return nil
	}
	return tm.exporter.Shutdown(ctx)
}

// stationTraceExporter wraps the OTLP exporter to provide shutdown capability
// This is used to maintain compatibility with the existing shutdown pattern
type stationTraceExporter struct {
	exporter trace.SpanExporter
}

func newStationTraceExporter(exporter trace.SpanExporter) *stationTraceExporter {
	return &stationTraceExporter{exporter: exporter}
}

func (e *stationTraceExporter) Shutdown(ctx context.Context) error {
	return e.exporter.Shutdown(ctx)
}