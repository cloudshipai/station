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
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerIO struct {
	client *client.Client
}

func NewDockerIO(cli *client.Client) *DockerIO {
	return &DockerIO{client: cli}
}

func (d *DockerIO) CopyToContainer(ctx context.Context, containerID, destPath string, content []byte, mode int64) error {
	if mode == 0 {
		mode = 0644
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	header := &tar.Header{
		Name:    filepath.Base(destPath),
		Mode:    mode,
		Size:    int64(len(content)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}

	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}

	destDir := filepath.Dir(destPath)
	if destDir == "" || destDir == "." {
		destDir = "/"
	}

	return d.client.CopyToContainer(ctx, containerID, destDir, &buf, container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	})
}

func (d *DockerIO) CopyFromContainer(ctx context.Context, containerID, srcPath string) ([]byte, error) {
	reader, _, err := d.client.CopyFromContainer(ctx, containerID, srcPath)
	if err != nil {
		return nil, fmt.Errorf("copy from container: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	header, err := tr.Next()
	if err != nil {
		return nil, fmt.Errorf("read tar header: %w", err)
	}

	if header.Typeflag == tar.TypeDir {
		return nil, fmt.Errorf("path is a directory, not a file")
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		return nil, fmt.Errorf("read tar content: %w", err)
	}

	return content, nil
}

func (d *DockerIO) CopyFromContainerWithLimit(ctx context.Context, containerID, srcPath string, maxBytes int) ([]byte, bool, error) {
	reader, stat, err := d.client.CopyFromContainer(ctx, containerID, srcPath)
	if err != nil {
		return nil, false, fmt.Errorf("copy from container: %w", err)
	}
	defer reader.Close()

	truncated := stat.Size > int64(maxBytes)

	tr := tar.NewReader(reader)
	header, err := tr.Next()
	if err != nil {
		return nil, false, fmt.Errorf("read tar header: %w", err)
	}

	if header.Typeflag == tar.TypeDir {
		return nil, false, fmt.Errorf("path is a directory, not a file")
	}

	limitReader := io.LimitReader(tr, int64(maxBytes))
	content, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, false, fmt.Errorf("read tar content: %w", err)
	}

	return content, truncated, nil
}

type DockerExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (d *DockerIO) ExecSimple(ctx context.Context, containerID string, cmd []string) (*DockerExecResult, error) {
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("create exec: %w", err)
	}

	attachResp, err := d.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("attach exec: %w", err)
	}
	defer attachResp.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
	if err != nil {
		return nil, fmt.Errorf("read exec output: %w", err)
	}

	inspectResp, err := d.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("inspect exec: %w", err)
	}

	return &DockerExecResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: inspectResp.ExitCode,
	}, nil
}

func (d *DockerIO) MkdirP(ctx context.Context, containerID, path string) error {
	result, err := d.ExecSimple(ctx, containerID, []string{"mkdir", "-p", path})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("mkdir failed: %s", result.Stderr)
	}
	return nil
}

func (d *DockerIO) FileExists(ctx context.Context, containerID, path string) (bool, error) {
	result, err := d.ExecSimple(ctx, containerID, []string{"test", "-e", path})
	if err != nil {
		return false, err
	}
	return result.ExitCode == 0, nil
}

func (d *DockerIO) IsDir(ctx context.Context, containerID, path string) (bool, error) {
	result, err := d.ExecSimple(ctx, containerID, []string{"test", "-d", path})
	if err != nil {
		return false, err
	}
	return result.ExitCode == 0, nil
}

func (d *DockerIO) StatFile(ctx context.Context, containerID, path string) (*FileEntry, error) {
	// stat -c format: %s=size %a=mode %Y=mtime %F=type
	result, err := d.ExecSimple(ctx, containerID, []string{
		"stat", "-c", "%s %a %Y %F", path,
	})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("stat failed: %s", strings.TrimSpace(result.Stderr))
	}

	output := strings.TrimSpace(result.Stdout)
	parts := strings.SplitN(output, " ", 4)
	if len(parts) < 4 {
		return nil, fmt.Errorf("unexpected stat output: %s", output)
	}

	var size int64
	var mtime int64
	fmt.Sscanf(parts[0], "%d", &size)
	fmt.Sscanf(parts[2], "%d", &mtime)

	entryType := "file"
	if strings.Contains(parts[3], "directory") {
		entryType = "dir"
	} else if strings.Contains(parts[3], "symbolic link") {
		entryType = "symlink"
	}

	return &FileEntry{
		Path:      path,
		Type:      entryType,
		Size:      size,
		Mode:      parts[1],
		MtimeUnix: mtime,
	}, nil
}

func (d *DockerIO) ListDir(ctx context.Context, containerID, path string, recursive bool) ([]FileEntry, error) {
	var cmd []string
	if recursive {
		// find -printf format: %P=relative_path %s=size %m=mode %T@=mtime %y=type
		cmd = []string{
			"find", path, "-printf", "%P\t%s\t%m\t%T@\t%y\n",
		}
	} else {
		cmd = []string{
			"find", path, "-maxdepth", "1", "-printf", "%P\t%s\t%m\t%T@\t%y\n",
		}
	}

	result, err := d.ExecSimple(ctx, containerID, cmd)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("find failed: %s", strings.TrimSpace(result.Stderr))
	}

	return parseDockerFileEntries(result.Stdout, path), nil
}

func (d *DockerIO) DeletePath(ctx context.Context, containerID, path string, recursive bool) error {
	var cmd []string
	if recursive {
		cmd = []string{"rm", "-rf", path}
	} else {
		cmd = []string{"rm", "-f", path}
	}

	result, err := d.ExecSimple(ctx, containerID, cmd)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("rm failed: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func parseDockerFileEntries(output string, basePath string) []FileEntry {
	var entries []FileEntry
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Tab-separated: path\tsize\tmode\tmtime\ttype
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}

		if parts[0] == "" {
			continue
		}

		var size int64
		var mtime float64
		fmt.Sscanf(parts[1], "%d", &size)
		fmt.Sscanf(parts[3], "%f", &mtime)

		entryType := "file"
		switch parts[4] {
		case "d":
			entryType = "dir"
		case "l":
			entryType = "symlink"
		case "f":
			entryType = "file"
		}

		entries = append(entries, FileEntry{
			Path:      parts[0],
			Type:      entryType,
			Size:      size,
			Mode:      parts[2],
			MtimeUnix: int64(mtime),
		})
	}

	return entries
}
