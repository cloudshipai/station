package coding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"station/internal/config"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type NATSBackend struct {
	client      *NATSCodingClient
	cfg         config.CodingConfig
	mu          sync.RWMutex
	sessions    map[string]*Session
	taskTimeout time.Duration
	tracer      trace.Tracer
}

func NewNATSBackend(cfg config.CodingConfig) (*NATSBackend, error) {
	client, err := NewNATSCodingClient(cfg.NATS)
	if err != nil {
		return nil, fmt.Errorf("create NATS client: %w", err)
	}

	taskTimeout := TaskTimeoutFromConfig(cfg)

	return &NATSBackend{
		client:      client,
		cfg:         cfg,
		sessions:    make(map[string]*Session),
		taskTimeout: taskTimeout,
		tracer:      otel.Tracer("station.coding.nats"),
	}, nil
}

func (b *NATSBackend) Ping(ctx context.Context) error {
	if !b.client.IsConnected() {
		return fmt.Errorf("NATS not connected")
	}
	return nil
}

func (b *NATSBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	sessionName := fmt.Sprintf("session-%s", uuid.New().String()[:8])
	workspaceName := sessionName

	if opts.WorkspacePath != "" {
		workspaceName = opts.WorkspacePath
	}

	session := &Session{
		ID:               sessionName,
		BackendSessionID: "",
		WorkspacePath:    workspaceName,
		Title:            opts.Title,
		CreatedAt:        time.Now(),
		LastUsedAt:       time.Now(),
		Metadata:         make(map[string]string),
	}

	if opts.RepoURL != "" {
		session.Metadata["repo_url"] = opts.RepoURL
		if opts.Branch != "" {
			session.Metadata["branch"] = opts.Branch
		}
	}

	b.mu.Lock()
	b.sessions[session.ID] = session
	b.mu.Unlock()

	if opts.RepoURL != "" {
		cloneTask := b.buildCloneTask(opts.RepoURL, opts.Branch, opts.GitCredentials)
		result, err := b.executeTask(ctx, session, Task{Instruction: cloneTask})
		if err != nil {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("clone repo: %w", err)}
		}
		if !result.Success {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("clone repo failed: %s", result.Error)}
		}

		if result.Trace != nil && result.Trace.SessionID != "" {
			session.BackendSessionID = result.Trace.SessionID
			b.mu.Lock()
			b.sessions[session.ID] = session
			b.mu.Unlock()
		}
	}

	return session, nil
}

func (b *NATSBackend) buildCloneTask(repoURL, branch string, creds *GitCredentials) string {
	url := repoURL
	if creds != nil && creds.HasToken() {
		url = creds.InjectCredentials(repoURL)
	}

	if branch != "" {
		return fmt.Sprintf("Clone the git repository: git clone --branch %s %s . && git status", branch, url)
	}
	return fmt.Sprintf("Clone the git repository: git clone %s . && git status", url)
}

func (b *NATSBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	b.mu.RLock()
	session, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		state, err := b.client.GetSession(ctx, sessionID)
		if err != nil {
			return nil, &Error{Op: "GetSession", Session: sessionID, Err: err}
		}
		if state == nil {
			return nil, &Error{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
		}

		session = &Session{
			ID:               state.SessionName,
			BackendSessionID: state.OpencodeID,
			WorkspacePath:    state.WorkspacePath,
			Metadata:         make(map[string]string),
		}
		if t, err := time.Parse(time.RFC3339, state.Created); err == nil {
			session.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, state.LastUsed); err == nil {
			session.LastUsedAt = t
		}
		if state.Git != nil {
			session.Metadata["repo_url"] = state.Git.URL
			session.Metadata["branch"] = state.Git.Branch
		}

		b.mu.Lock()
		b.sessions[sessionID] = session
		b.mu.Unlock()
	}

	return session, nil
}

func (b *NATSBackend) CloseSession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	delete(b.sessions, sessionID)
	b.mu.Unlock()

	if err := b.client.DeleteSession(ctx, sessionID); err != nil {
		return &Error{Op: "CloseSession", Session: sessionID, Err: err}
	}

	return nil
}

func (b *NATSBackend) Execute(ctx context.Context, sessionID string, task Task) (*Result, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return b.executeTask(ctx, session, task)
}

func (b *NATSBackend) executeTask(ctx context.Context, session *Session, task Task) (*Result, error) {
	b.mu.Lock()
	session.LastUsedAt = time.Now()
	b.mu.Unlock()

	timeout := task.Timeout
	if timeout == 0 {
		timeout = b.taskTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ctx, span := b.tracer.Start(ctx, "opencode.nats.task",
		trace.WithAttributes(
			attribute.String("opencode.session_id", session.ID),
			attribute.String("opencode.workspace", session.WorkspacePath),
		),
	)
	defer span.End()

	startTime := time.Now()
	taskID := uuid.New().String()

	natsTask := &CodingTask{
		TaskID: taskID,
		Session: TaskSession{
			Name:     session.ID,
			Continue: true,
		},
		Workspace: TaskWorkspace{
			Name: session.WorkspacePath,
		},
		Prompt:  b.buildPrompt(task, session.WorkspacePath),
		Timeout: int(timeout.Milliseconds()),
	}

	if repoURL := session.Metadata["repo_url"]; repoURL != "" {
		branch := session.Metadata["branch"]
		natsTask.Workspace.Git = &TaskGitConfig{
			URL:    repoURL,
			Branch: branch,
			Pull:   true,
		}
	}

	exec, err := b.client.ExecuteTask(ctx, natsTask)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, &Error{Op: "Execute", Session: session.ID, Err: err}
	}

	var toolCalls []ToolCall
	eventCount := 0

	go func() {
		for event := range exec.Events() {
			eventCount++
			b.processStreamEvent(ctx, span, event, &toolCalls)
		}
	}()

	codingResult, err := exec.Wait(ctx)
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
					SessionID: session.ID,
					StartTime: startTime,
					EndTime:   endTime,
					Duration:  duration,
				},
			}, nil
		}
		return nil, &Error{Op: "Execute", Session: session.ID, Err: err}
	}

	if session.BackendSessionID == "" && codingResult.Session.OpencodeID != "" {
		session.BackendSessionID = codingResult.Session.OpencodeID
		b.mu.Lock()
		b.sessions[session.ID] = session
		b.mu.Unlock()
	}

	span.SetAttributes(
		attribute.Int("opencode.tool_calls", codingResult.Metrics.ToolCalls),
		attribute.Int("opencode.stream_events", codingResult.Metrics.StreamEvents),
		attribute.Int("opencode.duration_ms", codingResult.Metrics.Duration),
	)

	if codingResult.Status != "completed" {
		span.SetStatus(codes.Error, codingResult.Error)
		return &Result{
			Success: false,
			Error:   codingResult.Error,
			Trace: &Trace{
				SessionID: codingResult.Session.OpencodeID,
				StartTime: startTime,
				EndTime:   endTime,
				Duration:  duration,
				ToolCalls: toolCalls,
			},
		}, nil
	}

	span.SetStatus(codes.Ok, "")

	return &Result{
		Success: true,
		Summary: codingResult.Result,
		Trace: &Trace{
			SessionID: codingResult.Session.OpencodeID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
			ToolCalls: toolCalls,
			Tokens: TokenUsage{
				Input:  codingResult.Metrics.PromptTokens,
				Output: codingResult.Metrics.CompletionTokens,
			},
		},
	}, nil
}

func (b *NATSBackend) processStreamEvent(ctx context.Context, parentSpan trace.Span, event *CodingStreamEvent, toolCalls *[]ToolCall) {
	switch event.Type {
	case "tool_start":
		if event.Tool != nil {
			_, tcSpan := b.tracer.Start(ctx, "opencode.tool."+event.Tool.Name,
				trace.WithAttributes(
					attribute.String("tool.name", event.Tool.Name),
					attribute.String("tool.call_id", event.Tool.CallID),
				),
			)
			tcSpan.End()
		}
	case "tool_end":
		if event.Tool != nil {
			tc := ToolCall{
				Tool:     event.Tool.Name,
				Output:   event.Tool.Output,
				Duration: time.Duration(event.Tool.Duration) * time.Millisecond,
			}
			if event.Tool.Args != nil {
				tc.Input = event.Tool.Args
			}
			*toolCalls = append(*toolCalls, tc)
		}
	case "error":
		parentSpan.AddEvent("error", trace.WithAttributes(
			attribute.String("error.message", event.Content),
		))
	}
}

func (b *NATSBackend) buildPrompt(task Task, workspacePath string) string {
	prompt := task.Instruction

	if task.Context != "" {
		prompt = fmt.Sprintf("Context: %s\n\nTask: %s", task.Context, task.Instruction)
	}

	if len(task.Files) > 0 {
		prompt = fmt.Sprintf("%s\n\nFocus on these files: %v", prompt, task.Files)
	}

	return prompt
}

func (b *NATSBackend) GitCommit(ctx context.Context, sessionID string, message string, addAll bool) (*GitCommitResult, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var task string
	if addAll {
		task = fmt.Sprintf(`Run git commands to commit changes:
1. git add -A
2. git commit -m "%s"
3. git rev-parse HEAD (to get commit hash)
4. Report: commit hash, files changed, insertions, deletions`, message)
	} else {
		task = fmt.Sprintf(`Run git commands to commit staged changes:
1. git commit -m "%s"
2. git rev-parse HEAD (to get commit hash)
3. Report: commit hash, files changed, insertions, deletions`, message)
	}

	result, err := b.executeTask(ctx, session, Task{Instruction: task})
	if err != nil {
		return nil, err
	}

	commitResult := &GitCommitResult{
		Success: result.Success,
		Message: message,
	}

	if !result.Success {
		commitResult.Error = result.Error
		if commitResult.Error == "" {
			commitResult.Error = result.Summary
		}
		return commitResult, nil
	}

	commitResult.CommitHash, commitResult.FilesChanged, commitResult.Insertions, commitResult.Deletions =
		parseGitOutputFromSummary(result.Summary)

	return commitResult, nil
}

func (b *NATSBackend) GitPush(ctx context.Context, sessionID string, remote, branch string, setUpstream bool) (*GitPushResult, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if remote == "" {
		remote = "origin"
	}

	var task string
	if branch == "" {
		if setUpstream {
			task = fmt.Sprintf("Run: git push -u %s HEAD", remote)
		} else {
			task = fmt.Sprintf("Run: git push %s HEAD", remote)
		}
	} else {
		if setUpstream {
			task = fmt.Sprintf("Run: git push -u %s %s", remote, branch)
		} else {
			task = fmt.Sprintf("Run: git push %s %s", remote, branch)
		}
	}

	result, err := b.executeTask(ctx, session, Task{Instruction: task})
	if err != nil {
		return nil, err
	}

	pushResult := &GitPushResult{
		Remote:  remote,
		Branch:  branch,
		Success: result.Success,
	}

	if !result.Success {
		pushResult.Error = result.Error
		if pushResult.Error == "" {
			pushResult.Error = result.Summary
		}
	} else {
		pushResult.Message = result.Summary
	}

	return pushResult, nil
}

func (b *NATSBackend) Close() error {
	return b.client.Close()
}
