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

type ClaudeCodeBackend struct {
	binaryPath      string
	taskTimeout     time.Duration
	model           string
	maxTurns        int
	allowedTools    []string
	disallowedTools []string
	mu              sync.RWMutex
	sessions        map[string]*Session
	tracer          trace.Tracer
}

func NewClaudeCodeBackend(cfg config.CodingConfig) *ClaudeCodeBackend {
	binaryPath := cfg.ClaudeCode.BinaryPath
	if binaryPath == "" {
		binaryPath = "claude"
	}

	taskTimeout := time.Duration(cfg.ClaudeCode.TimeoutSec) * time.Second
	if taskTimeout == 0 {
		taskTimeout = 5 * time.Minute
	}

	maxTurns := cfg.ClaudeCode.MaxTurns
	if maxTurns == 0 {
		maxTurns = 10
	}

	return &ClaudeCodeBackend{
		binaryPath:      binaryPath,
		taskTimeout:     taskTimeout,
		model:           cfg.ClaudeCode.Model,
		maxTurns:        maxTurns,
		allowedTools:    cfg.ClaudeCode.AllowedTools,
		disallowedTools: cfg.ClaudeCode.DisallowedTools,
		sessions:        make(map[string]*Session),
		tracer:          otel.Tracer("station.coding.claudecode"),
	}
}

func (b *ClaudeCodeBackend) Ping(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, b.binaryPath, "--version")
	return cmd.Run()
}

func (b *ClaudeCodeBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	sessionID := fmt.Sprintf("claude_%d", time.Now().UnixNano())

	session := &Session{
		ID:               sessionID,
		BackendSessionID: opts.ExistingSessionID,
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

func (b *ClaudeCodeBackend) buildCloneTask(repoURL, branch string, creds *GitCredentials) string {
	url := repoURL
	if creds != nil && creds.HasToken() {
		url = creds.InjectCredentials(repoURL)
	}

	if branch != "" {
		return fmt.Sprintf("Clone the git repository: git clone --branch %s %s . && git status", branch, url)
	}
	return fmt.Sprintf("Clone the git repository: git clone %s . && git status", url)
}

func (b *ClaudeCodeBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	b.mu.RLock()
	session, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &Error{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	return session, nil
}

func (b *ClaudeCodeBackend) CloseSession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.sessions[sessionID]; !ok {
		return &Error{Op: "CloseSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	delete(b.sessions, sessionID)
	return nil
}

func (b *ClaudeCodeBackend) Execute(ctx context.Context, sessionID string, task Task) (*Result, error) {
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

	ctx, span := b.tracer.Start(ctx, "claudecode.task",
		trace.WithAttributes(
			attribute.String("claudecode.session_id", session.ID),
			attribute.String("claudecode.workspace", session.WorkspacePath),
		),
	)
	defer span.End()

	args := []string{"-p", task.Instruction, "--print", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"}

	if session.BackendSessionID != "" {
		args = append(args, "--resume", session.BackendSessionID)
	}

	if b.model != "" {
		args = append(args, "--model", b.model)
	}

	if b.maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", b.maxTurns))
	}

	if len(b.allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(b.allowedTools, ","))
	}

	if len(b.disallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(b.disallowedTools, ","))
	}

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
	var finalText strings.Builder
	var errorMsg string
	var backendSessionID string
	var tokens TokenUsage
	var cost float64

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var event claudeEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		if event.SessionID != "" && backendSessionID == "" {
			backendSessionID = event.SessionID
		}

		switch event.Type {
		case "assistant":
			var msg claudeMessage
			if err := json.Unmarshal(event.Message, &msg); err == nil {
				for _, block := range msg.Content {
					switch block.Type {
					case "text":
						finalText.WriteString(block.Text)
					case "tool_use":
						tc := ToolCall{
							Tool: block.Name,
						}
						if len(block.Input) > 0 {
							var inputMap map[string]interface{}
							if err := json.Unmarshal(block.Input, &inputMap); err == nil {
								tc.Input = inputMap
							}
						}
						toolCalls = append(toolCalls, tc)

						_, tcSpan := b.tracer.Start(ctx, "claudecode.tool."+block.Name,
							trace.WithAttributes(
								attribute.String("tool.name", block.Name),
								attribute.String("tool.id", block.ID),
							),
						)
						tcSpan.End()
					}
				}
				if msg.Usage != nil {
					tokens.Input += msg.Usage.InputTokens
					tokens.Output += msg.Usage.OutputTokens
					tokens.CacheRead += msg.Usage.CacheReadInputTokens
					tokens.CacheWrite += msg.Usage.CacheCreationInputTokens
				}
			}

		case "user":
			var msg claudeMessage
			if err := json.Unmarshal(event.Message, &msg); err == nil {
				for i, block := range msg.Content {
					if block.Type == "tool_result" {
						var toolResult claudeToolResult
						rawBlock, _ := json.Marshal(block)
						if err := json.Unmarshal(rawBlock, &toolResult); err == nil {
							for j := len(toolCalls) - 1; j >= 0; j-- {
								if toolCalls[j].Output == "" {
									toolCalls[j].Output = toolResult.Content
									if toolResult.IsError {
										toolCalls[j].Error = toolResult.Content
									}
									break
								}
							}
						}
					}
					_ = i
				}
			}

		case "result":
			var result claudeResult
			if err := json.Unmarshal(event.Result, &result); err == nil {
				if result.SessionID != "" {
					backendSessionID = result.SessionID
				}
				cost = result.TotalCostUSD
				if result.Usage != nil {
					tokens.Input = result.Usage.InputTokens
					tokens.Output = result.Usage.OutputTokens
					tokens.CacheRead = result.Usage.CacheReadInputTokens
					tokens.CacheWrite = result.Usage.CacheCreationInputTokens
				}
				if result.IsError {
					errorMsg = result.Result
				}
			}

		case "system":
			var sysMsg claudeSystemMessage
			if err := json.Unmarshal(scanner.Bytes(), &sysMsg); err == nil {
				if sysMsg.Level == "error" {
					errorMsg = sysMsg.Message
				}
			}
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
		attribute.String("claudecode.backend_session_id", backendSessionID),
		attribute.Int("claudecode.tool_calls", len(toolCalls)),
		attribute.Float64("claudecode.cost", cost),
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
		Summary: finalText.String(),
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

func (b *ClaudeCodeBackend) GitCommit(ctx context.Context, sessionID string, message string, addAll bool) (*GitCommitResult, error) {
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

func (b *ClaudeCodeBackend) GitPush(ctx context.Context, sessionID string, remote, branch string, setUpstream bool) (*GitPushResult, error) {
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

func (b *ClaudeCodeBackend) GitBranch(ctx context.Context, sessionID string, branch string, create bool) (*GitBranchResult, error) {
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
