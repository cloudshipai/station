package trace

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "station/pkg/harness"

type Tracer struct {
	tracer trace.Tracer
}

func New() *Tracer {
	return &Tracer{
		tracer: otel.Tracer(TracerName),
	}
}

func NewWithTracer(t trace.Tracer) *Tracer {
	return &Tracer{tracer: t}
}

type ExecutionSpan struct {
	span      trace.Span
	startTime time.Time
}

func (t *Tracer) StartExecution(ctx context.Context, runID, agentID, agentName string) (context.Context, *ExecutionSpan) {
	ctx, span := t.tracer.Start(ctx, "agent_execution",
		trace.WithAttributes(
			attribute.String("harness.run_id", runID),
			attribute.String("harness.agent_id", agentID),
			attribute.String("harness.agent_name", agentName),
			attribute.String("harness.type", "agentic"),
		),
	)

	return ctx, &ExecutionSpan{
		span:      span,
		startTime: time.Now(),
	}
}

func (s *ExecutionSpan) SetWorkflow(workflowID, workflowRunID, stepName string) {
	s.span.SetAttributes(
		attribute.String("harness.workflow_id", workflowID),
		attribute.String("harness.workflow_run_id", workflowRunID),
		attribute.String("harness.step_name", stepName),
	)
}

func (s *ExecutionSpan) End(status string, totalSteps int, err error) {
	duration := time.Since(s.startTime)

	s.span.SetAttributes(
		attribute.String("harness.status", status),
		attribute.Int("harness.total_steps", totalSteps),
		attribute.Int64("harness.duration_ms", duration.Milliseconds()),
	)

	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	} else {
		s.span.SetStatus(codes.Ok, "")
	}

	s.span.End()
}

type SetupSpan struct {
	span      trace.Span
	startTime time.Time
}

func (t *Tracer) StartSetup(ctx context.Context) (context.Context, *SetupSpan) {
	ctx, span := t.tracer.Start(ctx, "harness_setup")
	return ctx, &SetupSpan{
		span:      span,
		startTime: time.Now(),
	}
}

func (s *SetupSpan) SetWorkspace(path, mode string) {
	s.span.SetAttributes(
		attribute.String("setup.workspace_path", path),
		attribute.String("setup.workspace_mode", mode),
	)
}

func (s *SetupSpan) SetGit(branch string, branchCreated bool) {
	s.span.SetAttributes(
		attribute.String("setup.git_branch", branch),
		attribute.Bool("setup.branch_created", branchCreated),
	)
}

func (s *SetupSpan) SetPreloadedFiles(count int, bytes int64) {
	s.span.SetAttributes(
		attribute.Int("setup.preload_files_count", count),
		attribute.Int64("setup.preload_bytes", bytes),
	)
}

func (s *SetupSpan) End(err error) {
	duration := time.Since(s.startTime)
	s.span.SetAttributes(attribute.Int64("setup.duration_ms", duration.Milliseconds()))

	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()
}

type LoopStepSpan struct {
	span      trace.Span
	startTime time.Time
	stepNum   int
}

func (t *Tracer) StartLoopStep(ctx context.Context, stepNum int) (context.Context, *LoopStepSpan) {
	ctx, span := t.tracer.Start(ctx, "agentic_loop_step",
		trace.WithAttributes(
			attribute.Int("step.number", stepNum),
		),
	)

	return ctx, &LoopStepSpan{
		span:      span,
		startTime: time.Now(),
		stepNum:   stepNum,
	}
}

func (s *LoopStepSpan) End(finishReason string) {
	duration := time.Since(s.startTime)
	s.span.SetAttributes(
		attribute.Int64("step.duration_ms", duration.Milliseconds()),
		attribute.String("step.finish_reason", finishReason),
	)
	s.span.End()
}

type LLMGenerateSpan struct {
	span      trace.Span
	startTime time.Time
}

func (t *Tracer) StartLLMGenerate(ctx context.Context, model string) (context.Context, *LLMGenerateSpan) {
	ctx, span := t.tracer.Start(ctx, "llm_generate",
		trace.WithAttributes(
			attribute.String("llm.model", model),
		),
	)

	return ctx, &LLMGenerateSpan{
		span:      span,
		startTime: time.Now(),
	}
}

func (s *LLMGenerateSpan) SetTokens(input, output int) {
	s.span.SetAttributes(
		attribute.Int("llm.input_tokens", input),
		attribute.Int("llm.output_tokens", output),
	)
}

func (s *LLMGenerateSpan) SetToolRequests(tools []string) {
	s.span.SetAttributes(
		attribute.StringSlice("llm.tool_requests", tools),
		attribute.Int("llm.tool_request_count", len(tools)),
	)
}

func (s *LLMGenerateSpan) SetThinking(thinking string) {
	if len(thinking) > 500 {
		thinking = thinking[:500] + "..."
	}
	s.span.SetAttributes(attribute.String("llm.thinking", thinking))
}

func (s *LLMGenerateSpan) End(err error) {
	duration := time.Since(s.startTime)
	s.span.SetAttributes(attribute.Int64("llm.duration_ms", duration.Milliseconds()))

	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()
}

type ToolExecutionSpan struct {
	span      trace.Span
	startTime time.Time
}

func (t *Tracer) StartToolExecution(ctx context.Context, toolName, callID string) (context.Context, *ToolExecutionSpan) {
	ctx, span := t.tracer.Start(ctx, "tool_execution",
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
			attribute.String("tool.call_id", callID),
		),
	)

	return ctx, &ToolExecutionSpan{
		span:      span,
		startTime: time.Now(),
	}
}

func (s *ToolExecutionSpan) SetInput(input map[string]interface{}) {
	for k, v := range input {
		if str, ok := v.(string); ok {
			if len(str) > 200 {
				str = str[:200] + "..."
			}
			s.span.SetAttributes(attribute.String("tool.input."+k, str))
		}
	}
}

func (s *ToolExecutionSpan) SetOutput(outputBytes int, truncated bool) {
	s.span.SetAttributes(
		attribute.Int("tool.output_bytes", outputBytes),
		attribute.Bool("tool.output_truncated", truncated),
	)
}

func (s *ToolExecutionSpan) SetHookResult(hookType, result string) {
	s.span.SetAttributes(attribute.String("tool.hook."+hookType, result))
}

func (s *ToolExecutionSpan) End(success bool, err error) {
	duration := time.Since(s.startTime)
	s.span.SetAttributes(
		attribute.Int64("tool.duration_ms", duration.Milliseconds()),
		attribute.Bool("tool.success", success),
	)

	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()
}

type CompactionSpan struct {
	span      trace.Span
	startTime time.Time
}

func (t *Tracer) StartCompaction(ctx context.Context, currentTokens, threshold int) (context.Context, *CompactionSpan) {
	ctx, span := t.tracer.Start(ctx, "compaction",
		trace.WithAttributes(
			attribute.Int("compaction.current_tokens", currentTokens),
			attribute.Int("compaction.threshold", threshold),
		),
	)

	return ctx, &CompactionSpan{
		span:      span,
		startTime: time.Now(),
	}
}

func (s *CompactionSpan) SetResult(triggered bool, messagesBefore, messagesAfter, tokensBefore, tokensAfter int) {
	s.span.SetAttributes(
		attribute.Bool("compaction.triggered", triggered),
		attribute.Int("compaction.messages_before", messagesBefore),
		attribute.Int("compaction.messages_after", messagesAfter),
		attribute.Int("compaction.tokens_before", tokensBefore),
		attribute.Int("compaction.tokens_after", tokensAfter),
	)
}

func (s *CompactionSpan) End(err error) {
	duration := time.Since(s.startTime)
	s.span.SetAttributes(attribute.Int64("compaction.duration_ms", duration.Milliseconds()))

	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()
}

type CleanupSpan struct {
	span      trace.Span
	startTime time.Time
}

func (t *Tracer) StartCleanup(ctx context.Context) (context.Context, *CleanupSpan) {
	ctx, span := t.tracer.Start(ctx, "harness_cleanup")
	return ctx, &CleanupSpan{
		span:      span,
		startTime: time.Now(),
	}
}

func (s *CleanupSpan) SetGitCommit(sha string) {
	s.span.SetAttributes(attribute.String("cleanup.git_commit", sha))
}

func (s *CleanupSpan) SetUploadedFiles(count int) {
	s.span.SetAttributes(attribute.Int("cleanup.uploaded_files", count))
}

func (s *CleanupSpan) SetSavedContext(key string) {
	s.span.SetAttributes(attribute.String("cleanup.context_key", key))
}

func (s *CleanupSpan) End(err error) {
	duration := time.Since(s.startTime)
	s.span.SetAttributes(attribute.Int64("cleanup.duration_ms", duration.Milliseconds()))

	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()
}

func (t *Tracer) RecordEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

func (t *Tracer) RecordDoomLoopDetected(ctx context.Context, toolName string, consecutiveCount int) {
	t.RecordEvent(ctx, "doom_loop_detected",
		attribute.String("doom_loop.tool", toolName),
		attribute.Int("doom_loop.consecutive_count", consecutiveCount),
	)
}

func (t *Tracer) RecordPermissionDenied(ctx context.Context, toolName, pattern, command string) {
	t.RecordEvent(ctx, "permission_denied",
		attribute.String("permission.tool", toolName),
		attribute.String("permission.pattern", pattern),
		attribute.String("permission.command", command),
	)
}

func (t *Tracer) RecordApprovalRequired(ctx context.Context, toolName, reason string) {
	t.RecordEvent(ctx, "approval_required",
		attribute.String("approval.tool", toolName),
		attribute.String("approval.reason", reason),
	)
}
