package telemetry

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/firebase/genkit/go/genkit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// OTelConfig configures OpenTelemetry integration with Genkit
type OTelConfig struct {
	// Export telemetry even in dev environment
	ForceExport bool
	
	// OTLP endpoint (e.g., http://jaeger:4318/v1/traces)
	OTLPEndpoint string
	
	// Service name for traces
	ServiceName string
	
	// Service version
	ServiceVersion string
}

// SetupOpenTelemetryWithGenkit configures OpenTelemetry to work with Genkit
func SetupOpenTelemetryWithGenkit(ctx context.Context, g *genkit.Genkit, cfg OTelConfig) error {
	// Determine if we should export telemetry
	shouldExport := cfg.ForceExport || os.Getenv("GENKIT_ENV") != "dev"
	
	if !shouldExport {
		log.Printf("📊 Telemetry export disabled (GENKIT_ENV=dev, set ForceExport=true to override)")
		return nil
	}

	// Set up OTLP endpoint
	endpoint := cfg.OTLPEndpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = "http://localhost:4318"
	}

	// Create OTLP HTTP trace exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Create resource with service information
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = os.Getenv("OTEL_SERVICE_NAME")
	}
	if serviceName == "" {
		serviceName = "station"
	}

	serviceVersion := cfg.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = os.Getenv("OTEL_SERVICE_VERSION")
	}
	if serviceVersion == "" {
		serviceVersion = "dev"
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create batch span processor with the exporter
	spanProcessor := trace.NewBatchSpanProcessor(
		exporter,
		trace.WithBatchTimeout(5*time.Second),
		trace.WithMaxExportBatchSize(100),
	)

	// Register the span processor with Genkit
	genkit.RegisterSpanProcessor(g, spanProcessor)

	// Create trace provider with the processor and resource
	tracerProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanProcessor),
		trace.WithResource(res),
	)

	// Set the global tracer provider
	otel.SetTracerProvider(tracerProvider)

	log.Printf("📊 OpenTelemetry configured with Genkit - exporting to %s", endpoint)
	return nil
}

// ShutdownOpenTelemetry gracefully shuts down OpenTelemetry
func ShutdownOpenTelemetry(ctx context.Context) error {
	// Get the current tracer provider
	if tp, ok := otel.GetTracerProvider().(*trace.TracerProvider); ok {
		return tp.Shutdown(ctx)
	}
	return nil
}