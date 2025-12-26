package services

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerBackend struct {
	client *client.Client
}

func NewDockerBackend() (*DockerBackend, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerBackend{client: cli}, nil
}

func (b *DockerBackend) Close() error {
	return b.client.Close()
}

func (b *DockerBackend) CreateSession(ctx context.Context, key SessionKey, cfg SessionConfig) (*CodeSession, error) {
	img := cfg.Image
	if img == "" {
		img = RuntimeToDefaultImage(cfg.Runtime)
	}

	_, _, err := b.client.ImageInspectWithRaw(ctx, img)
	if err != nil {
		pullReader, pullErr := b.client.ImagePull(ctx, img, image.PullOptions{})
		if pullErr != nil {
			return nil, fmt.Errorf("failed to pull image %s: %w", img, pullErr)
		}
		defer pullReader.Close()
		io.Copy(io.Discard, pullReader)
	}

	workdir := cfg.Workdir
	if workdir == "" {
		workdir = "/work"
	}

	containerName := fmt.Sprintf("station-sandbox-%s", key.String())

	envVars := make([]string, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	containerCfg := &container.Config{
		Image:      img,
		Cmd:        []string{"tail", "-f", "/dev/null"},
		WorkingDir: workdir,
		Env:        envVars,
		Labels: map[string]string{
			"station.sandbox":     "true",
			"station.session.key": key.String(),
		},
	}

	hostCfg := &container.HostConfig{
		NetworkMode: "none",
		Resources: container.Resources{
			Memory:   512 * 1024 * 1024,
			NanoCPUs: 1000000000,
		},
	}

	if cfg.AllowNetwork {
		hostCfg.NetworkMode = "bridge"
	}

	resp, err := b.client.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := b.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		b.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	execResp, err := b.client.ContainerExecCreate(ctx, resp.ID, container.ExecOptions{
		Cmd: []string{"mkdir", "-p", workdir},
	})
	if err == nil {
		b.client.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{})
	}

	now := time.Now()
	session := &CodeSession{
		ID:          resp.ID,
		ContainerID: resp.ID,
		Key:         key,
		Config:      cfg,
		Status:      SessionStatusReady,
		CreatedAt:   now,
		LastUsedAt:  now,
	}

	return session, nil
}

func (b *DockerBackend) DestroySession(ctx context.Context, sessionID string) error {
	return b.client.ContainerRemove(ctx, sessionID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

func (b *DockerBackend) GetSession(ctx context.Context, sessionID string) (*CodeSession, error) {
	info, err := b.client.ContainerInspect(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("container not found: %w", err)
	}

	status := SessionStatusReady
	if !info.State.Running {
		status = SessionStatusError
	}

	return &CodeSession{
		ID:          sessionID,
		ContainerID: sessionID,
		Status:      status,
	}, nil
}

func (b *DockerBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error) {
	start := time.Now()

	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	cmd := b.buildExecCommand(req.Command, req.Args, timeout)

	workdir := req.Workdir
	if workdir == "" {
		workdir = "/work"
	}

	envVars := make([]string, 0, len(req.Env))
	for k, v := range req.Env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	execCfg := container.ExecOptions{
		Cmd:          cmd,
		WorkingDir:   workdir,
		Env:          envVars,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := b.client.ContainerExecCreate(ctx, sessionID, execCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := b.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read exec output: %w", err)
	}

	inspectResp, err := b.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec: %w", err)
	}

	timedOut := inspectResp.ExitCode == 124

	return &ExecResult{
		ExitCode:   inspectResp.ExitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: time.Since(start).Milliseconds(),
		TimedOut:   timedOut,
	}, nil
}

func (b *DockerBackend) buildExecCommand(command string, args []string, timeoutSeconds int) []string {
	fullCmd := command
	if len(args) > 0 {
		fullCmd = command + " " + strings.Join(args, " ")
	}

	return []string{
		"timeout", fmt.Sprintf("%d", timeoutSeconds),
		"sh", "-c", fullCmd,
	}
}

func (b *DockerBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte) error {
	if !filepath.IsAbs(path) {
		path = filepath.Join("/work", path)
	}

	dir := filepath.Dir(path)
	mkdirExec, err := b.client.ContainerExecCreate(ctx, sessionID, container.ExecOptions{
		Cmd: []string{"mkdir", "-p", dir},
	})
	if err == nil {
		b.client.ContainerExecStart(ctx, mkdirExec.ID, container.ExecStartOptions{})
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name: filepath.Base(path),
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar: %w", err)
	}

	return b.client.CopyToContainer(ctx, sessionID, dir, &buf, container.CopyToContainerOptions{})
}

func (b *DockerBackend) ReadFile(ctx context.Context, sessionID, path string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join("/work", path)
	}

	reader, _, err := b.client.CopyFromContainer(ctx, sessionID, path)
	if err != nil {
		return nil, fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	_, err = tr.Next()
	if err != nil {
		return nil, fmt.Errorf("failed to read tar header: %w", err)
	}

	return io.ReadAll(tr)
}

func (b *DockerBackend) ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join("/work", path)
	}

	var cmd []string
	if recursive {
		cmd = []string{"find", path, "-maxdepth", "10", "-printf", "%y %s %T@ %P\\n"}
	} else {
		cmd = []string{"find", path, "-maxdepth", "1", "-mindepth", "1", "-printf", "%y %s %T@ %P\\n"}
	}

	result, err := b.Exec(ctx, sessionID, ExecRequest{
		Command:        strings.Join(cmd, " "),
		TimeoutSeconds: 30,
	})
	if err != nil {
		return nil, err
	}

	var entries []FileEntry
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 4)
		if len(parts) < 4 {
			continue
		}

		typeChar := parts[0]
		var fileType FileType
		switch typeChar {
		case "f":
			fileType = FileTypeFile
		case "d":
			fileType = FileTypeDirectory
		case "l":
			fileType = FileTypeSymlink
		default:
			fileType = FileTypeFile
		}

		var size int64
		fmt.Sscanf(parts[1], "%d", &size)

		name := parts[3]
		entries = append(entries, FileEntry{
			Name:      filepath.Base(name),
			Path:      filepath.Join(path, name),
			Type:      fileType,
			SizeBytes: size,
		})
	}

	return entries, nil
}

func (b *DockerBackend) DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error {
	if !filepath.IsAbs(path) {
		path = filepath.Join("/work", path)
	}

	var cmd string
	if recursive {
		cmd = fmt.Sprintf("rm -rf %s", path)
	} else {
		cmd = fmt.Sprintf("rm -f %s", path)
	}

	result, err := b.Exec(ctx, sessionID, ExecRequest{
		Command:        cmd,
		TimeoutSeconds: 30,
	})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("delete failed: %s", result.Stderr)
	}
	return nil
}

var _ SandboxBackend = (*DockerBackend)(nil)
