package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OpenCodeBackend implements SandboxBackend using an external OpenCode container.
// OpenCode is an AI-powered coding assistant that runs in a container and exposes
// an HTTP API for task execution. This backend translates sandbox operations into
// OpenCode task requests.
type OpenCodeBackend struct {
	config   OpenCodeConfig
	client   *http.Client
	mu       sync.RWMutex
	sessions map[string]*openCodeSession
	execs    map[string]*openCodeExecState
}

// openCodeSession tracks a virtual session mapped to OpenCode
type openCodeSession struct {
	ID            string
	OpenCodeID    string // OpenCode's session ID
	WorkspacePath string // Local workspace path (for file operations)
	Workdir       string // Working directory inside OpenCode
	Env           map[string]string
	Limits        ResourceLimits
	CreatedAt     time.Time
	LastUsedAt    time.Time
}

// openCodeExecState tracks async execution state
type openCodeExecState struct {
	id        string
	sessionID string
	cmd       []string
	startedAt time.Time
	done      chan struct{}
	result    *ExecResult
	chunks    []OutputChunk
	chunkMu   sync.Mutex
	nextSeq   int
}

type OpenCodeConfig struct {
	Enabled                bool
	ServerURL              string
	DefaultTimeout         time.Duration
	MaxStdoutBytes         int
	WorkspaceHostPath      string
	WorkspaceContainerPath string
	Model                  string
}

type OpenCodeTrace struct {
	MessageID    string             `json:"message_id"`
	SessionID    string             `json:"session_id"`
	Model        string             `json:"model"`
	Provider     string             `json:"provider"`
	Cost         float64            `json:"cost"`
	Tokens       OpenCodeTokens     `json:"tokens"`
	StartTime    time.Time          `json:"start_time"`
	EndTime      time.Time          `json:"end_time"`
	Duration     time.Duration      `json:"duration"`
	ToolCalls    []OpenCodeToolCall `json:"tool_calls"`
	Reasoning    []string           `json:"reasoning,omitempty"`
	FinalText    string             `json:"final_text"`
	FinishReason string             `json:"finish_reason"`
}

type OpenCodeTokens struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	Reasoning  int `json:"reasoning"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

type OpenCodeToolCall struct {
	Tool     string                 `json:"tool"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Output   string                 `json:"output,omitempty"`
	ExitCode int                    `json:"exit_code,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

func DefaultOpenCodeConfig() OpenCodeConfig {
	return OpenCodeConfig{
		Enabled:                false,
		ServerURL:              "http://localhost:4096",
		DefaultTimeout:         5 * time.Minute,
		MaxStdoutBytes:         1024 * 1024,
		WorkspaceHostPath:      "/tmp/station-opencode-workspaces",
		WorkspaceContainerPath: "/workspaces",
		Model:                  "claude-sonnet-4-20250514",
	}
}

func NewOpenCodeBackend(cfg OpenCodeConfig) (*OpenCodeBackend, error) {
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:4096"
	}

	if cfg.WorkspaceHostPath == "" {
		cfg.WorkspaceHostPath = "/tmp/station-opencode-workspaces"
	}

	if cfg.WorkspaceContainerPath == "" {
		cfg.WorkspaceContainerPath = "/workspaces"
	}

	if err := os.MkdirAll(cfg.WorkspaceHostPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace base dir: %w", err)
	}

	client := &http.Client{
		Timeout: cfg.DefaultTimeout,
	}

	return &OpenCodeBackend{
		config:   cfg,
		client:   client,
		sessions: make(map[string]*openCodeSession),
		execs:    make(map[string]*openCodeExecState),
	}, nil
}

func (b *OpenCodeBackend) hostToContainerPath(hostPath string) string {
	rel, err := filepath.Rel(b.config.WorkspaceHostPath, hostPath)
	if err != nil {
		return hostPath
	}
	return filepath.Join(b.config.WorkspaceContainerPath, rel)
}

// Ping checks if OpenCode server is available
func (b *OpenCodeBackend) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", b.config.ServerURL+"/global/health", nil)
	if err != nil {
		return &SandboxError{Op: "Ping", Err: fmt.Errorf("create request: %w", err)}
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return &SandboxError{Op: "Ping", Err: fmt.Errorf("OpenCode server not available at %s: %w", b.config.ServerURL, err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &SandboxError{Op: "Ping", Err: fmt.Errorf("OpenCode server unhealthy: status %d", resp.StatusCode)}
	}

	return nil
}

func (b *OpenCodeBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	sessionID := fmt.Sprintf("oc_%s", generateShortID())

	workspacePath := filepath.Join(b.config.WorkspaceHostPath, sessionID)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("create workspace: %w", err)}
	}

	containerPath := b.hostToContainerPath(workspacePath)
	openCodeSessionID, err := b.createOpenCodeSession(ctx, containerPath)
	if err != nil {
		os.RemoveAll(workspacePath)
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("create OpenCode session: %w", err)}
	}

	workdir := opts.Workdir
	if workdir == "" {
		workdir = containerPath
	}

	ocSession := &openCodeSession{
		ID:            sessionID,
		OpenCodeID:    openCodeSessionID,
		WorkspacePath: workspacePath,
		Workdir:       workdir,
		Env:           opts.Env,
		Limits:        opts.Limits,
		CreatedAt:     time.Now(),
		LastUsedAt:    time.Now(),
	}

	b.mu.Lock()
	b.sessions[sessionID] = ocSession
	b.mu.Unlock()

	return &Session{
		ID:            sessionID,
		ContainerID:   openCodeSessionID, // Store OpenCode session ID here
		Image:         "opencode",
		Workdir:       workdir,
		WorkspacePath: workspacePath,
		Env:           opts.Env,
		Limits:        opts.Limits,
		CreatedAt:     ocSession.CreatedAt,
		LastUsedAt:    ocSession.LastUsedAt,
	}, nil
}

// GetSession retrieves a session by ID
func (b *OpenCodeBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	b.mu.RLock()
	ocSession, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	return &Session{
		ID:            ocSession.ID,
		ContainerID:   ocSession.OpenCodeID,
		Image:         "opencode",
		Workdir:       ocSession.Workdir,
		WorkspacePath: ocSession.WorkspacePath,
		Env:           ocSession.Env,
		Limits:        ocSession.Limits,
		CreatedAt:     ocSession.CreatedAt,
		LastUsedAt:    ocSession.LastUsedAt,
	}, nil
}

// DestroySession destroys a session and cleans up resources
func (b *OpenCodeBackend) DestroySession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	ocSession, ok := b.sessions[sessionID]
	if ok {
		delete(b.sessions, sessionID)
	}
	b.mu.Unlock()

	if !ok {
		return &SandboxError{Op: "DestroySession", Session: sessionID, Err: ErrSessionNotFound}
	}

	// Clean up local workspace
	if ocSession.WorkspacePath != "" {
		os.RemoveAll(ocSession.WorkspacePath)
	}

	// Note: OpenCode sessions are managed by OpenCode server
	// We don't explicitly destroy them via API

	return nil
}

// Exec executes a command in the sandbox via OpenCode
func (b *OpenCodeBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error) {
	ocSession, err := b.getOpenCodeSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Update last used time
	b.mu.Lock()
	ocSession.LastUsedAt = time.Now()
	b.mu.Unlock()

	// Build the command string
	cmdStr := strings.Join(req.Cmd, " ")

	// Build environment prefix if needed
	envPrefix := ""
	for k, v := range req.Env {
		envPrefix += fmt.Sprintf("%s=%q ", k, v)
	}

	// Build the task for OpenCode
	// We ask OpenCode to execute the exact command and return the output
	task := fmt.Sprintf(`Execute this exact bash command and return ONLY the raw output (no explanation, no markdown formatting):

cd %s && %s%s

Return the command output exactly as produced. If there's an error, return the error message.`,
		req.Cwd, envPrefix, cmdStr)

	if req.Cwd == "" {
		task = fmt.Sprintf(`Execute this exact bash command and return ONLY the raw output (no explanation, no markdown formatting):

%s%s

Return the command output exactly as produced. If there's an error, return the error message.`,
			envPrefix, cmdStr)
	}

	start := time.Now()

	// Send task to OpenCode
	response, err := b.sendTask(ctx, ocSession.OpenCodeID, task)
	duration := time.Since(start)

	if err != nil {
		return &ExecResult{
			ID:       generateShortID(),
			ExitCode: -1,
			Stderr:   fmt.Sprintf("OpenCode execution error: %v", err),
			Duration: duration,
		}, nil
	}

	// Parse the response - OpenCode returns the command output
	stdout, truncated := b.truncateOutput([]byte(response))

	return &ExecResult{
		ID:        generateShortID(),
		ExitCode:  0, // Assume success if we got a response
		Stdout:    stdout,
		Duration:  duration,
		Truncated: truncated,
	}, nil
}

// ExecAsync starts an async execution
func (b *OpenCodeBackend) ExecAsync(ctx context.Context, sessionID string, req ExecRequest) (*ExecHandle, error) {
	ocSession, err := b.getOpenCodeSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	execID := generateShortID()
	state := &openCodeExecState{
		id:        execID,
		sessionID: sessionID,
		cmd:       req.Cmd,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}

	b.mu.Lock()
	b.execs[execID] = state
	b.mu.Unlock()

	// Run execution in background
	go func() {
		defer close(state.done)
		result, _ := b.Exec(context.Background(), sessionID, req)
		if result != nil {
			state.result = result
			state.result.ID = execID
		} else {
			state.result = &ExecResult{
				ID:       execID,
				ExitCode: -1,
				Stderr:   "execution failed",
				Duration: time.Since(state.startedAt),
			}
		}
	}()

	_ = ocSession

	return &ExecHandle{
		ID:        execID,
		SessionID: sessionID,
		Cmd:       req.Cmd,
		StartedAt: state.startedAt,
	}, nil
}

// ExecWait waits for an async execution to complete
func (b *OpenCodeBackend) ExecWait(ctx context.Context, sessionID, execID string, timeout time.Duration) (*ExecResult, error) {
	b.mu.RLock()
	state, ok := b.execs[execID]
	b.mu.RUnlock()

	if !ok {
		return nil, &SandboxError{Op: "ExecWait", Session: sessionID, Err: ErrExecNotFound}
	}

	if state.sessionID != sessionID {
		return nil, &SandboxError{Op: "ExecWait", Session: sessionID, Err: ErrExecNotFound}
	}

	select {
	case <-state.done:
		return state.result, nil
	case <-ctx.Done():
		return nil, &SandboxError{Op: "ExecWait", Session: sessionID, Err: ctx.Err()}
	case <-time.After(timeout):
		return nil, &SandboxError{Op: "ExecWait", Session: sessionID, Err: ErrTimeout}
	}
}

// ExecRead reads output chunks from an async execution
func (b *OpenCodeBackend) ExecRead(ctx context.Context, sessionID, execID string, sinceSeq int, maxChunks int) (*ExecChunks, error) {
	b.mu.RLock()
	state, ok := b.execs[execID]
	b.mu.RUnlock()

	if !ok {
		return nil, &SandboxError{Op: "ExecRead", Session: sessionID, Err: ErrExecNotFound}
	}

	if state.sessionID != sessionID {
		return nil, &SandboxError{Op: "ExecRead", Session: sessionID, Err: ErrExecNotFound}
	}

	state.chunkMu.Lock()
	defer state.chunkMu.Unlock()

	var chunks []OutputChunk
	for _, chunk := range state.chunks {
		if chunk.Seq > sinceSeq {
			chunks = append(chunks, chunk)
			if maxChunks > 0 && len(chunks) >= maxChunks {
				break
			}
		}
	}

	done := false
	select {
	case <-state.done:
		done = true
	default:
	}

	return &ExecChunks{
		Chunks: chunks,
		Done:   done,
	}, nil
}

// WriteFile writes a file to the sandbox workspace
func (b *OpenCodeBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error {
	ocSession, err := b.getOpenCodeSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Normalize path
	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(ocSession.WorkspacePath, path)

	// Create directory if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &SandboxError{Op: "WriteFile", Session: sessionID, Err: fmt.Errorf("create directory: %w", err)}
	}

	if mode == 0 {
		mode = 0644
	}

	if err := os.WriteFile(fullPath, content, mode); err != nil {
		return &SandboxError{Op: "WriteFile", Session: sessionID, Err: fmt.Errorf("write file: %w", err)}
	}

	return nil
}

// ReadFile reads a file from the sandbox workspace
func (b *OpenCodeBackend) ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, bool, error) {
	ocSession, err := b.getOpenCodeSession(ctx, sessionID)
	if err != nil {
		return nil, false, err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(ocSession.WorkspacePath, path)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, false, &SandboxError{Op: "ReadFile", Session: sessionID, Err: fmt.Errorf("stat file: %w", err)}
	}

	if info.IsDir() {
		return nil, false, &SandboxError{Op: "ReadFile", Session: sessionID, Err: fmt.Errorf("path is a directory")}
	}

	if maxBytes <= 0 {
		maxBytes = b.config.MaxStdoutBytes
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, false, &SandboxError{Op: "ReadFile", Session: sessionID, Err: fmt.Errorf("open file: %w", err)}
	}
	defer f.Close()

	truncated := info.Size() > int64(maxBytes)
	readSize := maxBytes
	if int64(readSize) > info.Size() {
		readSize = int(info.Size())
	}

	content := make([]byte, readSize)
	_, err = io.ReadFull(f, content)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, false, &SandboxError{Op: "ReadFile", Session: sessionID, Err: fmt.Errorf("read file: %w", err)}
	}

	return content, truncated, nil
}

// ListFiles lists files in the sandbox workspace
func (b *OpenCodeBackend) ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error) {
	ocSession, err := b.getOpenCodeSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	path = normalizeWorkspacePath(path)
	basePath := filepath.Join(ocSession.WorkspacePath, path)

	var entries []FileEntry

	if recursive {
		err = filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			relPath, _ := filepath.Rel(ocSession.WorkspacePath, p)
			if relPath == "." {
				return nil
			}

			entryType := "file"
			if info.IsDir() {
				entryType = "dir"
			}

			entries = append(entries, FileEntry{
				Path:      relPath,
				Type:      entryType,
				Size:      info.Size(),
				Mode:      fmt.Sprintf("%04o", info.Mode().Perm()),
				MtimeUnix: info.ModTime().Unix(),
			})
			return nil
		})
	} else {
		dirEntries, readErr := os.ReadDir(basePath)
		if readErr != nil {
			return nil, &SandboxError{Op: "ListFiles", Session: sessionID, Err: fmt.Errorf("read directory: %w", readErr)}
		}

		for _, de := range dirEntries {
			info, infoErr := de.Info()
			if infoErr != nil {
				continue
			}

			entryType := "file"
			if de.IsDir() {
				entryType = "dir"
			}

			relPath := filepath.Join(path, de.Name())
			entries = append(entries, FileEntry{
				Path:      relPath,
				Type:      entryType,
				Size:      info.Size(),
				Mode:      fmt.Sprintf("%04o", info.Mode().Perm()),
				MtimeUnix: info.ModTime().Unix(),
			})
		}
	}

	if err != nil {
		return nil, &SandboxError{Op: "ListFiles", Session: sessionID, Err: fmt.Errorf("walk directory: %w", err)}
	}

	return entries, nil
}

// DeleteFile deletes a file from the sandbox workspace
func (b *OpenCodeBackend) DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error {
	ocSession, err := b.getOpenCodeSession(ctx, sessionID)
	if err != nil {
		return err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(ocSession.WorkspacePath, path)

	if fullPath == ocSession.WorkspacePath {
		return &SandboxError{Op: "DeleteFile", Session: sessionID, Err: fmt.Errorf("cannot delete workspace root")}
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return &SandboxError{Op: "DeleteFile", Session: sessionID, Err: fmt.Errorf("stat path: %w", err)}
	}

	if info.IsDir() && !recursive {
		return &SandboxError{Op: "DeleteFile", Session: sessionID, Err: fmt.Errorf("path is a directory, use recursive=true")}
	}

	if recursive {
		err = os.RemoveAll(fullPath)
	} else {
		err = os.Remove(fullPath)
	}

	if err != nil {
		return &SandboxError{Op: "DeleteFile", Session: sessionID, Err: fmt.Errorf("delete: %w", err)}
	}

	return nil
}

// Close cleans up the backend resources
func (b *OpenCodeBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clean up all sessions
	for _, session := range b.sessions {
		if session.WorkspacePath != "" {
			os.RemoveAll(session.WorkspacePath)
		}
	}
	b.sessions = make(map[string]*openCodeSession)

	return nil
}

// =============================================================================
// Internal helper methods
// =============================================================================

// getOpenCodeSession retrieves the internal OpenCode session
func (b *OpenCodeBackend) getOpenCodeSession(ctx context.Context, sessionID string) (*openCodeSession, error) {
	b.mu.RLock()
	ocSession, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	return ocSession, nil
}

func (b *OpenCodeBackend) createOpenCodeSession(ctx context.Context, directory string) (string, error) {
	reqBody := map[string]interface{}{
		"title": "station-sandbox",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/session?directory=%s", b.config.ServerURL, directory)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create session failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if sessionID, ok := result["id"].(string); ok {
		return sessionID, nil
	}

	return generateShortID(), nil
}

// sendTask sends a task to OpenCode and waits for the response
func (b *OpenCodeBackend) sendTask(ctx context.Context, sessionID string, task string) (string, error) {
	reqBody := map[string]interface{}{
		"parts": []map[string]interface{}{
			{
				"type": "text",
				"text": task,
			},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/session/%s/message", b.config.ServerURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("send task failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return b.parseOpenCodeResponse(body)
}

func (b *OpenCodeBackend) parseOpenCodeResponse(body []byte) (string, error) {
	var result struct {
		Info struct {
			ID       string `json:"id"`
			Model    string `json:"modelID"`
			Provider string `json:"providerID"`
			Finish   string `json:"finish"`
		} `json:"info"`
		Parts []struct {
			Type   string `json:"type"`
			Text   string `json:"text,omitempty"`
			Tool   string `json:"tool,omitempty"`
			Output string `json:"output,omitempty"`
		} `json:"parts"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	var textParts []string
	for _, part := range result.Parts {
		switch part.Type {
		case "text":
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		case "tool-result":
			if part.Output != "" {
				textParts = append(textParts, part.Output)
			}
		}
	}

	if len(textParts) > 0 {
		return strings.Join(textParts, "\n"), nil
	}

	return string(body), nil
}

// truncateOutput truncates output to max bytes
func (b *OpenCodeBackend) truncateOutput(data []byte) (string, bool) {
	if len(data) <= b.config.MaxStdoutBytes {
		return string(data), false
	}
	return string(data[:b.config.MaxStdoutBytes]) + "\n... [truncated]", true
}
