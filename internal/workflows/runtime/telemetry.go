package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"station/internal/workflows"
)

const (
	workflowTracerName = "station.workflows"
	workflowMeterName  = "station.workflows"
)

type WorkflowTelemetry struct {
	tracer trace.Tracer
	meter  metric.Meter

	runCounter     metric.Int64Counter
	runDuration    metric.Float64Histogram
	stepCounter    metric.Int64Counter
	stepDuration   metric.Float64Histogram
	activeRuns     metric.Int64UpDownCounter
	failureCounter metric.Int64Counter

	mu       sync.RWMutex
	runSpans map[string]trace.Span
}

func NewWorkflowTelemetry() (*WorkflowTelemetry, error) {
	wt := &WorkflowTelemetry{
		tracer:   otel.Tracer(workflowTracerName),
		meter:    otel.Meter(workflowMeterName),
		runSpans: make(map[string]trace.Span),
	}

	var err error

	wt.runCounter, err = wt.meter.Int64Counter(
		"station_workflow_runs_total",
		metric.WithDescription("Total number of workflow runs started"),
		metric.WithUnit("{run}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create run counter: %w", err)
	}

	wt.runDuration, err = wt.meter.Float64Histogram(
		"station_workflow_run_duration_seconds",
		metric.WithDescription("Duration of workflow runs in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create run duration histogram: %w", err)
	}

	wt.stepCounter, err = wt.meter.Int64Counter(
		"station_workflow_steps_total",
		metric.WithDescription("Total number of workflow steps executed"),
		metric.WithUnit("{step}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create step counter: %w", err)
	}

	wt.stepDuration, err = wt.meter.Float64Histogram(
		"station_workflow_step_duration_seconds",
		metric.WithDescription("Duration of workflow step execution in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create step duration histogram: %w", err)
	}

	wt.activeRuns, err = wt.meter.Int64UpDownCounter(
		"station_workflow_runs_active",
		metric.WithDescription("Number of currently active workflow runs"),
		metric.WithUnit("{run}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active runs counter: %w", err)
	}

	wt.failureCounter, err = wt.meter.Int64Counter(
		"station_workflow_failures_total",
		metric.WithDescription("Total number of workflow failures (runs + steps)"),
		metric.WithUnit("{failure}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create failure counter: %w", err)
	}

	return wt, nil
}

func (wt *WorkflowTelemetry) StartRunSpan(ctx context.Context, runID, workflowName string) context.Context {
	ctx, span := wt.tracer.Start(ctx, fmt.Sprintf("workflow.run.%s", workflowName),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("workflow.run_id", runID),
			attribute.String("workflow.name", workflowName),
		),
	)

	wt.mu.Lock()
	wt.runSpans[runID] = span
	wt.mu.Unlock()

	wt.runCounter.Add(ctx, 1,
		metric.WithAttributes(attribute.String("workflow.name", workflowName)),
	)
	wt.activeRuns.Add(ctx, 1,
		metric.WithAttributes(attribute.String("workflow.name", workflowName)),
	)

	return ctx
}

func (wt *WorkflowTelemetry) GetRunSpan(runID string) trace.Span {
	wt.mu.RLock()
	defer wt.mu.RUnlock()
	return wt.runSpans[runID]
}

func (wt *WorkflowTelemetry) EndRunSpan(ctx context.Context, runID, workflowName string, status string, duration time.Duration, err error) {
	wt.mu.Lock()
	span, exists := wt.runSpans[runID]
	if exists {
		delete(wt.runSpans, runID)
	}
	wt.mu.Unlock()

	if !exists || span == nil {
		return
	}

	span.SetAttributes(
		attribute.String("workflow.status", status),
		attribute.Float64("workflow.duration_seconds", duration.Seconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		wt.failureCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("workflow.name", workflowName),
				attribute.String("failure.type", "run"),
			),
		)
	} else if status == "completed" {
		span.SetStatus(codes.Ok, "workflow completed successfully")
	}

	span.End()

	wt.runDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			attribute.String("workflow.name", workflowName),
			attribute.String("workflow.status", status),
		),
	)

	wt.activeRuns.Add(ctx, -1,
		metric.WithAttributes(attribute.String("workflow.name", workflowName)),
	)
}

func (wt *WorkflowTelemetry) StartStepSpan(ctx context.Context, runID, stepID string, stepType workflows.ExecutionStepType) (context.Context, trace.Span) {
	ctx, span := wt.tracer.Start(ctx, fmt.Sprintf("workflow.step.%s", stepID),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("workflow.run_id", runID),
			attribute.String("workflow.step_id", stepID),
			attribute.String("workflow.step_type", string(stepType)),
		),
	)

	wt.stepCounter.Add(ctx, 1,
		metric.WithAttributes(attribute.String("workflow.step_type", string(stepType))),
	)

	return ctx, span
}

func (wt *WorkflowTelemetry) EndStepSpan(span trace.Span, stepType workflows.ExecutionStepType, status StepStatus, duration time.Duration, err error) {
	if span == nil {
		return
	}

	span.SetAttributes(
		attribute.String("workflow.step_status", string(status)),
		attribute.Float64("workflow.step_duration_seconds", duration.Seconds()),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else if status == StepStatusCompleted || status == StepStatusApproved {
		span.SetStatus(codes.Ok, "step completed")
	}

	span.End()

	ctx := context.Background()
	wt.stepDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			attribute.String("workflow.step_type", string(stepType)),
			attribute.String("workflow.step_status", string(status)),
		),
	)

	if err != nil || status == StepStatusFailed {
		wt.failureCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("workflow.step_type", string(stepType)),
				attribute.String("failure.type", "step"),
			),
		)
	}
}

// NATSTraceCarrier implements propagation.TextMapCarrier for NATS trace propagation.
type NATSTraceCarrier struct {
	headers map[string]string
}

func NewNATSTraceCarrier() *NATSTraceCarrier {
	return &NATSTraceCarrier{headers: make(map[string]string)}
}

func NewNATSTraceCarrierFromHeaders(headers map[string]string) *NATSTraceCarrier {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &NATSTraceCarrier{headers: headers}
}

func (c *NATSTraceCarrier) Get(key string) string {
	return c.headers[key]
}

func (c *NATSTraceCarrier) Set(key, value string) {
	c.headers[key] = value
}

func (c *NATSTraceCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

func (c *NATSTraceCarrier) Headers() map[string]string {
	return c.headers
}

func InjectTraceContext(ctx context.Context, carrier *NATSTraceCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

func ExtractTraceContext(ctx context.Context, carrier *NATSTraceCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

type StepScheduleMessage struct {
	Step         workflows.ExecutionStep `json:"step"`
	TraceContext map[string]string       `json:"trace_context,omitempty"`
}

func MarshalStepWithTrace(ctx context.Context, step workflows.ExecutionStep) ([]byte, error) {
	carrier := NewNATSTraceCarrier()
	InjectTraceContext(ctx, carrier)
	return json.Marshal(StepScheduleMessage{
		Step:         step,
		TraceContext: carrier.Headers(),
	})
}

func UnmarshalStepWithTrace(ctx context.Context, data []byte) (workflows.ExecutionStep, context.Context, error) {
	var msg StepScheduleMessage
	if err := json.Unmarshal(data, &msg); err == nil && msg.Step.ID != "" {
		if len(msg.TraceContext) > 0 {
			carrier := NewNATSTraceCarrierFromHeaders(msg.TraceContext)
			ctx = ExtractTraceContext(ctx, carrier)
		}
		return msg.Step, ctx, nil
	}

	var step workflows.ExecutionStep
	if err := json.Unmarshal(data, &step); err != nil {
		return workflows.ExecutionStep{}, ctx, err
	}
	return step, ctx, nil
}
