package coding

import (
	"context"
	"fmt"
	"strings"
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

func (b *OpenCodeBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	workspacePath := runID

	if opts.WorkspacePath != "" {
		workspacePath = opts.WorkspacePath
	}

	backendID, err := b.client.CreateSession(ctx, "", opts.Title)
	if err != nil {
		return nil, &Error{Op: "CreateSession", Err: err}
	}

	session := &Session{
		ID:               fmt.Sprintf("coding_%d", time.Now().UnixNano()),
		BackendSessionID: backendID,
		WorkspacePath:    workspacePath,
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

		cloneTask := b.buildCloneTask(opts.RepoURL, opts.Branch, opts.GitCredentials)
		result, err := b.executeInternal(ctx, session, Task{Instruction: cloneTask})
		if err != nil {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("clone repo: %w", err)}
		}
		if !result.Success {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("clone repo failed: %s", result.Error)}
		}
	}

	b.mu.Lock()
	b.sessions[session.ID] = session
	b.mu.Unlock()

	return session, nil
}

func (b *OpenCodeBackend) buildCloneTask(repoURL, branch string, creds *GitCredentials) string {
	url := repoURL
	if creds != nil && creds.HasToken() {
		url = creds.InjectCredentials(repoURL)
	}

	if branch != "" {
		return fmt.Sprintf("Clone the git repository: git clone --branch %s %s . && git status", branch, url)
	}
	return fmt.Sprintf("Clone the git repository: git clone %s . && git status", url)
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
	return b.executeInternal(ctx, session, task)
}

func (b *OpenCodeBackend) executeInternal(ctx context.Context, session *Session, task Task) (*Result, error) {
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
		return nil, &Error{Op: "Execute", Session: session.ID, Err: err}
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

func (b *OpenCodeBackend) GitCommit(ctx context.Context, sessionID string, message string, addAll bool) (*GitCommitResult, error) {
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

	result, err := b.executeInternal(ctx, session, Task{Instruction: task})
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

func (b *OpenCodeBackend) GitPush(ctx context.Context, sessionID string, remote, branch string, setUpstream bool) (*GitPushResult, error) {
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

	result, err := b.executeInternal(ctx, session, Task{Instruction: task})
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

func parseGitOutputFromSummary(summary string) (hash string, files, insertions, deletions int) {
	lines := strings.Split(summary, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Commit hash:") || strings.HasPrefix(line, "commit hash:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				hash = strings.TrimSpace(parts[1])
			}
		}

		if len(line) == 40 && isHexString(line) {
			hash = line
		}

		if strings.Contains(line, "file") && strings.Contains(line, "changed") {
			files, insertions, deletions = parseGitCommitStats(line)
		}
	}
	return
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func (b *OpenCodeBackend) buildPrompt(task Task, workspacePath string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Working directory: %s\nCreate this directory if needed and perform all operations there.\n\n", workspacePath))

	if task.Context != "" {
		sb.WriteString("Context: ")
		sb.WriteString(task.Context)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Task: ")
	sb.WriteString(task.Instruction)

	if len(task.Files) > 0 {
		sb.WriteString(fmt.Sprintf("\n\nFocus on these files: %v", task.Files))
	}

	return sb.String()
}
