package coding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"station/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type OpenCodeBackend struct {
	client      *OpenCodeClient
	cfg         config.CodingConfig
	mu          sync.RWMutex
	sessions    map[string]*Session
	taskTimeout time.Duration
	tracer      trace.Tracer
}

func NewOpenCodeBackend(cfg config.CodingConfig) *OpenCodeBackend {
	url := OpenCodeURLFromConfig(cfg)
	maxAttempts := MaxAttemptsFromConfig(cfg)
	taskTimeout := TaskTimeoutFromConfig(cfg)

	client := NewOpenCodeClient(url,
		WithMaxAttempts(maxAttempts),
		WithRetryDelay(time.Second, 30*time.Second, 2.0),
	)

	return &OpenCodeBackend{
		client:      client,
		cfg:         cfg,
		sessions:    make(map[string]*Session),
		taskTimeout: taskTimeout,
		tracer:      otel.Tracer("station.coding"),
	}
}

func (b *OpenCodeBackend) Ping(ctx context.Context) error {
	return b.client.Health(ctx)
}

func (b *OpenCodeBackend) CreateSession(ctx context.Context, workspacePath, title string) (*Session, error) {
	backendID, err := b.client.CreateSession(ctx, workspacePath, title)
	if err != nil {
		return nil, &Error{Op: "CreateSession", Err: err}
	}

	session := &Session{
		ID:               fmt.Sprintf("coding_%d", time.Now().UnixNano()),
		BackendSessionID: backendID,
		WorkspacePath:    workspacePath,
		Title:            title,
		CreatedAt:        time.Now(),
		LastUsedAt:       time.Now(),
	}

	b.mu.Lock()
	b.sessions[session.ID] = session
	b.mu.Unlock()

	return session, nil
}

func (b *OpenCodeBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	b.mu.RLock()
	session, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &Error{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	return session, nil
}

func (b *OpenCodeBackend) CloseSession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.sessions[sessionID]; !ok {
		return &Error{Op: "CloseSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	delete(b.sessions, sessionID)
	return nil
}

func (b *OpenCodeBackend) Execute(ctx context.Context, sessionID string, task Task) (*Result, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	session.LastUsedAt = time.Now()
	b.mu.Unlock()

	timeout := task.Timeout
	if timeout == 0 {
		timeout = b.taskTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ctx, span := b.tracer.Start(ctx, "opencode.task",
		trace.WithAttributes(
			attribute.String("opencode.session_id", session.BackendSessionID),
			attribute.String("opencode.workspace", session.WorkspacePath),
		),
	)
	defer span.End()

	startTime := time.Now()

	prompt := b.buildPrompt(task, session.WorkspacePath)
	resp, err := b.client.SendMessage(ctx, session.BackendSessionID, session.WorkspacePath, prompt)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		if ctx.Err() == context.DeadlineExceeded {
			return &Result{
				Success: false,
				Error:   "task timed out",
				Trace: &Trace{
					SessionID: session.BackendSessionID,
					StartTime: startTime,
					EndTime:   endTime,
					Duration:  duration,
				},
			}, nil
		}
		return nil, &Error{Op: "Execute", Session: sessionID, Err: err}
	}

	span.SetAttributes(
		attribute.String("opencode.model", resp.Model),
		attribute.String("opencode.provider", resp.Provider),
		attribute.Float64("opencode.cost", resp.Cost),
		attribute.Int("opencode.tokens.input", resp.Tokens.Input),
		attribute.Int("opencode.tokens.output", resp.Tokens.Output),
		attribute.Int("opencode.tool_calls", len(resp.ToolCalls)),
	)

	for _, tc := range resp.ToolCalls {
		_, tcSpan := b.tracer.Start(ctx, "opencode.tool."+tc.Tool,
			trace.WithAttributes(
				attribute.String("tool.name", tc.Tool),
			),
		)
		if len(tc.Output) > 500 {
			tcSpan.SetAttributes(attribute.String("tool.output_preview", tc.Output[:500]+"..."))
		} else if tc.Output != "" {
			tcSpan.SetAttributes(attribute.String("tool.output", tc.Output))
		}
		tcSpan.End()
	}

	span.SetStatus(codes.Ok, "")

	return &Result{
		Success: true,
		Summary: resp.Text,
		Trace: &Trace{
			MessageID:    resp.ID,
			SessionID:    session.BackendSessionID,
			Model:        resp.Model,
			Provider:     resp.Provider,
			Cost:         resp.Cost,
			Tokens:       resp.Tokens,
			StartTime:    startTime,
			EndTime:      endTime,
			Duration:     duration,
			ToolCalls:    resp.ToolCalls,
			Reasoning:    resp.Reasoning,
			FinishReason: resp.FinishReason,
		},
	}, nil
}

func (b *OpenCodeBackend) buildPrompt(task Task, workspacePath string) string {
	prompt := fmt.Sprintf("IMPORTANT: Work in directory: %s\nAll file operations must use this path.\n\n%s", workspacePath, task.Instruction)

	if task.Context != "" {
		prompt = fmt.Sprintf("IMPORTANT: Work in directory: %s\nAll file operations must use this path.\n\nContext: %s\n\nTask: %s", workspacePath, task.Context, task.Instruction)
	}

	if len(task.Files) > 0 {
		prompt = fmt.Sprintf("%s\n\nFocus on these files: %v", prompt, task.Files)
	}

	return prompt
}
