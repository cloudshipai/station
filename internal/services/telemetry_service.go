package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName    = "station"
	serviceVersion = "0.2.7"
)

// TelemetryService manages OpenTelemetry initialization and instrumentation
type TelemetryService struct {
	tracer       trace.Tracer
	meter        metric.Meter
	shutdownFunc func(context.Context) error
	config       *TelemetryConfig

	// Business metrics
	agentExecutionCounter  metric.Int64Counter
	agentExecutionDuration metric.Float64Histogram
	tokenUsageCounter      metric.Int64Counter
	toolCallCounter        metric.Int64Counter
	errorCounter           metric.Int64Counter
}

// TelemetryConfig holds configuration for telemetry services
type TelemetryConfig struct {
	Enabled      bool
	OTLPEndpoint string
	ServiceName  string
	Environment  string
}

// NewTelemetryService creates a new telemetry service with configuration
func NewTelemetryService(config *TelemetryConfig) *TelemetryService {
	// Set defaults if nil config provided
	if config == nil {
		config = &TelemetryConfig{
			Enabled:     true,
			ServiceName: serviceName,
			Environment: "development",
		}
	}

	return &TelemetryService{
		config: config,
	}
}

// Initialize sets up OpenTelemetry with appropriate exporters based on configuration
func (ts *TelemetryService) Initialize(ctx context.Context) error {
	// Skip initialization if telemetry is disabled
	if !ts.config.Enabled {
		return nil
	}

	// Create resource with service information
	serviceName := ts.config.ServiceName
	if serviceName == "" {
		serviceName = "station"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
			semconv.DeploymentEnvironmentKey.String(ts.config.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTEL resource: %w", err)
	}

	// Debug logging silenced - use STN_DEBUG=true for telemetry debug info

	// Initialize trace provider with appropriate exporter
	traceProvider, err := ts.initTraceProvider(ctx, res)
	if err != nil {
		return fmt.Errorf("failed to initialize trace provider: %w", err)
	}

	// Set global providers - CRITICAL for spans to be exported
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Create tracer and meter
	ts.tracer = otel.Tracer(serviceName)
	ts.meter = otel.Meter(serviceName)

	// Initialize business metrics
	if err := ts.initMetrics(); err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	return nil
}

// initTraceProvider creates a trace provider with the appropriate exporter
func (ts *TelemetryService) initTraceProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	// Use OTLP endpoint from configuration with fallback to environment variables
	otlpEndpoint := ts.config.OTLPEndpoint
	if otlpEndpoint == "" {
		otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if otlpEndpoint == "" {
			otlpEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
		}
	}

	if otlpEndpoint != "" {
		// Use OTLP exporter (production)
		if os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL") == "grpc" {
			exporter, err = otlptracegrpc.New(ctx)
		} else {
			// Parse the endpoint URL to extract just the host:port
			endpoint := otlpEndpoint
			if strings.HasPrefix(endpoint, "http://") {
				endpoint = strings.TrimPrefix(endpoint, "http://")
			} else if strings.HasPrefix(endpoint, "https://") {
				endpoint = strings.TrimPrefix(endpoint, "https://")
			}

			exporter, err = otlptracehttp.New(ctx,
				otlptracehttp.WithEndpoint(endpoint),
				otlptracehttp.WithInsecure(), // Use HTTP instead of HTTPS for localhost
			)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	} else {
		// Development mode - use no-op exporter to avoid log spam
		exporter = &noOpExporter{}
	}

	// Create trace provider with resource and exporter - optimized for immediate export
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(time.Second*1), // Reduced timeout for faster export
			sdktrace.WithMaxExportBatchSize(1),       // Export immediately
			sdktrace.WithExportTimeout(time.Second*10),
		),
		sdktrace.WithSampler(ts.getSampler()),
	)

	// Store shutdown function
	ts.shutdownFunc = tp.Shutdown

	return tp, nil
}

// initMetrics initializes business-specific metrics
func (ts *TelemetryService) initMetrics() error {
	var err error

	// Agent execution metrics
	ts.agentExecutionCounter, err = ts.meter.Int64Counter(
		"station_agent_executions_total",
		metric.WithDescription("Total number of agent executions"),
	)
	if err != nil {
		return fmt.Errorf("failed to create agent execution counter: %w", err)
	}

	ts.agentExecutionDuration, err = ts.meter.Float64Histogram(
		"station_agent_execution_duration_seconds",
		metric.WithDescription("Duration of agent executions in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create agent execution duration histogram: %w", err)
	}

	// Token usage metrics
	ts.tokenUsageCounter, err = ts.meter.Int64Counter(
		"station_token_usage_total",
		metric.WithDescription("Total number of tokens used"),
	)
	if err != nil {
		return fmt.Errorf("failed to create token usage counter: %w", err)
	}

	// Tool call metrics
	ts.toolCallCounter, err = ts.meter.Int64Counter(
		"station_tool_calls_total",
		metric.WithDescription("Total number of tool calls made"),
	)
	if err != nil {
		return fmt.Errorf("failed to create tool call counter: %w", err)
	}

	// Error metrics
	ts.errorCounter, err = ts.meter.Int64Counter(
		"station_errors_total",
		metric.WithDescription("Total number of errors encountered"),
	)
	if err != nil {
		return fmt.Errorf("failed to create error counter: %w", err)
	}

	return nil
}

// getSampler returns the appropriate sampling strategy
func (ts *TelemetryService) getSampler() sdktrace.Sampler {
	// In production, we might want to sample less aggressively
	env := getEnvironment()
	switch env {
	case "production":
		return sdktrace.TraceIDRatioBased(0.1) // Sample 10% in production
	case "staging":
		return sdktrace.TraceIDRatioBased(0.5) // Sample 50% in staging
	default:
		return sdktrace.AlwaysSample() // Sample everything in development
	}
}

// StartSpan creates a new span with common attributes
func (ts *TelemetryService) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Return no-op span if tracer is not initialized (telemetry disabled)
	if ts.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	ctx, span := ts.tracer.Start(ctx, name, opts...)
	return ctx, span
}

// RecordAgentExecution records metrics for an agent execution
func (ts *TelemetryService) RecordAgentExecution(ctx context.Context, agentID int64, agentName, model string, duration time.Duration, success bool, tokenUsage map[string]interface{}) {
	// Skip if telemetry is disabled
	if ts.tracer == nil {
		return
	}
	// Common attributes
	attrs := []attribute.KeyValue{
		attribute.Int64("agent.id", agentID),
		attribute.String("agent.name", agentName),
		attribute.String("model.name", model),
		attribute.Bool("execution.success", success),
	}

	// Execution counter
	ts.agentExecutionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Duration histogram
	ts.agentExecutionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	// Token usage
	if tokenUsage != nil {
		if inputTokens, ok := extractInt64(tokenUsage["input_tokens"]); ok {
			ts.tokenUsageCounter.Add(ctx, inputTokens, metric.WithAttributes(
				append(attrs, attribute.String("token.type", "input"))...,
			))
		}
		if outputTokens, ok := extractInt64(tokenUsage["output_tokens"]); ok {
			ts.tokenUsageCounter.Add(ctx, outputTokens, metric.WithAttributes(
				append(attrs, attribute.String("token.type", "output"))...,
			))
		}
	}
}

// RecordToolCall records metrics for a tool call
func (ts *TelemetryService) RecordToolCall(ctx context.Context, toolName string, success bool, duration time.Duration) {
	// Skip if telemetry is disabled
	if ts.tracer == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
		attribute.Bool("tool.success", success),
	}

	ts.toolCallCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordError records an error metric
func (ts *TelemetryService) RecordError(ctx context.Context, errorType, component string) {
	// Skip if telemetry is disabled
	if ts.tracer == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("error.type", errorType),
		attribute.String("component", component),
	}

	ts.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// Shutdown gracefully shuts down the telemetry service
func (ts *TelemetryService) Shutdown(ctx context.Context) error {
	if ts.shutdownFunc != nil {
		return ts.shutdownFunc(ctx)
	}
	return nil
}

// ForceFlush forces immediate export of all pending spans
func (ts *TelemetryService) ForceFlush(ctx context.Context) error {
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return tp.ForceFlush(ctx)
	}
	return nil
}

// Helper functions

func getEnvironment() string {
	env := os.Getenv("STATION_ENVIRONMENT")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = "development"
	}
	return env
}

func extractInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	default:
		return 0, false
	}
}

// noOpExporter is a no-op span exporter for development
type noOpExporter struct{}

func (e *noOpExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *noOpExporter) Shutdown(ctx context.Context) error {
	return nil
}
