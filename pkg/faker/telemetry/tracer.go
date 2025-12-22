package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer abstracts OpenTelemetry operations for faker
type Tracer interface {
	// StartSpan starts a new span with the given name and attributes
	StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span)

	// RecordError records an error on the span
	RecordError(span trace.Span, err error)

	// RecordEvent records an event on the span
	RecordEvent(span trace.Span, name string, attrs ...attribute.KeyValue)

	// SetStatus sets the status of the span
	SetStatus(span trace.Span, code codes.Code, description string)

	// Shutdown gracefully shuts down the tracer and flushes remaining spans
	Shutdown(ctx context.Context) error
}

// tracer implements Tracer interface
type tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	debug    bool
}

// NewTracer creates a new OpenTelemetry tracer for faker
func NewTracer(endpoint string, debug bool) (Tracer, error) {
	if endpoint == "" {
		// Return a no-op tracer if no endpoint configured
		return &noopTracer{}, nil
	}

	ctx := context.Background()

	// Check if endpoint uses HTTPS before stripping protocol
	useHTTPS := strings.HasPrefix(endpoint, "https://")
	// Parse endpoint to remove protocol - OTLP HTTP exporter expects host:port format
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER OTEL] Initializing with endpoint: %s (https=%v)\n", endpoint, useHTTPS)
	}

	// Create OTLP HTTP trace exporter
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
	}
	// Only use insecure (HTTP) if endpoint is not HTTPS
	if !useHTTPS {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource for faker process without schema to avoid version conflicts
	res := resource.NewSchemaless(
		semconv.ServiceName("station"),
		semconv.ServiceVersion("faker"),
	)

	// Use SimpleSpanProcessor for immediate export (better for short-lived MCP tool calls)
	// BatchSpanProcessor can delay export up to 5 seconds, causing spans to be lost
	spanProcessor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Create tracer provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(spanProcessor),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER OTEL] Tracer initialized successfully\n")
	}

	return &tracer{
		provider: tracerProvider,
		tracer:   otel.Tracer("station.faker"),
		debug:    debug,
	}, nil
}

// StartSpan starts a new span with the given name and attributes
func (t *tracer) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordError records an error on the span
func (t *tracer) RecordError(span trace.Span, err error) {
	span.RecordError(err)
}

// RecordEvent records an event on the span
func (t *tracer) RecordEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetStatus sets the status of the span
func (t *tracer) SetStatus(span trace.Span, code codes.Code, description string) {
	span.SetStatus(code, description)
}

// Shutdown gracefully shuts down the tracer and flushes remaining spans
func (t *tracer) Shutdown(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}

	// Create timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return t.provider.Shutdown(shutdownCtx)
}

// noopTracer is a no-op implementation when telemetry is disabled
type noopTracer struct{}

func (n *noopTracer) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}

func (n *noopTracer) RecordError(span trace.Span, err error) {}

func (n *noopTracer) RecordEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {}

func (n *noopTracer) SetStatus(span trace.Span, code codes.Code, description string) {}

func (n *noopTracer) Shutdown(ctx context.Context) error {
	return nil
}
