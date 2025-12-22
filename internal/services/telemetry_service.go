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
	"station/internal/config"
	"station/internal/logging"
)

const (
	serviceName    = "station"
	serviceVersion = "0.2.7"

	// CloudShip telemetry endpoint
	cloudShipTelemetryEndpoint = "telemetry.cloudshipai.com"
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

	// CloudShip attribute processor - adds org_id/station_name to ALL spans
	cloudShipProcessor *cloudShipAttributeProcessor
}

// TelemetryConfig holds configuration for telemetry services
// This wraps config.TelemetryConfig and adds runtime fields
type TelemetryConfig struct {
	Enabled     bool
	Provider    config.TelemetryProvider
	Endpoint    string            // OTLP endpoint URL
	Headers     map[string]string // Custom headers (for OTLP provider)
	ServiceName string
	Environment string
	SampleRate  float64

	// CloudShip telemetry authentication (used when Provider = "cloudship")
	CloudShipAPIKey string // Registration key for CloudShip telemetry endpoint

	// CloudShip resource attributes (added to all traces for filtering)
	StationName string // Station name for trace filtering
	StationID   string // Station ID (UUID) for trace filtering
	OrgID       string // Organization ID for multi-tenant filtering
}

// CloudShipInfo holds CloudShip-specific information for telemetry
type CloudShipInfo struct {
	RegistrationKey string
	StationName     string
	StationID       string
	OrgID           string
}

// NewTelemetryConfigFromConfig creates a TelemetryConfig from config.TelemetryConfig
func NewTelemetryConfigFromConfig(cfg *config.TelemetryConfig, cloudShipInfo *CloudShipInfo) *TelemetryConfig {
	if cfg == nil {
		return &TelemetryConfig{
			Enabled:     true,
			Provider:    config.TelemetryProviderJaeger,
			Endpoint:    "http://localhost:4318",
			ServiceName: serviceName,
			Environment: "development",
			SampleRate:  1.0,
			Headers:     make(map[string]string),
		}
	}

	tc := &TelemetryConfig{
		Enabled:     cfg.Enabled,
		Provider:    cfg.Provider,
		Endpoint:    cfg.Endpoint,
		Headers:     cfg.Headers,
		ServiceName: cfg.ServiceName,
		Environment: cfg.Environment,
		SampleRate:  cfg.SampleRate,
	}

	// Set defaults
	if tc.ServiceName == "" {
		tc.ServiceName = serviceName
	}
	if tc.Environment == "" {
		tc.Environment = "development"
	}
	if tc.SampleRate <= 0 {
		tc.SampleRate = 1.0
	}
	if tc.Headers == nil {
		tc.Headers = make(map[string]string)
	}

	// Apply CloudShip info if provided
	if cloudShipInfo != nil {
		tc.CloudShipAPIKey = cloudShipInfo.RegistrationKey
		tc.StationName = cloudShipInfo.StationName
		tc.StationID = cloudShipInfo.StationID
		tc.OrgID = cloudShipInfo.OrgID
	}

	// Handle CloudShip provider
	if tc.Provider == config.TelemetryProviderCloudShip {
		tc.Endpoint = "https://" + cloudShipTelemetryEndpoint
	}

	// Handle Jaeger provider defaults
	if tc.Provider == config.TelemetryProviderJaeger && tc.Endpoint == "" {
		tc.Endpoint = "http://localhost:4318"
	}

	return tc
}

// NewTelemetryService creates a new telemetry service with configuration
func NewTelemetryService(cfg *TelemetryConfig) *TelemetryService {
	// Set defaults if nil config provided
	if cfg == nil {
		cfg = &TelemetryConfig{
			Enabled:     true,
			Provider:    config.TelemetryProviderJaeger,
			Endpoint:    "http://localhost:4318",
			ServiceName: serviceName,
			Environment: "development",
			SampleRate:  1.0,
			Headers:     make(map[string]string),
		}
	}

	// Ensure Headers map is initialized
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}

	return &TelemetryService{
		config: cfg,
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

	// Build resource attributes
	resourceAttrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(serviceVersion),
		semconv.DeploymentEnvironmentKey.String(ts.config.Environment),
	}

	// Add CloudShip-specific attributes for filtering in the platform UI
	if ts.config.StationName != "" {
		resourceAttrs = append(resourceAttrs, attribute.String("cloudship.station_name", ts.config.StationName))
	}
	if ts.config.StationID != "" {
		resourceAttrs = append(resourceAttrs, attribute.String("cloudship.station_id", ts.config.StationID))
	}
	if ts.config.OrgID != "" {
		resourceAttrs = append(resourceAttrs, attribute.String("cloudship.org_id", ts.config.OrgID))
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(resourceAttrs...),
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

	// Determine exporter based on provider configuration
	switch ts.config.Provider {
	case config.TelemetryProviderNone:
		// Telemetry disabled - use no-op exporter
		exporter = &noOpExporter{}

	case config.TelemetryProviderJaeger:
		// Local Jaeger - HTTP, no auth, insecure
		endpoint := ts.config.Endpoint
		if endpoint == "" {
			endpoint = "http://localhost:4318"
		}
		endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")

		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithInsecure(), // Jaeger local is always insecure
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Jaeger OTLP exporter: %w", err)
		}

	case config.TelemetryProviderCloudShip:
		// CloudShip managed telemetry - HTTPS with registration key auth
		endpoint := cloudShipTelemetryEndpoint
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
			// TLS is automatic for non-insecure connections
		}

		// Add CloudShip API key header (registration key)
		if ts.config.CloudShipAPIKey != "" {
			opts = append(opts, otlptracehttp.WithHeaders(map[string]string{
				"X-CloudShip-API-Key": ts.config.CloudShipAPIKey,
			}))
		}

		exporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create CloudShip OTLP exporter: %w", err)
		}

	case config.TelemetryProviderOTLP:
		// Custom OTLP endpoint - user provides endpoint and optional headers
		endpoint := ts.config.Endpoint
		if endpoint == "" {
			// Fall back to environment variables
			endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
			if endpoint == "" {
				endpoint = os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
			}
		}

		if endpoint == "" {
			// No endpoint configured - use no-op
			exporter = &noOpExporter{}
		} else {
			// Check if gRPC protocol is requested
			if os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL") == "grpc" {
				exporter, err = otlptracegrpc.New(ctx)
			} else {
				// Parse the endpoint URL to extract just the host:port
				useHTTPS := strings.HasPrefix(endpoint, "https://")
				endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")

				opts := []otlptracehttp.Option{
					otlptracehttp.WithEndpoint(endpoint),
				}

				// Use TLS for HTTPS endpoints
				if !useHTTPS {
					opts = append(opts, otlptracehttp.WithInsecure())
				}

				// Add custom headers if configured
				if len(ts.config.Headers) > 0 {
					opts = append(opts, otlptracehttp.WithHeaders(ts.config.Headers))
				}

				exporter, err = otlptracehttp.New(ctx, opts...)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to create custom OTLP exporter: %w", err)
			}
		}

	default:
		// Unknown provider - try to auto-detect from endpoint or use no-op
		endpoint := ts.config.Endpoint
		if endpoint == "" {
			exporter = &noOpExporter{}
		} else {
			// Auto-detect based on endpoint
			useHTTPS := strings.HasPrefix(endpoint, "https://")
			endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")

			opts := []otlptracehttp.Option{
				otlptracehttp.WithEndpoint(endpoint),
			}

			if !useHTTPS {
				opts = append(opts, otlptracehttp.WithInsecure())
			}

			// Add CloudShip key if configured
			if ts.config.CloudShipAPIKey != "" {
				opts = append(opts, otlptracehttp.WithHeaders(map[string]string{
					"X-CloudShip-API-Key": ts.config.CloudShipAPIKey,
				}))
			}

			exporter, err = otlptracehttp.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
			}
		}
	}

	// Create CloudShip attribute processor to add org_id/station_name to ALL spans
	// This is necessary because resources are immutable but CloudShip auth happens after init
	ts.cloudShipProcessor = newCloudShipAttributeProcessor(ts.config)

	// Build list of span processors
	var spanProcessors []sdktrace.TracerProviderOption

	// Add primary exporter
	spanProcessors = append(spanProcessors, sdktrace.WithBatcher(exporter,
		sdktrace.WithBatchTimeout(time.Second*1), // Reduced timeout for faster export
		sdktrace.WithMaxExportBatchSize(1),       // Export immediately
		sdktrace.WithExportTimeout(time.Second*10),
	))

	// Check for dual export to local Jaeger (useful for debugging)
	// Set OTEL_EXPORTER_JAEGER_ENDPOINT=localhost:4318 to enable
	jaegerEndpoint := os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT")
	if jaegerEndpoint != "" && ts.config.Provider == config.TelemetryProviderCloudShip {
		// Create additional Jaeger exporter for local debugging
		jaegerOpts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(jaegerEndpoint),
			otlptracehttp.WithInsecure(), // Local Jaeger is always insecure
		}
		jaegerExporter, jaegerErr := otlptracehttp.New(ctx, jaegerOpts...)
		if jaegerErr != nil {
			logging.Info("Warning: Failed to create Jaeger exporter for dual export: %v", jaegerErr)
		} else {
			spanProcessors = append(spanProcessors, sdktrace.WithBatcher(jaegerExporter,
				sdktrace.WithBatchTimeout(time.Second*1),
				sdktrace.WithMaxExportBatchSize(1),
				sdktrace.WithExportTimeout(time.Second*5),
			))
			logging.Info("📊 Dual export enabled: CloudShip + Jaeger (%s)", jaegerEndpoint)
		}
	}

	// Add CloudShip attribute processor
	spanProcessors = append(spanProcessors, sdktrace.WithSpanProcessor(ts.cloudShipProcessor))

	// Create trace provider with resource and all processors
	providerOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(ts.getSampler()),
	}
	providerOpts = append(providerOpts, spanProcessors...)

	tp := sdktrace.NewTracerProvider(providerOpts...)

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
	// Use configured sample rate if set
	if ts.config != nil && ts.config.SampleRate > 0 && ts.config.SampleRate < 1.0 {
		return sdktrace.TraceIDRatioBased(ts.config.SampleRate)
	}

	// Fall back to environment-based defaults
	env := ts.config.Environment
	if env == "" {
		env = getEnvironment()
	}
	switch env {
	case "production":
		return sdktrace.TraceIDRatioBased(0.1) // Sample 10% in production
	case "staging":
		return sdktrace.TraceIDRatioBased(0.5) // Sample 50% in staging
	default:
		return sdktrace.AlwaysSample() // Sample everything in development
	}
}

// SetCloudShipInfo updates the CloudShip info after authentication
// This is called when ManagementChannel receives AuthResult from CloudShip
// Since OTEL resources are immutable, we need to re-initialize the tracer provider
// with the correct resource attributes for CloudShip platform filtering
func (ts *TelemetryService) SetCloudShipInfo(stationID, stationName, orgID string) {
	if ts.config == nil {
		return
	}
	ts.config.StationID = stationID
	ts.config.StationName = stationName
	ts.config.OrgID = orgID
	logging.Info("Telemetry CloudShip info updated: station_id=%s station_name=%s org_id=%s", stationID, stationName, orgID)

	// Re-initialize the tracer provider with correct resource attributes
	// This is necessary because OTEL resources are immutable and CloudShip auth
	// happens AFTER initial telemetry setup. CloudShip's Tempo queries filter by
	// resource attributes (resource.cloudship.station_id), not span attributes.
	if err := ts.reinitializeWithCloudShipResources(); err != nil {
		logging.Info("Warning: Failed to reinitialize telemetry with CloudShip resources: %v", err)
	}
}

// reinitializeWithCloudShipResources shuts down the current tracer provider and
// creates a new one with CloudShip attributes as resource attributes
func (ts *TelemetryService) reinitializeWithCloudShipResources() error {
	ctx := context.Background()

	// Shutdown existing provider to flush pending spans
	if ts.shutdownFunc != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := ts.shutdownFunc(shutdownCtx); err != nil {
			logging.Info("Warning: Error shutting down previous tracer provider: %v", err)
		}
	}

	// Re-initialize with updated resource attributes
	return ts.Initialize(ctx)
}

// StartSpan creates a new span with common attributes
func (ts *TelemetryService) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Return no-op span if tracer is not initialized (telemetry disabled)
	if ts.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	ctx, span := ts.tracer.Start(ctx, name, opts...)

	// Add CloudShip attributes to every span for filtering
	// This is needed because resources are set at init time before CloudShip auth
	if ts.config != nil {
		if ts.config.StationID != "" {
			span.SetAttributes(attribute.String("cloudship.station_id", ts.config.StationID))
		}
		if ts.config.StationName != "" {
			span.SetAttributes(attribute.String("cloudship.station_name", ts.config.StationName))
		}
		if ts.config.OrgID != "" {
			span.SetAttributes(attribute.String("cloudship.org_id", ts.config.OrgID))
		}
	}

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

// cloudShipAttributeProcessor is a SpanProcessor that adds CloudShip attributes
// (org_id, station_id, station_name) to ALL spans, including those created by
// external libraries like Genkit that don't use TelemetryService.StartSpan().
//
// This solves the problem that OTEL resources are immutable and set at init time,
// but CloudShip auth happens later. By using a SpanProcessor, we can add attributes
// to spans at the time they are started, when we have the auth info available.
type cloudShipAttributeProcessor struct {
	config *TelemetryConfig
}

// newCloudShipAttributeProcessor creates a new CloudShip attribute processor
func newCloudShipAttributeProcessor(config *TelemetryConfig) *cloudShipAttributeProcessor {
	return &cloudShipAttributeProcessor{config: config}
}

// OnStart is called when a span is started. We add CloudShip attributes here.
func (p *cloudShipAttributeProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if p.config == nil {
		logging.Info("SpanProcessor OnStart: config is nil!")
		return
	}

	// Debug: Always log first few spans to verify processor is being called
	logging.Info("SpanProcessor OnStart: span=%s org_id='%s' station_name='%s' station_id='%s'",
		s.Name(), p.config.OrgID, p.config.StationName, p.config.StationID)

	// Add CloudShip attributes to the span for multi-tenant filtering
	// These attributes are crucial for the CloudShip platform to filter traces by org
	if p.config.StationID != "" {
		s.SetAttributes(attribute.String("cloudship.station_id", p.config.StationID))
	}
	if p.config.StationName != "" {
		s.SetAttributes(attribute.String("cloudship.station_name", p.config.StationName))
	}
	if p.config.OrgID != "" {
		s.SetAttributes(attribute.String("cloudship.org_id", p.config.OrgID))
	}
}

// OnEnd is called when a span ends. No action needed.
func (p *cloudShipAttributeProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	// No action needed on span end
}

// Shutdown shuts down the processor.
func (p *cloudShipAttributeProcessor) Shutdown(ctx context.Context) error {
	return nil
}

// ForceFlush forces a flush of any pending spans.
func (p *cloudShipAttributeProcessor) ForceFlush(ctx context.Context) error {
	return nil
}
