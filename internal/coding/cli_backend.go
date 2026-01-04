package coding

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"station/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type CLIBackend struct {
	binaryPath  string
	taskTimeout time.Duration
	mu          sync.RWMutex
	sessions    map[string]*Session
	tracer      trace.Tracer
}

func NewCLIBackend(cfg config.CodingConfig) *CLIBackend {
	binaryPath := cfg.CLI.BinaryPath
	if binaryPath == "" {
		binaryPath = "opencode"
	}

	taskTimeout := time.Duration(cfg.CLI.TimeoutSec) * time.Second
	if taskTimeout == 0 {
		taskTimeout = 5 * time.Minute
	}

	return &CLIBackend{
		binaryPath:  binaryPath,
		taskTimeout: taskTimeout,
		sessions:    make(map[string]*Session),
		tracer:      otel.Tracer("station.coding.cli"),
	}
}

type cliEvent struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	SessionID string          `json:"sessionID"`
	Part      json.RawMessage `json:"part,omitempty"`
	Error     json.RawMessage `json:"error,omitempty"`
}

type cliTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Time *struct {
		End int64 `json:"end,omitempty"`
	} `json:"time,omitempty"`
}

type cliToolPart struct {
	Type  string `json:"type"`
	Tool  string `json:"tool"`
	State struct {
		Status string          `json:"status"`
		Input  json.RawMessage `json:"input"`
		Output string          `json:"output"`
		Title  string          `json:"title,omitempty"`
	} `json:"state"`
}

type cliStepFinishPart struct {
	Type   string  `json:"type"`
	Reason string  `json:"reason"`
	Cost   float64 `json:"cost"`
	Tokens struct {
		Input     int `json:"input"`
		Output    int `json:"output"`
		Reasoning int `json:"reasoning"`
		Cache     struct {
			Read  int `json:"read"`
			Write int `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
}

func (b *CLIBackend) Ping(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, b.binaryPath, "--version")
	return cmd.Run()
}

type openCodeSession struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (b *CLIBackend) resolveSessionID(ctx context.Context, nameOrID string) (string, error) {
	if strings.HasPrefix(nameOrID, "ses_") {
		return nameOrID, nil
	}

	cmd := exec.CommandContext(ctx, b.binaryPath, "session", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("list sessions: %w", err)
	}

	var sessions []openCodeSession
	if err := json.Unmarshal(output, &sessions); err != nil {
		return "", fmt.Errorf("parse sessions: %w", err)
	}

	nameOrIDLower := strings.ToLower(nameOrID)
	for _, s := range sessions {
		if strings.ToLower(s.Title) == nameOrIDLower {
			return s.ID, nil
		}
	}

	for _, s := range sessions {
		if strings.Contains(strings.ToLower(s.Title), nameOrIDLower) {
			return s.ID, nil
		}
	}

	return "", fmt.Errorf("session not found: %q (use 'opencode session list' to see available sessions)", nameOrID)
}

func (b *CLIBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	sessionID := fmt.Sprintf("cli_%d", time.Now().UnixNano())

	var backendSessionID string
	if opts.ExistingSessionID != "" {
		resolved, err := b.resolveSessionID(ctx, opts.ExistingSessionID)
		if err != nil {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("resolve session: %w", err)}
		}
		backendSessionID = resolved
	}

	session := &Session{
		ID:               sessionID,
		BackendSessionID: backendSessionID,
		WorkspacePath:    opts.WorkspacePath,
		Title:            opts.Title,
		CreatedAt:        time.Now(),
		LastUsedAt:       time.Now(),
		Metadata:         make(map[string]string),
	}

	if opts.WorkspacePath != "" {
		if err := os.MkdirAll(opts.WorkspacePath, 0755); err != nil {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("create workspace: %w", err)}
		}
	}

	if opts.RepoURL != "" {
		session.Metadata["repo_url"] = opts.RepoURL
		session.Metadata["branch"] = opts.Branch
	}

	b.mu.Lock()
	b.sessions[sessionID] = session
	b.mu.Unlock()

	if opts.RepoURL != "" {
		cloneTask := b.buildCloneTask(opts.RepoURL, opts.Branch, opts.GitCredentials)
		result, err := b.Execute(ctx, sessionID, Task{Instruction: cloneTask})
		if err != nil {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("clone repo: %w", err)}
		}
		if !result.Success {
			return nil, &Error{Op: "CreateSession", Err: fmt.Errorf("clone failed: %s", result.Error)}
		}
	}

	return session, nil
}

func (b *CLIBackend) buildCloneTask(repoURL, branch string, creds *GitCredentials) string {
	url := repoURL
	if creds != nil && creds.HasToken() {
		url = creds.InjectCredentials(repoURL)
	}

	if branch != "" {
		return fmt.Sprintf("Clone the git repository: git clone --branch %s %s . && git status", branch, url)
	}
	return fmt.Sprintf("Clone the git repository: git clone %s . && git status", url)
}

func (b *CLIBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	b.mu.RLock()
	session, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &Error{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	return session, nil
}

func (b *CLIBackend) CloseSession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.sessions[sessionID]; !ok {
		return &Error{Op: "CloseSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	delete(b.sessions, sessionID)
	return nil
}

func (b *CLIBackend) Execute(ctx context.Context, sessionID string, task Task) (*Result, error) {
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

	ctx, span := b.tracer.Start(ctx, "opencode.cli.task",
		trace.WithAttributes(
			attribute.String("opencode.session_id", session.ID),
			attribute.String("opencode.workspace", session.WorkspacePath),
		),
	)
	defer span.End()

	args := []string{"run", "--format", "json"}

	if session.BackendSessionID != "" {
		args = append(args, "--session", session.BackendSessionID)
	} else if session.Title != "" {
		args = append(args, "--title", session.Title)
	}

	args = append(args, task.Instruction)

	cmd := exec.CommandContext(ctx, b.binaryPath, args...)

	if session.WorkspacePath != "" {
		cmd.Dir = session.WorkspacePath
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		span.RecordError(err)
		return nil, &Error{Op: "Execute", Session: sessionID, Err: fmt.Errorf("stdout pipe: %w", err)}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		span.RecordError(err)
		return nil, &Error{Op: "Execute", Session: sessionID, Err: fmt.Errorf("stderr pipe: %w", err)}
	}

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		span.RecordError(err)
		return nil, &Error{Op: "Execute", Session: sessionID, Err: fmt.Errorf("start command: %w", err)}
	}

	var toolCalls []ToolCall
	var finalText string
	var errorMsg string
	var backendSessionID string
	var tokens TokenUsage
	var cost float64

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var event cliEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		if event.SessionID != "" && backendSessionID == "" {
			backendSessionID = event.SessionID
		}

		switch event.Type {
		case "tool_use":
			var part cliToolPart
			if err := json.Unmarshal(event.Part, &part); err == nil {
				tc := ToolCall{
					Tool:   part.Tool,
					Output: part.State.Output,
				}
				if len(part.State.Input) > 0 {
					var inputMap map[string]interface{}
					if err := json.Unmarshal(part.State.Input, &inputMap); err == nil {
						tc.Input = inputMap
					}
				}
				toolCalls = append(toolCalls, tc)

				_, tcSpan := b.tracer.Start(ctx, "opencode.tool."+part.Tool,
					trace.WithAttributes(
						attribute.String("tool.name", part.Tool),
					),
				)
				if len(part.State.Output) > 500 {
					tcSpan.SetAttributes(attribute.String("tool.output_preview", part.State.Output[:500]+"..."))
				} else if part.State.Output != "" {
					tcSpan.SetAttributes(attribute.String("tool.output", part.State.Output))
				}
				tcSpan.End()
			}

		case "text":
			var part cliTextPart
			if err := json.Unmarshal(event.Part, &part); err == nil {
				if part.Time != nil && part.Time.End != 0 {
					finalText = part.Text
				}
			}

		case "step_finish":
			var part cliStepFinishPart
			if err := json.Unmarshal(event.Part, &part); err == nil {
				tokens = TokenUsage{
					Input:      part.Tokens.Input,
					Output:     part.Tokens.Output,
					Reasoning:  part.Tokens.Reasoning,
					CacheRead:  part.Tokens.Cache.Read,
					CacheWrite: part.Tokens.Cache.Write,
				}
				cost = part.Cost
			}

		case "error":
			errorMsg = string(event.Error)
		}
	}

	stderrBytes, _ := io.ReadAll(stderr)

	cmdErr := cmd.Wait()
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	if backendSessionID != "" && session.BackendSessionID == "" {
		b.mu.Lock()
		session.BackendSessionID = backendSessionID
		b.sessions[sessionID] = session
		b.mu.Unlock()
	}

	span.SetAttributes(
		attribute.String("opencode.backend_session_id", backendSessionID),
		attribute.Int("opencode.tool_calls", len(toolCalls)),
		attribute.Float64("opencode.cost", cost),
	)

	if cmdErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			span.SetStatus(codes.Error, "timeout")
			return &Result{
				Success: false,
				Error:   "task timed out",
				Trace: &Trace{
					SessionID: backendSessionID,
					StartTime: startTime,
					EndTime:   endTime,
					Duration:  duration,
					ToolCalls: toolCalls,
				},
			}, nil
		}

		if errorMsg == "" {
			errorMsg = strings.TrimSpace(string(stderrBytes))
		}
		if errorMsg == "" {
			errorMsg = cmdErr.Error()
		}

		span.RecordError(cmdErr)
		span.SetStatus(codes.Error, errorMsg)

		return &Result{
			Success: false,
			Error:   errorMsg,
			Trace: &Trace{
				SessionID: backendSessionID,
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
		Summary: finalText,
		Trace: &Trace{
			SessionID: backendSessionID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
			ToolCalls: toolCalls,
			Tokens:    tokens,
			Cost:      cost,
		},
	}, nil
}

func (b *CLIBackend) GitCommit(ctx context.Context, sessionID string, message string, addAll bool) (*GitCommitResult, error) {
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

	result, err := b.Execute(ctx, sessionID, Task{Instruction: task})
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

func (b *CLIBackend) GitPush(ctx context.Context, sessionID string, remote, branch string, setUpstream bool) (*GitPushResult, error) {
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

	result, err := b.Execute(ctx, sessionID, Task{Instruction: task})
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

func (b *CLIBackend) GitBranch(ctx context.Context, sessionID string, branch string, create bool) (*GitBranchResult, error) {
	var task string
	if create {
		task = fmt.Sprintf(`Run git commands to create and switch to a new branch:
1. git branch (to get current branch)
2. git checkout -b %s
3. Report: new branch name, previous branch name`, branch)
	} else {
		task = fmt.Sprintf(`Run git commands to switch to an existing branch:
1. git branch (to get current branch)
2. git checkout %s
3. Report: branch name, previous branch name`, branch)
	}

	result, err := b.Execute(ctx, sessionID, Task{Instruction: task})
	if err != nil {
		return nil, err
	}

	branchResult := &GitBranchResult{
		Success:    result.Success,
		Branch:     branch,
		Created:    create,
		SwitchedTo: result.Success,
	}

	if !result.Success {
		branchResult.Error = result.Error
		if branchResult.Error == "" {
			branchResult.Error = result.Summary
		}
	}

	return branchResult, nil
}
