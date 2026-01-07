package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DockerSandbox struct {
	id          string
	containerID string
	config      Config
	created     bool
}

func NewDockerSandbox(cfg Config) (*DockerSandbox, error) {
	if cfg.Image == "" {
		cfg.Image = "ubuntu:22.04"
	}

	return &DockerSandbox{
		id:     uuid.New().String(),
		config: cfg,
	}, nil
}

func (s *DockerSandbox) Create(ctx context.Context) error {
	if s.created {
		return nil
	}

	args := []string{
		"create",
		"--name", s.containerName(),
		"--rm",
	}

	if s.config.Resources.CPU > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", s.config.Resources.CPU))
	}

	if s.config.Resources.Memory != "" {
		args = append(args, "--memory", s.config.Resources.Memory)
	}

	if s.config.Resources.PIDs > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", s.config.Resources.PIDs))
	}

	if !s.config.Network.Enabled {
		args = append(args, "--network", "none")
	}

	if s.config.WorkspacePath != "" {
		absPath, err := filepath.Abs(s.config.WorkspacePath)
		if err != nil {
			return fmt.Errorf("failed to get absolute workspace path: %w", err)
		}

		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("failed to create workspace directory: %w", err)
		}

		args = append(args, "-v", fmt.Sprintf("%s:/workspace", absPath))
		args = append(args, "-w", "/workspace")
	}

	for k, v := range s.config.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, "--cap-drop", "ALL")
	args = append(args, "--security-opt", "no-new-privileges")
	args = append(args, "--read-only")
	args = append(args, "--tmpfs", "/tmp:rw,noexec,nosuid,size=100m")

	args = append(args, s.config.Image, "tail", "-f", "/dev/null")

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create container: %w, output: %s", err, string(output))
	}

	s.containerID = strings.TrimSpace(string(output))

	startCmd := exec.CommandContext(ctx, "docker", "start", s.containerName())
	if output, err := startCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start container: %w, output: %s", err, string(output))
	}

	s.created = true
	return nil
}

func (s *DockerSandbox) Exec(ctx context.Context, command string, args ...string) (*ExecResult, error) {
	return s.ExecWithStdin(ctx, nil, command, args...)
}

func (s *DockerSandbox) ExecWithStdin(ctx context.Context, stdin io.Reader, command string, args ...string) (*ExecResult, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	start := time.Now()

	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	dockerArgs := []string{"exec"}
	if stdin != nil {
		dockerArgs = append(dockerArgs, "-i")
	}
	dockerArgs = append(dockerArgs, s.containerName(), command)
	dockerArgs = append(dockerArgs, args...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

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

func (s *DockerSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	containerPath := s.resolveContainerPath(path)

	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName(), "cat", containerPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", containerPath, err)
	}

	return output, nil
}

func (s *DockerSandbox) WriteFile(ctx context.Context, path string, content []byte, mode uint32) error {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return err
		}
	}

	containerPath := s.resolveContainerPath(path)

	dir := filepath.Dir(containerPath)
	mkdirCmd := exec.CommandContext(ctx, "docker", "exec", s.containerName(), "mkdir", "-p", dir)
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", s.containerName(), "tee", containerPath)
	cmd.Stdin = bytes.NewReader(content)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write file %s: %w", containerPath, err)
	}

	if mode != 0 {
		chmodCmd := exec.CommandContext(ctx, "docker", "exec", s.containerName(),
			"chmod", fmt.Sprintf("%o", mode), containerPath)
		if err := chmodCmd.Run(); err != nil {
			return fmt.Errorf("failed to chmod file %s: %w", containerPath, err)
		}
	}

	return nil
}

func (s *DockerSandbox) DeleteFile(ctx context.Context, path string) error {
	if !s.created {
		return nil
	}

	containerPath := s.resolveContainerPath(path)
	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName(), "rm", "-f", containerPath)
	return cmd.Run()
}

func (s *DockerSandbox) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return nil, err
		}
	}

	containerPath := s.resolveContainerPath(path)

	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName(),
		"find", containerPath, "-maxdepth", "1", "-printf", "%f\\t%s\\t%m\\t%T@\\t%y\\n")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", containerPath, err)
	}

	var files []FileInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}

		name := parts[0]
		if name == filepath.Base(containerPath) {
			continue
		}

		var size int64
		fmt.Sscanf(parts[1], "%d", &size)

		var mode uint32
		fmt.Sscanf(parts[2], "%o", &mode)

		var modTimeUnix float64
		fmt.Sscanf(parts[3], "%f", &modTimeUnix)
		modTime := time.Unix(int64(modTimeUnix), 0)

		isDir := parts[4] == "d"

		files = append(files, FileInfo{
			Name:    name,
			Size:    size,
			Mode:    mode,
			ModTime: modTime,
			IsDir:   isDir,
		})
	}

	return files, nil
}

func (s *DockerSandbox) FileExists(ctx context.Context, path string) (bool, error) {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return false, err
		}
	}

	containerPath := s.resolveContainerPath(path)
	cmd := exec.CommandContext(ctx, "docker", "exec", s.containerName(), "test", "-e", containerPath)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, err
}

func (s *DockerSandbox) CopyIn(ctx context.Context, hostPath, sandboxPath string) error {
	if !s.created {
		if err := s.Create(ctx); err != nil {
			return err
		}
	}

	containerPath := s.resolveContainerPath(sandboxPath)
	cmd := exec.CommandContext(ctx, "docker", "cp", hostPath, fmt.Sprintf("%s:%s", s.containerName(), containerPath))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy %s to container: %w, output: %s", hostPath, err, string(output))
	}
	return nil
}

func (s *DockerSandbox) CopyOut(ctx context.Context, sandboxPath, hostPath string) error {
	if !s.created {
		return fmt.Errorf("container not created")
	}

	containerPath := s.resolveContainerPath(sandboxPath)

	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "cp", fmt.Sprintf("%s:%s", s.containerName(), containerPath), hostPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy %s from container: %w, output: %s", containerPath, err, string(output))
	}
	return nil
}

func (s *DockerSandbox) GetMetrics(ctx context.Context) (*Metrics, error) {
	if !s.created {
		return &Metrics{}, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format",
		"{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}", s.containerName())

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}

	metrics := &Metrics{}
	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) >= 1 {
		cpuStr := strings.TrimSuffix(parts[0], "%")
		fmt.Sscanf(cpuStr, "%f", &metrics.CPUUsage)
	}
	if len(parts) >= 5 {
		var pids int
		fmt.Sscanf(parts[4], "%d", &pids)
		metrics.ProcessCount = pids
	}

	return metrics, nil
}

func (s *DockerSandbox) Destroy(ctx context.Context) error {
	if !s.created {
		return nil
	}

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", s.containerName())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	s.created = false
	return nil
}

func (s *DockerSandbox) ID() string {
	return s.id
}

func (s *DockerSandbox) Config() *Config {
	return &s.config
}

func (s *DockerSandbox) containerName() string {
	return fmt.Sprintf("station-sandbox-%s", s.id[:8])
}

func (s *DockerSandbox) resolveContainerPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join("/workspace", path)
}
