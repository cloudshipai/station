package lattice

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "station.lattice"

type Telemetry struct {
	tracer trace.Tracer
	meter  metric.Meter

	agentInvocations    metric.Int64Counter
	workflowInvocations metric.Int64Counter
	invocationDuration  metric.Float64Histogram
	registryOperations  metric.Int64Counter
	presenceHeartbeats  metric.Int64Counter
	natsRequests        metric.Int64Counter
	natsRequestDuration metric.Float64Histogram
	errorCounter        metric.Int64Counter
}

func NewTelemetry() *Telemetry {
	tracer := otel.Tracer(tracerName)
	meter := otel.Meter(tracerName)

	t := &Telemetry{
		tracer: tracer,
		meter:  meter,
	}

	t.agentInvocations, _ = meter.Int64Counter("lattice.agent.invocations",
		metric.WithDescription("Number of remote agent invocations"))

	t.workflowInvocations, _ = meter.Int64Counter("lattice.workflow.invocations",
		metric.WithDescription("Number of remote workflow invocations"))

	t.invocationDuration, _ = meter.Float64Histogram("lattice.invocation.duration_ms",
		metric.WithDescription("Duration of remote invocations in milliseconds"))

	t.registryOperations, _ = meter.Int64Counter("lattice.registry.operations",
		metric.WithDescription("Number of registry operations"))

	t.presenceHeartbeats, _ = meter.Int64Counter("lattice.presence.heartbeats",
		metric.WithDescription("Number of presence heartbeats sent"))

	t.natsRequests, _ = meter.Int64Counter("lattice.nats.requests",
		metric.WithDescription("Number of NATS requests"))

	t.natsRequestDuration, _ = meter.Float64Histogram("lattice.nats.duration_ms",
		metric.WithDescription("Duration of NATS requests in milliseconds"))

	t.errorCounter, _ = meter.Int64Counter("lattice.errors",
		metric.WithDescription("Number of lattice errors"))

	return t
}

func (t *Telemetry) StartAgentInvocationSpan(ctx context.Context, targetStation, agentID, agentName, task string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, "lattice.agent.invoke",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("lattice.target_station", targetStation),
			attribute.String("lattice.agent_id", agentID),
			attribute.String("lattice.agent_name", agentName),
			attribute.String("lattice.task", truncateString(task, 200)),
		))

	return ctx, span
}

func (t *Telemetry) EndAgentInvocationSpan(span trace.Span, status, result string, toolCalls int, durationMs float64, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		t.errorCounter.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("error_type", "agent_invocation"),
		))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("lattice.status", status),
		attribute.Int("lattice.tool_calls", toolCalls),
		attribute.Float64("lattice.duration_ms", durationMs),
	)

	t.agentInvocations.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("status", status),
	))
	t.invocationDuration.Record(context.Background(), durationMs)

	span.End()
}

func (t *Telemetry) StartWorkflowInvocationSpan(ctx context.Context, targetStation, workflowID string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, "lattice.workflow.invoke",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("lattice.target_station", targetStation),
			attribute.String("lattice.workflow_id", workflowID),
		))

	return ctx, span
}

func (t *Telemetry) EndWorkflowInvocationSpan(span trace.Span, status, runID string, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		t.errorCounter.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("error_type", "workflow_invocation"),
		))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.SetAttributes(
		attribute.String("lattice.status", status),
		attribute.String("lattice.run_id", runID),
	)

	t.workflowInvocations.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("status", status),
	))

	span.End()
}

func (t *Telemetry) StartHandleAgentRequestSpan(ctx context.Context, stationID, agentID, agentName string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, "lattice.agent.handle",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("lattice.station_id", stationID),
			attribute.String("lattice.agent_id", agentID),
			attribute.String("lattice.agent_name", agentName),
		))

	return ctx, span
}

func (t *Telemetry) StartHandleWorkflowRequestSpan(ctx context.Context, stationID, workflowID string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, "lattice.workflow.handle",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("lattice.station_id", stationID),
			attribute.String("lattice.workflow_id", workflowID),
		))

	return ctx, span
}

func (t *Telemetry) StartRegistrySpan(ctx context.Context, operation, stationID string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, "lattice.registry."+operation,
		trace.WithAttributes(
			attribute.String("lattice.operation", operation),
			attribute.String("lattice.station_id", stationID),
		))

	t.registryOperations.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", operation),
	))

	return ctx, span
}

func (t *Telemetry) StartNATSRequestSpan(ctx context.Context, subject string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, "lattice.nats.request",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("nats.subject", subject),
		))

	return ctx, span
}

func (t *Telemetry) EndNATSRequestSpan(span trace.Span, durationMs float64, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		t.natsRequests.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("status", "error"),
		))
	} else {
		span.SetStatus(codes.Ok, "")
		t.natsRequests.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("status", "success"),
		))
	}

	span.SetAttributes(attribute.Float64("nats.duration_ms", durationMs))
	t.natsRequestDuration.Record(context.Background(), durationMs)

	span.End()
}

func (t *Telemetry) RecordHeartbeat(stationID string) {
	t.presenceHeartbeats.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("station_id", stationID),
	))
}

func (t *Telemetry) RecordError(errorType, component string, err error) {
	t.errorCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("error_type", errorType),
		attribute.String("component", component),
	))
}

func (t *Telemetry) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

func (t *Telemetry) EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

type TimedOperation struct {
	start time.Time
}

func StartTimer() *TimedOperation {
	return &TimedOperation{start: time.Now()}
}

func (t *TimedOperation) ElapsedMs() float64 {
	return float64(time.Since(t.start).Milliseconds())
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
