package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type HostSandbox struct {
	id     string
	config Config
}

func NewHostSandbox(cfg Config) (*HostSandbox, error) {
	return &HostSandbox{
		id:     uuid.New().String(),
		config: cfg,
	}, nil
}

func (s *HostSandbox) Create(ctx context.Context) error {
	if s.config.WorkspacePath != "" {
		return os.MkdirAll(s.config.WorkspacePath, 0755)
	}
	return nil
}

func (s *HostSandbox) Exec(ctx context.Context, command string, args ...string) (*ExecResult, error) {
	return s.ExecWithStdin(ctx, nil, command, args...)
}

func (s *HostSandbox) ExecWithStdin(ctx context.Context, stdin io.Reader, command string, args ...string) (*ExecResult, error) {
	start := time.Now()

	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, command, args...)

	if s.config.WorkspacePath != "" {
		cmd.Dir = s.config.WorkspacePath
	}

	for k, v := range s.config.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if len(cmd.Env) > 0 {
		cmd.Env = append(os.Environ(), cmd.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdin != nil {
		cmd.Stdin = stdin
	}

	err := cmd.Run()
	duration := time.Since(start)

	result := &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Killed = true
		result.KillReason = "timeout"
		result.ExitCode = -1
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	return result, nil
}

func (s *HostSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	fullPath := s.resolvePath(path)
	return os.ReadFile(fullPath)
}

func (s *HostSandbox) WriteFile(ctx context.Context, path string, content []byte, mode uint32) error {
	fullPath := s.resolvePath(path)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	if mode == 0 {
		mode = 0644
	}

	return os.WriteFile(fullPath, content, os.FileMode(mode))
}

func (s *HostSandbox) DeleteFile(ctx context.Context, path string) error {
	fullPath := s.resolvePath(path)
	return os.Remove(fullPath)
}

func (s *HostSandbox) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	fullPath := s.resolvePath(path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			Mode:    uint32(info.Mode()),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
		})
	}

	return files, nil
}

func (s *HostSandbox) FileExists(ctx context.Context, path string) (bool, error) {
	fullPath := s.resolvePath(path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *HostSandbox) CopyIn(ctx context.Context, hostPath, sandboxPath string) error {
	content, err := os.ReadFile(hostPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(hostPath)
	if err != nil {
		return err
	}

	return s.WriteFile(ctx, sandboxPath, content, uint32(info.Mode()))
}

func (s *HostSandbox) CopyOut(ctx context.Context, sandboxPath, hostPath string) error {
	content, err := s.ReadFile(ctx, sandboxPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(hostPath, content, 0644)
}

func (s *HostSandbox) GetMetrics(ctx context.Context) (*Metrics, error) {
	return &Metrics{}, nil
}

func (s *HostSandbox) Destroy(ctx context.Context) error {
	return nil
}

func (s *HostSandbox) ID() string {
	return s.id
}

func (s *HostSandbox) Config() *Config {
	return &s.config
}

func (s *HostSandbox) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if s.config.WorkspacePath != "" {
		return filepath.Join(s.config.WorkspacePath, path)
	}
	return path
}
