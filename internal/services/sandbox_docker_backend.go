package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerBackend implements SandboxBackend using the Docker API with persistent
// containers and bind-mounted workspaces.
type DockerBackend struct {
	client   *client.Client
	config   CodeModeConfig
	mu       sync.RWMutex
	sessions map[string]*Session
	execs    map[string]*execState
}

type execState struct {
	id        string
	sessionID string
	cmd       []string
	startedAt time.Time
	done      chan struct{}
	result    *ExecResult
	chunks    []OutputChunk
	chunkMu   sync.Mutex
	nextSeq   int
	cancelled bool
}

func NewDockerBackend(cfg CodeModeConfig) (*DockerBackend, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	if cfg.DockerHost != "" {
		opts = append(opts, client.WithHost(cfg.DockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	if err := os.MkdirAll(cfg.WorkspaceBaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace base dir: %w", err)
	}

	return &DockerBackend{
		client:   cli,
		config:   cfg,
		sessions: make(map[string]*Session),
		execs:    make(map[string]*execState),
	}, nil
}

func (b *DockerBackend) Ping(ctx context.Context) error {
	_, err := b.client.Ping(ctx)
	if err != nil {
		return &SandboxError{Op: "Ping", Err: fmt.Errorf("docker not available: %w", err)}
	}
	return nil
}

func (b *DockerBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	if !b.isImageAllowed(opts.Image) {
		return nil, &SandboxError{Op: "CreateSession", Err: ErrImageNotAllowed}
	}

	sessionID := fmt.Sprintf("sbx_%s", generateShortID())
	workspacePath := filepath.Join(b.config.WorkspaceBaseDir, sessionID)

	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("create workspace: %w", err)}
	}

	if err := b.ensureImage(ctx, opts.Image); err != nil {
		os.RemoveAll(workspacePath)
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("pull image: %w", err)}
	}

	workdir := opts.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}

	env := make([]string, 0, len(opts.Env))
	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	containerConfig := &container.Config{
		Image:      opts.Image,
		WorkingDir: workdir,
		Env:        env,
		Tty:        false,
		OpenStdin:  true,
		StdinOnce:  false,
		Cmd:        []string{"sleep", "infinity"},
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: workspacePath,
				Target: workdir,
			},
		},
		AutoRemove:  false,
		NetworkMode: "none",
	}

	if opts.Limits.CPUMillicores > 0 {
		hostConfig.Resources.NanoCPUs = int64(opts.Limits.CPUMillicores) * 1e6
	}
	if opts.Limits.MemoryMB > 0 {
		hostConfig.Resources.Memory = int64(opts.Limits.MemoryMB) * 1024 * 1024
	}

	if opts.NetworkEnabled {
		hostConfig.NetworkMode = "bridge"
	}

	resp, err := b.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, sessionID)
	if err != nil {
		os.RemoveAll(workspacePath)
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("create container: %w", err)}
	}

	if err := b.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		b.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		os.RemoveAll(workspacePath)
		return nil, &SandboxError{Op: "CreateSession", Session: sessionID, Err: fmt.Errorf("start container: %w", err)}
	}

	session := &Session{
		ID:            sessionID,
		ContainerID:   resp.ID,
		Image:         opts.Image,
		Workdir:       workdir,
		WorkspacePath: workspacePath,
		Env:           opts.Env,
		Limits:        opts.Limits,
		CreatedAt:     time.Now(),
		LastUsedAt:    time.Now(),
	}

	b.mu.Lock()
	b.sessions[sessionID] = session
	b.mu.Unlock()

	return session, nil
}

func (b *DockerBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	b.mu.RLock()
	session, ok := b.sessions[sessionID]
	b.mu.RUnlock()

	if !ok {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: ErrSessionNotFound}
	}

	inspect, err := b.client.ContainerInspect(ctx, session.ContainerID)
	if err != nil || !inspect.State.Running {
		return nil, &SandboxError{Op: "GetSession", Session: sessionID, Err: ErrSessionClosed}
	}

	return session, nil
}

func (b *DockerBackend) DestroySession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	session, ok := b.sessions[sessionID]
	if ok {
		delete(b.sessions, sessionID)
	}
	b.mu.Unlock()

	if !ok {
		return &SandboxError{Op: "DestroySession", Session: sessionID, Err: ErrSessionNotFound}
	}

	timeout := 10
	_ = b.client.ContainerStop(ctx, session.ContainerID, container.StopOptions{Timeout: &timeout})
	_ = b.client.ContainerRemove(ctx, session.ContainerID, container.RemoveOptions{Force: true})

	if session.WorkspacePath != "" {
		os.RemoveAll(session.WorkspacePath)
	}

	return nil
}

func (b *DockerBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	if s, ok := b.sessions[sessionID]; ok {
		s.LastUsedAt = time.Now()
	}
	b.mu.Unlock()

	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = int(b.config.DefaultTimeout.Seconds())
	}

	cmd := append([]string{"timeout", fmt.Sprintf("%d", timeout)}, req.Cmd...)

	cwd := req.Cwd
	if cwd == "" {
		cwd = session.Workdir
	}

	env := make([]string, 0, len(req.Env))
	for k, v := range req.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		WorkingDir:   cwd,
		Env:          env,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := b.client.ContainerExecCreate(ctx, session.ContainerID, execConfig)
	if err != nil {
		return nil, &SandboxError{Op: "Exec", Session: sessionID, Err: fmt.Errorf("create exec: %w", err)}
	}

	attachResp, err := b.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, &SandboxError{Op: "Exec", Session: sessionID, Err: fmt.Errorf("attach exec: %w", err)}
	}
	defer attachResp.Close()

	start := time.Now()
	stdout, stderr, err := b.readExecOutput(ctx, attachResp.Reader)
	duration := time.Since(start)

	if err != nil && ctx.Err() != nil {
		b.killExec(context.Background(), session.ContainerID, req.Cmd)
		return &ExecResult{
			ID:        execResp.ID,
			ExitCode:  -1,
			Stdout:    string(stdout),
			Stderr:    string(stderr),
			Duration:  duration,
			Cancelled: true,
		}, nil
	}

	inspect, err := b.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, &SandboxError{Op: "Exec", Session: sessionID, Err: fmt.Errorf("inspect exec: %w", err)}
	}

	stdoutStr, stdoutTrunc := b.truncateOutput(stdout)
	stderrStr, stderrTrunc := b.truncateOutput(stderr)

	return &ExecResult{
		ID:        execResp.ID,
		ExitCode:  inspect.ExitCode,
		Stdout:    stdoutStr,
		Stderr:    stderrStr,
		Duration:  duration,
		Truncated: stdoutTrunc || stderrTrunc,
	}, nil
}

func (b *DockerBackend) ExecAsync(ctx context.Context, sessionID string, req ExecRequest) (*ExecHandle, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	if s, ok := b.sessions[sessionID]; ok {
		s.LastUsedAt = time.Now()
	}
	b.mu.Unlock()

	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = int(b.config.DefaultTimeout.Seconds())
	}

	cmd := append([]string{"timeout", fmt.Sprintf("%d", timeout)}, req.Cmd...)

	cwd := req.Cwd
	if cwd == "" {
		cwd = session.Workdir
	}

	env := make([]string, 0, len(req.Env))
	for k, v := range req.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		WorkingDir:   cwd,
		Env:          env,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := b.client.ContainerExecCreate(ctx, session.ContainerID, execConfig)
	if err != nil {
		return nil, &SandboxError{Op: "ExecAsync", Session: sessionID, Err: fmt.Errorf("create exec: %w", err)}
	}

	attachResp, err := b.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, &SandboxError{Op: "ExecAsync", Session: sessionID, Err: fmt.Errorf("attach exec: %w", err)}
	}

	state := &execState{
		id:        execResp.ID,
		sessionID: sessionID,
		cmd:       req.Cmd,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}

	b.mu.Lock()
	b.execs[execResp.ID] = state
	b.mu.Unlock()

	go b.collectExecOutput(state, attachResp, session.ContainerID)

	return &ExecHandle{
		ID:        execResp.ID,
		SessionID: sessionID,
		Cmd:       req.Cmd,
		StartedAt: state.startedAt,
	}, nil
}

func (b *DockerBackend) ExecWait(ctx context.Context, sessionID, execID string, timeout time.Duration) (*ExecResult, error) {
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

func (b *DockerBackend) ExecRead(ctx context.Context, sessionID, execID string, sinceSeq int, maxChunks int) (*ExecChunks, error) {
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

func (b *DockerBackend) collectExecOutput(state *execState, attachResp types.HijackedResponse, containerID string) {
	defer attachResp.Close()
	defer close(state.done)

	var stdoutBuf, stderrBuf bytes.Buffer
	_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)

	duration := time.Since(state.startedAt)

	state.chunkMu.Lock()
	if stdoutBuf.Len() > 0 {
		state.chunks = append(state.chunks, OutputChunk{
			Seq:    state.nextSeq,
			Stream: "stdout",
			Text:   stdoutBuf.String(),
		})
		state.nextSeq++
	}
	if stderrBuf.Len() > 0 {
		state.chunks = append(state.chunks, OutputChunk{
			Seq:    state.nextSeq,
			Stream: "stderr",
			Text:   stderrBuf.String(),
		})
		state.nextSeq++
	}
	state.chunkMu.Unlock()

	var exitCode int
	if err == nil {
		inspect, inspectErr := b.client.ContainerExecInspect(context.Background(), state.id)
		if inspectErr == nil {
			exitCode = inspect.ExitCode
		}
	} else {
		exitCode = -1
	}

	stdoutStr, stdoutTrunc := b.truncateOutput(stdoutBuf.Bytes())
	stderrStr, stderrTrunc := b.truncateOutput(stderrBuf.Bytes())

	state.result = &ExecResult{
		ID:        state.id,
		ExitCode:  exitCode,
		Stdout:    stdoutStr,
		Stderr:    stderrStr,
		Duration:  duration,
		Truncated: stdoutTrunc || stderrTrunc,
		Cancelled: state.cancelled,
	}
}

func (b *DockerBackend) readExecOutput(ctx context.Context, reader io.Reader) (stdout, stderr []byte, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	doneCh := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, reader)
		doneCh <- err
	}()

	select {
	case err = <-doneCh:
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
	case <-ctx.Done():
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), ctx.Err()
	}
}

func (b *DockerBackend) killExec(ctx context.Context, containerID string, cmd []string) {
	if len(cmd) > 0 {
		pkillCmd := []string{"pkill", "-f", strings.Join(cmd, " ")}
		execConfig := container.ExecOptions{Cmd: pkillCmd}
		resp, err := b.client.ContainerExecCreate(ctx, containerID, execConfig)
		if err == nil {
			b.client.ContainerExecStart(ctx, resp.ID, container.ExecStartOptions{})
		}
	}
}

func (b *DockerBackend) truncateOutput(data []byte) (string, bool) {
	if len(data) <= b.config.MaxStdoutBytes {
		return string(data), false
	}
	return string(data[:b.config.MaxStdoutBytes]) + "\n... [truncated]", true
}

// normalizeWorkspacePath handles LLM path normalization: "/workspace/foo.py" -> "foo.py"
func normalizeWorkspacePath(path string) string {
	path = strings.TrimPrefix(path, "/")

	if path == "workspace" || path == "workspace/" {
		return "."
	}
	if strings.HasPrefix(path, "workspace/") {
		path = strings.TrimPrefix(path, "workspace/")
	}

	if path == "" {
		return "."
	}

	return path
}

func (b *DockerBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(session.WorkspacePath, path)

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

func (b *DockerBackend) ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, bool, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, false, err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(session.WorkspacePath, path)

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

func (b *DockerBackend) ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error) {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	path = normalizeWorkspacePath(path)
	basePath := filepath.Join(session.WorkspacePath, path)

	var entries []FileEntry

	if recursive {
		err = filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			relPath, _ := filepath.Rel(session.WorkspacePath, p)
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

func (b *DockerBackend) DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error {
	session, err := b.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	path = normalizeWorkspacePath(path)
	fullPath := filepath.Join(session.WorkspacePath, path)

	if fullPath == session.WorkspacePath {
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

func (b *DockerBackend) isImageAllowed(img string) bool {
	if len(b.config.AllowedImages) == 0 {
		return true
	}
	for _, allowed := range b.config.AllowedImages {
		if allowed == img {
			return true
		}
	}
	return false
}

func (b *DockerBackend) ensureImage(ctx context.Context, imageName string) error {
	_, _, err := b.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		return nil
	}

	reader, err := b.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	for {
		var msg map[string]interface{}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read pull response: %w", err)
		}
		if errMsg, ok := msg["error"].(string); ok {
			return fmt.Errorf("pull image: %s", errMsg)
		}
	}

	return nil
}

func generateShortID() string {
	return fmt.Sprintf("%d%04d", time.Now().UnixNano()%1e9, time.Now().Nanosecond()%10000)
}

func (b *DockerBackend) Close() error {
	return b.client.Close()
}
