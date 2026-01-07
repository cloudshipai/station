package services

import (
	"bytes"
	"context"
	"encoding/base64"
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

// FlyMachinesBackend implements SandboxBackend using Fly.io Machines API.
// This backend spawns ephemeral Fly Machines for sandbox execution, which is
// ideal for Station deployments on Fly.io where Docker-in-Docker is not available.
//
// Each sandbox session creates a new Fly Machine that runs until destroyed.
// The machine uses the specified image and exposes exec via the Machines API.
type FlyMachinesBackend struct {
	config   FlyMachinesConfig
	client   *http.Client
	mu       sync.RWMutex
	sessions map[string]*flyMachineSession
	execs    map[string]*flyExecState
}

// flyMachineSession tracks a sandbox session backed by a Fly Machine
type flyMachineSession struct {
	ID         string
	MachineID  string            // Fly Machine ID
	AppName    string            // Fly app name (for API calls)
	Image      string            // Container image
	Workdir    string            // Working directory
	Env        map[string]string // Environment variables
	Limits     ResourceLimits    // Resource limits
	CreatedAt  time.Time
	LastUsedAt time.Time
	PrivateIP  string // Machine's private IP for exec
}

// flyExecState tracks async execution state
type flyExecState struct {
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

// FlyMachinesConfig configures the Fly Machines backend
type FlyMachinesConfig struct {
	Enabled        bool            // Whether Fly Machines backend is enabled
	APIToken       string          // Fly.io API token (FLY_API_TOKEN)
	OrgSlug        string          // Fly.io organization slug
	AppPrefix      string          // Prefix for sandbox app names (e.g., "stn-sandbox")
	Region         string          // Primary region for machines (e.g., "ord")
	DefaultImage   string          // Default image for sandboxes
	DefaultTimeout time.Duration   // Default execution timeout
	MaxStdoutBytes int             // Maximum stdout bytes to capture
	MachineSize    string          // Machine size preset (e.g., "shared-cpu-1x")
	MemoryMB       int             // Memory in MB (default: 256)
	CPUKind        string          // CPU kind: "shared" or "performance"
	CPUs           int             // Number of CPUs (default: 1)
	RegistryAuth   FlyRegistryAuth // Private registry authentication
}

// FlyRegistryAuth holds credentials for pulling from private registries
type FlyRegistryAuth struct {
	Username      string // Registry username
	Password      string // Registry password or access token
	ServerAddress string // Registry server URL (e.g., "ghcr.io")
}

func DefaultFlyMachinesConfig() FlyMachinesConfig {
	apiToken := os.Getenv("FLY_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("FLY_API_KEY")
	}
	return FlyMachinesConfig{
		Enabled:        false,
		APIToken:       apiToken,
		OrgSlug:        os.Getenv("FLY_ORG"),
		AppPrefix:      "stn-sandbox",
		Region:         "ord",
		DefaultImage:   "python:3.11-slim",
		DefaultTimeout: 2 * time.Minute,
		MaxStdoutBytes: 1024 * 1024,
		MachineSize:    "shared-cpu-1x",
		MemoryMB:       256,
		CPUKind:        "shared",
		CPUs:           1,
	}
}

// Fly Machines API types

type flyMachineCreateRequest struct {
	Name   string           `json:"name,omitempty"`
	Region string           `json:"region"`
	Config flyMachineConfig `json:"config"`
}

type flyMachineConfig struct {
	Image    string            `json:"image"`
	ImageRef *flyImageRef      `json:"image_ref,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Guest    flyGuestConfig    `json:"guest,omitempty"`
	Init     flyInitConfig     `json:"init,omitempty"`
	Restart  flyRestartConfig  `json:"restart,omitempty"`
	AutoStop string            `json:"auto_stop,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type flyImageRef struct {
	Registry    string               `json:"registry,omitempty"`
	Repository  string               `json:"repository,omitempty"`
	Tag         string               `json:"tag,omitempty"`
	Credentials *flyImageCredentials `json:"credentials,omitempty"`
}

type flyImageCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type flyGuestConfig struct {
	CPUKind  string `json:"cpu_kind,omitempty"`
	CPUs     int    `json:"cpus,omitempty"`
	MemoryMB int    `json:"memory_mb,omitempty"`
}

type flyInitConfig struct {
	Cmd        []string `json:"cmd,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty"`
}

type flyRestartConfig struct {
	Policy string `json:"policy,omitempty"`
}

type flyMachineResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	State      string `json:"state"`
	Region     string `json:"region"`
	PrivateIP  string `json:"private_ip"`
	InstanceID string `json:"instance_id"`
}

type flyExecRequest struct {
	Cmd     string `json:"cmd"`
	Timeout int    `json:"timeout,omitempty"`
}

type flyExecResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// NewFlyMachinesBackend creates a new Fly Machines backend
func NewFlyMachinesBackend(cfg FlyMachinesConfig) (*FlyMachinesBackend, error) {
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("FLY_API_TOKEN is required for Fly Machines backend")
	}

	if cfg.OrgSlug == "" {
		return nil, fmt.Errorf("FLY_ORG is required for Fly Machines backend")
	}

	client := &http.Client{
		Timeout: cfg.DefaultTimeout + 30*time.Second, // Extra buffer for API calls
	}

	return &FlyMachinesBackend{
		config:   cfg,
		client:   client,
		sessions: make(map[string]*flyMachineSession),
		execs:    make(map[string]*flyExecState),
	}, nil
}

// Ping checks if the Fly Machines API is accessible
func (b *FlyMachinesBackend) Ping(ctx context.Context) error {
	// Check if we can list apps (simple health check)
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.machines.dev/v1/apps?org_slug=%s", b.config.OrgSlug),
		nil)
	if err != nil {
		return &SandboxError{Op: "Ping", Err: fmt.Errorf("create request: %w", err)}
	}

	req.Header.Set("Authorization", "Bearer "+b.config.APIToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return &SandboxError{Op: "Ping", Err: fmt.Errorf("Fly Machines API not available: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &SandboxError{Op: "Ping", Err: fmt.Errorf("Fly API error: status %d, body: %s", resp.StatusCode, string(body))}
	}

	return nil
}

// CreateSession creates a new sandbox session by spawning a Fly Machine
func (b *FlyMachinesBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	sessionID := fmt.Sprintf("fly_%s", generateShortID())

	// Build the app name - we use a shared app for all sandboxes
	appName := b.config.AppPrefix

	// Ensure the app exists (create if needed)
	if err := b.ensureApp(ctx, appName); err != nil {
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("ensure app: %w", err)}
	}

	// Determine image
	image := opts.Image
	if image == "" {
		image = b.config.DefaultImage
	}

	// Build environment variables
	env := make(map[string]string)
	for k, v := range opts.Env {
		env[k] = v
	}

	// Determine workdir
	workdir := opts.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}

	// Create the Fly Machine
	machineReq := flyMachineCreateRequest{
		Name:   sessionID,
		Region: b.config.Region,
		Config: flyMachineConfig{
			Image: image,
			Env:   env,
			Guest: flyGuestConfig{
				CPUKind:  b.config.CPUKind,
				CPUs:     b.config.CPUs,
				MemoryMB: b.config.MemoryMB,
			},
			Init: flyInitConfig{
				// Keep the container running with sleep infinity
				Cmd: []string{"sleep", "infinity"},
			},
			Restart: flyRestartConfig{
				Policy: "no", // Don't restart on exit
			},
			AutoStop: "off",
			Metadata: map[string]string{
				"station_session": sessionID,
				"created_at":      time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	// Apply resource limits
	if opts.Limits.MemoryMB > 0 {
		machineReq.Config.Guest.MemoryMB = opts.Limits.MemoryMB
	}
	if opts.Limits.CPUMillicores >= 1000 {
		machineReq.Config.Guest.CPUs = opts.Limits.CPUMillicores / 1000
	}

	// Add private registry credentials if configured
	if b.config.RegistryAuth.Username != "" && b.config.RegistryAuth.Password != "" {
		machineReq.Config.ImageRef = b.buildImageRef(image)
	}

	machineResp, err := b.createMachine(ctx, appName, machineReq)
	if err != nil {
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("create machine: %w", err)}
	}

	// Wait for machine to be running
	if err := b.waitForMachineState(ctx, appName, machineResp.ID, "started", 60*time.Second); err != nil {
		// Clean up the machine if it failed to start
		_ = b.destroyMachine(ctx, appName, machineResp.ID)
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("wait for machine: %w", err)}
	}

	if _, err := b.execOnMachine(ctx, appName, machineResp.ID, shellCmd("mkdir", "-p", workdir), 30); err != nil {
	}

	flySession := &flyMachineSession{
		ID:         sessionID,
		MachineID:  machineResp.ID,
		AppName:    appName,
		Image:      image,
		Workdir:    workdir,
		Env:        opts.Env,
		Limits:     opts.Limits,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		PrivateIP:  machineResp.PrivateIP,
	}

	b.mu.Lock()
	b.sessions[sessionID] = flySession
	b.mu.Unlock()

	return &Session{
		ID:          sessionID,
		ContainerID: machineResp.ID, // Store Fly Machine ID
		Image:       image,
		Workdir:     workdir,
		Env:         opts.Env,
		Limits:      opts.Limits,
		CreatedAt:   flySession.CreatedAt,
		LastUsedAt:  flySession.LastUsedAt,
	}, nil
}

// GetSession retrieves a session by ID
func (b *FlyMachinesBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	b.mu.RLock()
	flySession, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	// Verify machine is still running
	machine, err := b.getMachine(ctx, flySession.AppName, flySession.MachineID)
	if err != nil {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: fmt.Errorf("get machine: %w", err)}
	}

	if machine.State != "started" {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: ErrSessionClosed}
	}

	return &Session{
		ID:          flySession.ID,
		ContainerID: flySession.MachineID,
		Image:       flySession.Image,
		Workdir:     flySession.Workdir,
		Env:         flySession.Env,
		Limits:      flySession.Limits,
		CreatedAt:   flySession.CreatedAt,
		LastUsedAt:  flySession.LastUsedAt,
	}, nil
}

// DestroySession destroys a session and its Fly Machine
func (b *FlyMachinesBackend) DestroySession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	flySession, ok := b.sessions[sessionID]
	if ok {
		delete(b.sessions, sessionID)
	}
	b.mu.Unlock()

	if !ok {
		return &SandboxError{Op: "DestroySession", Session: sessionID, Err: ErrSessionNotFound}
	}

	// Destroy the Fly Machine
	if err := b.destroyMachine(ctx, flySession.AppName, flySession.MachineID); err != nil {
		// Log but don't fail - the machine might already be gone
		// In production, we'd log this
	}

	return nil
}

// Exec executes a command synchronously
func (b *FlyMachinesBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error) {
	flySession, err := b.getFlySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	flySession.LastUsedAt = time.Now()
	b.mu.Unlock()

	// Build the command
	cmd := req.Cmd
	cwd := req.Cwd
	if cwd == "" {
		cwd = flySession.Workdir
	}

	// Wrap command with cd and env
	shellCmd := b.buildShellCommand(cmd, cwd, req.Env)

	// Determine timeout
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = int(b.config.DefaultTimeout.Seconds())
	}

	start := time.Now()
	execResult, err := b.execOnMachine(ctx, flySession.AppName, flySession.MachineID, shellCmd, timeout)
	duration := time.Since(start)

	if err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return &ExecResult{
				ID:        generateShortID(),
				ExitCode:  -1,
				Stderr:    fmt.Sprintf("execution cancelled: %v", ctx.Err()),
				Duration:  duration,
				Cancelled: true,
			}, nil
		}
		return &ExecResult{
			ID:       generateShortID(),
			ExitCode: -1,
			Stderr:   fmt.Sprintf("exec failed: %v", err),
			Duration: duration,
		}, nil
	}

	stdoutStr, stdoutTrunc := b.truncateOutput([]byte(execResult.Stdout))
	stderrStr, stderrTrunc := b.truncateOutput([]byte(execResult.Stderr))

	return &ExecResult{
		ID:        generateShortID(),
		ExitCode:  execResult.ExitCode,
		Stdout:    stdoutStr,
		Stderr:    stderrStr,
		Duration:  duration,
		Truncated: stdoutTrunc || stderrTrunc,
	}, nil
}

// ExecAsync starts an async execution
func (b *FlyMachinesBackend) ExecAsync(ctx context.Context, sessionID string, req ExecRequest) (*ExecHandle, error) {
	_, err := b.getFlySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	execID := generateShortID()
	state := &flyExecState{
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

	return &ExecHandle{
		ID:        execID,
		SessionID: sessionID,
		Cmd:       req.Cmd,
		StartedAt: state.startedAt,
	}, nil
}

// ExecWait waits for an async execution to complete
func (b *FlyMachinesBackend) ExecWait(ctx context.Context, sessionID, execID string, timeout time.Duration) (*ExecResult, error) {
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
func (b *FlyMachinesBackend) ExecRead(ctx context.Context, sessionID, execID string, sinceSeq int, maxChunks int) (*ExecChunks, error) {
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

// WriteFile writes a file to the machine
func (b *FlyMachinesBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error {
	flySession, err := b.getFlySession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Normalize path
	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(flySession.Workdir, path)

	dir := filepath.Dir(fullPath)
	if dir != "" && dir != "." && dir != "/" {
		_, _ = b.execOnMachine(ctx, flySession.AppName, flySession.MachineID, shellCmd("mkdir", "-p", dir), 30)
	}

	b64Content := base64.StdEncoding.EncodeToString(content)
	fileMode := mode
	if fileMode == 0 {
		fileMode = 0644
	}

	innerCmd := fmt.Sprintf("echo %s | base64 -d > %s && chmod %04o %s",
		shellQuoteSingle(b64Content), shellQuoteSingle(fullPath), fileMode, shellQuoteSingle(fullPath))
	cmdStr := fmt.Sprintf(`sh -c %s`, shellQuoteSingle(innerCmd))

	result, err := b.execOnMachine(ctx, flySession.AppName, flySession.MachineID, cmdStr, 60)
	if err != nil {
		return &SandboxError{Op: "WriteFile", Session: sessionID, Err: fmt.Errorf("exec failed: %w", err)}
	}

	if result.ExitCode != 0 {
		return &SandboxError{Op: "WriteFile", Session: sessionID, Err: fmt.Errorf("write failed: %s", result.Stderr)}
	}

	return nil
}

// ReadFile reads a file from the machine
func (b *FlyMachinesBackend) ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, bool, error) {
	flySession, err := b.getFlySession(ctx, sessionID)
	if err != nil {
		return nil, false, err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(flySession.Workdir, path)

	if maxBytes <= 0 {
		maxBytes = b.config.MaxStdoutBytes
	}

	innerCmd := fmt.Sprintf("set -o pipefail && head -c %d %s | base64", maxBytes+1, shellQuoteSingle(fullPath))
	cmdStr := fmt.Sprintf(`sh -c %s`, shellQuoteSingle(innerCmd))

	result, err := b.execOnMachine(ctx, flySession.AppName, flySession.MachineID, cmdStr, 60)
	if err != nil {
		return nil, false, &SandboxError{Op: "ReadFile", Session: sessionID, Err: fmt.Errorf("exec failed: %w", err)}
	}

	if result.ExitCode != 0 {
		return nil, false, &SandboxError{Op: "ReadFile", Session: sessionID, Err: fmt.Errorf("read failed: %s", result.Stderr)}
	}

	// Decode base64
	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Stdout))
	if err != nil {
		return nil, false, &SandboxError{Op: "ReadFile", Session: sessionID, Err: fmt.Errorf("decode failed: %w", err)}
	}

	truncated := len(content) > maxBytes
	if truncated {
		content = content[:maxBytes]
	}

	return content, truncated, nil
}

// ListFiles lists files in a directory
func (b *FlyMachinesBackend) ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error) {
	flySession, err := b.getFlySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(flySession.Workdir, path)

	var innerCmd string
	if recursive {
		innerCmd = fmt.Sprintf("find %s -printf '%%y|%%s|%%m|%%T@|%%P\\n' 2>/dev/null | head -1000", shellQuoteSingle(fullPath))
	} else {
		innerCmd = fmt.Sprintf("ls -la %s 2>/dev/null | tail -n +2", shellQuoteSingle(fullPath))
	}
	cmdStr := fmt.Sprintf(`sh -c %s`, shellQuoteSingle(innerCmd))

	result, err := b.execOnMachine(ctx, flySession.AppName, flySession.MachineID, cmdStr, 60)
	if err != nil {
		return nil, &SandboxError{Op: "ListFiles", Session: sessionID, Err: fmt.Errorf("exec failed: %w", err)}
	}

	// Parse the output
	var entries []FileEntry
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")

	if recursive {
		// Parse find output: type|size|mode|mtime|path
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 5)
			if len(parts) < 5 {
				continue
			}

			entryType := "file"
			if parts[0] == "d" {
				entryType = "dir"
			}

			var size int64
			fmt.Sscanf(parts[1], "%d", &size)

			var mtimeUnix int64
			fmt.Sscanf(parts[3], "%d", &mtimeUnix)

			entries = append(entries, FileEntry{
				Path:      parts[4],
				Type:      entryType,
				Size:      size,
				Mode:      parts[2],
				MtimeUnix: mtimeUnix,
			})
		}
	} else {
		// Parse ls -la output
		for _, line := range lines {
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 9 {
				continue
			}

			entryType := "file"
			if strings.HasPrefix(fields[0], "d") {
				entryType = "dir"
			}

			var size int64
			fmt.Sscanf(fields[4], "%d", &size)

			// Reconstruct filename (might have spaces)
			name := strings.Join(fields[8:], " ")
			if name == "." || name == ".." {
				continue
			}

			entries = append(entries, FileEntry{
				Path: filepath.Join(path, name),
				Type: entryType,
				Size: size,
				Mode: fields[0],
			})
		}
	}

	return entries, nil
}

// DeleteFile deletes a file or directory
func (b *FlyMachinesBackend) DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error {
	flySession, err := b.getFlySession(ctx, sessionID)
	if err != nil {
		return err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(flySession.Workdir, path)

	// Safety check - don't delete workspace root
	if fullPath == flySession.Workdir || path == "." || path == "" {
		return &SandboxError{Op: "DeleteFile", Session: sessionID, Err: fmt.Errorf("cannot delete workspace root")}
	}

	var cmdStr string
	if recursive {
		cmdStr = shellCmd("rm", "-rf", fullPath)
	} else {
		cmdStr = shellCmd("rm", fullPath)
	}

	result, err := b.execOnMachine(ctx, flySession.AppName, flySession.MachineID, cmdStr, 60)
	if err != nil {
		return &SandboxError{Op: "DeleteFile", Session: sessionID, Err: fmt.Errorf("exec failed: %w", err)}
	}

	if result.ExitCode != 0 {
		return &SandboxError{Op: "DeleteFile", Session: sessionID, Err: fmt.Errorf("delete failed: %s", result.Stderr)}
	}

	return nil
}

// Close cleans up all sessions
func (b *FlyMachinesBackend) Close() error {
	b.mu.Lock()
	sessions := make([]*flyMachineSession, 0, len(b.sessions))
	for _, s := range b.sessions {
		sessions = append(sessions, s)
	}
	b.sessions = make(map[string]*flyMachineSession)
	b.mu.Unlock()

	// Destroy all machines
	ctx := context.Background()
	for _, s := range sessions {
		_ = b.destroyMachine(ctx, s.AppName, s.MachineID)
	}

	return nil
}

// =============================================================================
// Internal helper methods
// =============================================================================

func (b *FlyMachinesBackend) getFlySession(ctx context.Context, sessionID string) (*flyMachineSession, error) {
	b.mu.RLock()
	flySession, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	return flySession, nil
}

func (b *FlyMachinesBackend) buildShellCommand(cmd []string, cwd string, env map[string]string) string {
	envPrefix := ""
	for k, v := range env {
		envPrefix += fmt.Sprintf("%s=%s ", shellQuoteSingle(k), shellQuoteSingle(v))
	}

	cmdStr := shellQuoteJoin(cmd)
	if cwd != "" {
		cmdStr = fmt.Sprintf("cd %s && %s%s", shellQuoteSingle(cwd), envPrefix, cmdStr)
	} else if envPrefix != "" {
		cmdStr = envPrefix + cmdStr
	}

	return fmt.Sprintf(`sh -c %s`, shellQuoteSingle(cmdStr))
}

func (b *FlyMachinesBackend) truncateOutput(data []byte) (string, bool) {
	if len(data) <= b.config.MaxStdoutBytes {
		return string(data), false
	}
	return string(data[:b.config.MaxStdoutBytes]) + "\n... [truncated]", true
}

func (b *FlyMachinesBackend) buildImageRef(image string) *flyImageRef {
	registry := b.config.RegistryAuth.ServerAddress
	repository := image
	tag := "latest"

	if idx := strings.LastIndex(image, ":"); idx != -1 {
		repository = image[:idx]
		tag = image[idx+1:]
	}

	if registry != "" && strings.HasPrefix(repository, registry+"/") {
		repository = strings.TrimPrefix(repository, registry+"/")
	} else if strings.Contains(repository, "/") {
		parts := strings.SplitN(repository, "/", 2)
		if strings.Contains(parts[0], ".") {
			registry = parts[0]
			repository = parts[1]
		}
	}

	return &flyImageRef{
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
		Credentials: &flyImageCredentials{
			Username: b.config.RegistryAuth.Username,
			Password: b.config.RegistryAuth.Password,
		},
	}
}

// =============================================================================
// Fly Machines API methods
// =============================================================================

func (b *FlyMachinesBackend) ensureApp(ctx context.Context, appName string) error {
	// Check if app exists
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.machines.dev/v1/apps/%s", appName),
		nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.config.APIToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("get app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil // App exists
	}

	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("get app failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Create the app
	createReq := map[string]string{
		"app_name": appName,
		"org_slug": b.config.OrgSlug,
	}
	bodyBytes, _ := json.Marshal(createReq)

	req, err = http.NewRequestWithContext(ctx, "POST",
		"https://api.machines.dev/v1/apps",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err = b.client.Do(req)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create app failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (b *FlyMachinesBackend) createMachine(ctx context.Context, appName string, machineReq flyMachineCreateRequest) (*flyMachineResponse, error) {
	bodyBytes, err := json.Marshal(machineReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines", appName),
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create machine failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var machineResp flyMachineResponse
	if err := json.Unmarshal(body, &machineResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &machineResp, nil
}

func (b *FlyMachinesBackend) getMachine(ctx context.Context, appName, machineID string) (*flyMachineResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines/%s", appName, machineID),
		nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.config.APIToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get machine failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var machineResp flyMachineResponse
	if err := json.NewDecoder(resp.Body).Decode(&machineResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &machineResp, nil
}

func (b *FlyMachinesBackend) waitForMachineState(ctx context.Context, appName, machineID, targetState string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		machine, err := b.getMachine(ctx, appName, machineID)
		if err != nil {
			return err
		}

		if machine.State == targetState {
			return nil
		}

		if machine.State == "failed" || machine.State == "destroyed" {
			return fmt.Errorf("machine entered state: %s", machine.State)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return fmt.Errorf("timeout waiting for machine state %s", targetState)
}

func shellQuoteJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuoteSingle(arg)
	}
	return strings.Join(quoted, " ")
}

func shellQuoteSingle(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\r'\"\\$`!&|;<>(){}[]#*?~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func shellCmd(args ...string) string {
	return shellQuoteJoin(args)
}

func (b *FlyMachinesBackend) execOnMachine(ctx context.Context, appName, machineID string, cmdStr string, timeoutSec int) (*flyExecResponse, error) {
	execReq := flyExecRequest{
		Cmd:     cmdStr,
		Timeout: timeoutSec,
	}
	bodyBytes, err := json.Marshal(execReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines/%s/exec", appName, machineID),
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exec failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var execResp flyExecResponse
	if err := json.Unmarshal(body, &execResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &execResp, nil
}

func (b *FlyMachinesBackend) destroyMachine(ctx context.Context, appName, machineID string) error {
	// First stop the machine
	stopReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines/%s/stop", appName, machineID),
		nil)
	if err == nil {
		stopReq.Header.Set("Authorization", "Bearer "+b.config.APIToken)
		stopResp, _ := b.client.Do(stopReq)
		if stopResp != nil {
			stopResp.Body.Close()
		}
	}

	// Wait a moment for stop to complete
	time.Sleep(500 * time.Millisecond)

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines/%s?force=true", appName, machineID),
		nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+b.config.APIToken)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete machine failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
