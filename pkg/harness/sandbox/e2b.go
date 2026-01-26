package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"station/pkg/harness/sandbox/e2b"
)

type E2BSandbox struct {
	cfg             Config
	client          *e2b.Client
	streamingClient *e2b.StreamingClient
	sandboxID       string
	created         bool
}

type E2BConfig struct {
	APIKey     string
	TemplateID string
	TimeoutSec int
}

func NewE2BSandbox(cfg Config, e2bCfg E2BConfig) (*E2BSandbox, error) {
	return newE2BSandbox(cfg, e2bCfg, "")
}

func NewE2BSandboxWithAPIURL(cfg Config, e2bCfg E2BConfig, apiURL string) (*E2BSandbox, error) {
	return newE2BSandbox(cfg, e2bCfg, apiURL)
}

func newE2BSandbox(cfg Config, e2bCfg E2BConfig, apiURL string) (*E2BSandbox, error) {
	if e2bCfg.APIKey == "" {
		e2bCfg.APIKey = os.Getenv("E2B_API_KEY")
	}
	if e2bCfg.APIKey == "" {
		return nil, fmt.Errorf("E2B_API_KEY is required")
	}

	if e2bCfg.TemplateID == "" {
		e2bCfg.TemplateID = "base"
	}

	if e2bCfg.TimeoutSec == 0 {
		e2bCfg.TimeoutSec = int(cfg.Timeout.Seconds())
		if e2bCfg.TimeoutSec == 0 {
			e2bCfg.TimeoutSec = 300
		}
	}

	clientCfg := e2b.ClientConfig{
		APIKey:  e2bCfg.APIKey,
		Timeout: 60 * time.Second,
	}
	if apiURL != "" {
		clientCfg.APIURL = apiURL
	}

	client := e2b.NewClient(clientCfg)

	return &E2BSandbox{
		cfg:    cfg,
		client: client,
	}, nil
}

func (s *E2BSandbox) Create(ctx context.Context) error {
	if s.created {
		return nil
	}

	templateID := "base"
	if s.cfg.Image != "" {
		templateID = s.cfg.Image
	}

	timeoutSec := int(s.cfg.Timeout.Seconds())
	if timeoutSec == 0 {
		timeoutSec = 300
	}

	info, err := s.client.CreateSandbox(ctx, e2b.CreateSandboxRequest{
		TemplateID: templateID,
		Timeout:    timeoutSec,
		EnvVars:    s.cfg.Environment,
	})
	if err != nil {
		return fmt.Errorf("create e2b sandbox: %w", err)
	}

	s.sandboxID = info.SandboxID
	s.created = true

	s.streamingClient = e2b.NewStreamingClient(s.client.EnvdURL(), info.EnvdAccessToken)

	if s.cfg.WorkspacePath != "" {
		_ = s.client.MakeDir(ctx, s.cfg.WorkspacePath)
	}

	return nil
}

func (s *E2BSandbox) Exec(ctx context.Context, command string, args ...string) (*ExecResult, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	start := time.Now()

	// Build full command with working directory if specified
	fullCmd := command
	if len(args) > 0 {
		for _, arg := range args {
			fullCmd += " " + arg
		}
	}

	// Wrap with cd if workspace path is set
	if s.cfg.WorkspacePath != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", s.cfg.WorkspacePath, fullCmd)
	}

	result, err := s.client.Exec(ctx, "sh", "-c", fullCmd)
	if err != nil {
		return nil, fmt.Errorf("exec in e2b sandbox: %w", err)
	}

	return &ExecResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		Duration: time.Since(start),
	}, nil
}

func (s *E2BSandbox) ExecWithStdin(ctx context.Context, stdin io.Reader, command string, args ...string) (*ExecResult, error) {
	// E2B doesn't have native stdin support via REST, so we write to a temp file and redirect
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	// Read stdin content
	stdinContent, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}

	// Write to temp file
	tmpFile := "/tmp/.stdin_" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := s.client.WriteFile(ctx, tmpFile, stdinContent); err != nil {
		return nil, fmt.Errorf("write stdin to temp file: %w", err)
	}
	defer s.client.DeleteFile(ctx, tmpFile)

	// Build command with stdin redirect
	fullCmd := command
	if len(args) > 0 {
		for _, arg := range args {
			fullCmd += " " + arg
		}
	}
	fullCmd = fmt.Sprintf("cat %s | %s", tmpFile, fullCmd)

	return s.Exec(ctx, "sh", "-c", fullCmd)
}

func (s *E2BSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	return s.client.ReadFile(ctx, path)
}

func (s *E2BSandbox) WriteFile(ctx context.Context, path string, content []byte, mode uint32) error {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return err
		}
	}

	if err := s.client.WriteFile(ctx, path, content); err != nil {
		return err
	}

	// Set file mode if not default
	if mode != 0 && mode != 0644 {
		_, err := s.client.Exec(ctx, "chmod", fmt.Sprintf("%o", mode), path)
		if err != nil {
			// Non-fatal, log and continue
		}
	}

	return nil
}

func (s *E2BSandbox) DeleteFile(ctx context.Context, path string) error {
	if !s.created {
		return nil // Nothing to delete if sandbox doesn't exist
	}

	return s.client.DeleteFile(ctx, path)
}

func (s *E2BSandbox) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	e2bFiles, err := s.client.ListDir(ctx, path)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, len(e2bFiles))
	for i, f := range e2bFiles {
		var mode uint32 = 0644
		if f.IsDir {
			mode = 0755
		}

		files[i] = FileInfo{
			Name:    f.Name,
			Size:    f.Size,
			Mode:    mode,
			ModTime: f.ModTime,
			IsDir:   f.IsDir,
		}
	}

	return files, nil
}

func (s *E2BSandbox) FileExists(ctx context.Context, path string) (bool, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return false, err
		}
	}

	return s.client.FileExists(ctx, path)
}

func (s *E2BSandbox) CopyIn(ctx context.Context, hostPath, sandboxPath string) error {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return err
		}
	}

	// Read from host
	content, err := os.ReadFile(hostPath)
	if err != nil {
		return fmt.Errorf("read host file: %w", err)
	}

	// Write to sandbox
	return s.client.WriteFile(ctx, sandboxPath, content)
}

func (s *E2BSandbox) CopyOut(ctx context.Context, sandboxPath, hostPath string) error {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return err
		}
	}

	// Read from sandbox
	content, err := s.client.ReadFile(ctx, sandboxPath)
	if err != nil {
		return fmt.Errorf("read sandbox file: %w", err)
	}

	// Write to host
	return os.WriteFile(hostPath, content, 0644)
}

func (s *E2BSandbox) GetMetrics(ctx context.Context) (*Metrics, error) {
	if !s.created {
		return &Metrics{}, nil
	}

	// E2B doesn't expose detailed metrics, return basic info from ps
	result, err := s.client.Exec(ctx, "ps", "aux", "--no-headers", "|", "wc", "-l")
	if err != nil {
		return &Metrics{}, nil
	}

	var processCount int
	fmt.Sscanf(result.Stdout, "%d", &processCount)

	return &Metrics{
		ProcessCount: processCount,
	}, nil
}

func (s *E2BSandbox) Destroy(ctx context.Context) error {
	if !s.created {
		return nil
	}

	err := s.client.Kill(ctx)
	s.created = false
	s.sandboxID = ""
	return err
}

func (s *E2BSandbox) ID() string {
	return s.sandboxID
}

func (s *E2BSandbox) Config() *Config {
	return &s.cfg
}

func (s *E2BSandbox) ExecStream(ctx context.Context, opts ExecOptions, command string, args ...string) (ProcessHandle, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	cwd := opts.Cwd
	if cwd == "" {
		cwd = s.cfg.WorkspacePath
	}

	env := make(map[string]string)
	for k, v := range s.cfg.Environment {
		env[k] = v
	}
	for k, v := range opts.Env {
		env[k] = v
	}

	if s.streamingClient != nil {
		streamHandle, err := s.streamingClient.ExecStream(ctx, e2b.StreamExecOptions{
			Cmd:      command,
			Args:     args,
			Cwd:      cwd,
			Env:      env,
			OnStdout: opts.OnStdout,
			OnStderr: opts.OnStderr,
		})
		if err != nil {
			return nil, fmt.Errorf("start streaming exec: %w", err)
		}

		return &e2bStreamingHandle{
			handle:   streamHandle,
			start:    time.Now(),
			onStdout: opts.OnStdout,
			onStderr: opts.OnStderr,
		}, nil
	}

	fullCmd := command
	for _, arg := range args {
		fullCmd += " " + arg
	}
	if cwd != "" {
		fullCmd = fmt.Sprintf("cd %s && %s", cwd, fullCmd)
	}

	handle := &e2bProcessHandle{
		sandbox:  s,
		ctx:      ctx,
		command:  fullCmd,
		onStdout: opts.OnStdout,
		onStderr: opts.OnStderr,
		done:     make(chan struct{}),
		start:    time.Now(),
	}

	go handle.run()

	return handle, nil
}

func (s *E2BSandbox) ListProcesses(ctx context.Context) ([]ProcessInfo, error) {
	result, err := s.client.Exec(ctx, "ps", "-eo", "pid,cmd", "--no-headers")
	if err != nil {
		return nil, err
	}

	var processes []ProcessInfo
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		var pid int
		fmt.Sscanf(fields[0], "%d", &pid)
		processes = append(processes, ProcessInfo{
			PID:     pid,
			Command: strings.Join(fields[1:], " "),
		})
	}

	return processes, nil
}

func (s *E2BSandbox) KillProcess(ctx context.Context, pid int) error {
	_, err := s.client.Exec(ctx, "kill", "-9", fmt.Sprintf("%d", pid))
	return err
}

var _ Sandbox = (*E2BSandbox)(nil)
var _ StreamingSandbox = (*E2BSandbox)(nil)

type e2bProcessHandle struct {
	sandbox  *E2BSandbox
	ctx      context.Context
	command  string
	onStdout func([]byte)
	onStderr func([]byte)
	done     chan struct{}
	start    time.Time
	result   *ExecResult
	err      error
}

func (h *e2bProcessHandle) PID() int {
	return 0
}

func (h *e2bProcessHandle) Wait() (*ExecResult, error) {
	<-h.done
	return h.result, h.err
}

func (h *e2bProcessHandle) Kill() error {
	return fmt.Errorf("kill not supported for E2B streaming exec")
}

func (h *e2bProcessHandle) SendStdin(data []byte) error {
	return fmt.Errorf("stdin not supported for E2B streaming exec")
}

func (h *e2bProcessHandle) run() {
	defer close(h.done)

	execResult, err := h.sandbox.client.Exec(h.ctx, "sh", "-c", h.command)
	if err != nil {
		h.err = err
		return
	}

	if h.onStdout != nil && execResult.Stdout != "" {
		h.onStdout([]byte(execResult.Stdout))
	}
	if h.onStderr != nil && execResult.Stderr != "" {
		h.onStderr([]byte(execResult.Stderr))
	}

	h.result = &ExecResult{
		ExitCode: execResult.ExitCode,
		Stdout:   execResult.Stdout,
		Stderr:   execResult.Stderr,
		Duration: time.Since(h.start),
	}
}

type e2bStreamingHandle struct {
	handle   *e2b.StreamExecHandle
	start    time.Time
	onStdout func([]byte)
	onStderr func([]byte)
	stdout   []byte
	stderr   []byte
}

func (h *e2bStreamingHandle) PID() int {
	return int(h.handle.PID())
}

func (h *e2bStreamingHandle) Wait() (*ExecResult, error) {
	exitCode, exitError, err := h.handle.Wait()
	if err != nil {
		return nil, err
	}

	result := &ExecResult{
		ExitCode: int(exitCode),
		Stdout:   string(h.stdout),
		Stderr:   string(h.stderr),
		Duration: time.Since(h.start),
	}

	if exitError != "" {
		result.Stderr += exitError
	}

	return result, nil
}

func (h *e2bStreamingHandle) Kill() error {
	return h.handle.Close()
}

func (h *e2bStreamingHandle) SendStdin(data []byte) error {
	return fmt.Errorf("stdin via streaming handle not yet implemented")
}
