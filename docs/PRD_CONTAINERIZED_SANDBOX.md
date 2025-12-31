# PRD: Containerized Sandbox Support

**Status**: In Progress  
**Author**: Station Team  
**Date**: 2025-12-31  
**Version**: 1.0

---

## Problem Statement

When Station runs inside a Docker container (via `stn up`), the sandbox system fails with:

```
Error response from daemon: invalid mount config for type "bind": 
bind source path does not exist: /tmp/station-sandboxes/sbx_7999184118503
```

### Root Cause

The `DockerBackend` creates sandboxes using **bind mounts**:

```go
hostConfig := &container.HostConfig{
    Mounts: []mount.Mount{
        {
            Type:   mount.TypeBind,
            Source: workspacePath,  // /tmp/station-sandboxes/sbx_XXX
            Target: workdir,
        },
    },
}
```

When Station runs inside a container with Docker-in-Docker (socket mounted):
- Station creates `/tmp/station-sandboxes/sbx_XXX` inside its container
- Docker daemon (on host) looks for the path on the **host filesystem**
- Path doesn't exist on host → mount fails

```
┌─────────────────────────────────────────────────────────────┐
│                     DOCKER HOST                              │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │          Station Container (stn up)                   │   │
│  │                                                       │   │
│  │   /tmp/station-sandboxes/sbx_XXX  ← EXISTS HERE      │   │
│  │                                                       │   │
│  │   Docker Client ──────────────────────────────────────│───│─┐
│  └──────────────────────────────────────────────────────┘   │ │
│                                                              │ │
│  /tmp/station-sandboxes/sbx_XXX  ← DOES NOT EXIST HERE      │ │
│                                                              │ │
│  Docker Daemon ◄─────────────────────────────────────────────│─┘
│      │                                                       │
│      └─► Creates sandbox container with bind mount           │
│          Source: /tmp/station-sandboxes/sbx_XXX (HOST PATH)  │
│          FAILS: Path doesn't exist on host!                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Target Environments

The solution must work across all these deployment scenarios:

| Environment | Docker Available | Current Status | Target |
|-------------|------------------|----------------|--------|
| Linux native (`stn serve`) | ✅ | ✅ Works | ✅ |
| macOS native (`stn serve`) | ✅ | ✅ Works | ✅ |
| `stn up` (Docker-in-Docker) | ✅ | ❌ Fails | ✅ |
| EC2 / VM | ✅ | ✅ Works | ✅ |
| ECS Fargate | ❌ | ❌ N/A | ⚠️ Cloud fallback |
| Kubernetes | ❌* | ❌ N/A | ⚠️ Cloud fallback |
| Fly.io | ❌ | ❌ N/A | ⚠️ Cloud fallback |

*Kubernetes typically uses containerd, not Docker.

---

## Solution: Docker API File Operations

### Core Insight

The problem is **not** Docker containers themselves—it's that we use **bind mounts** which require shared filesystem access. The fix: **use Docker API for all file operations** instead of host filesystem.

### Current vs New Architecture

```
CURRENT (breaks in DinD):
┌─────────────┐                      ┌─────────────┐
│   Station   │                      │   Sandbox   │
│             │  bind mount          │  Container  │
│ /tmp/sbx_X/ │◄────────────────────►│ /workspace/ │
│  (host FS)  │                      │             │
└─────────────┘                      └─────────────┘
     │
     └── os.WriteFile(), os.ReadFile() ← Direct FS access

NEW (works everywhere):
┌─────────────┐                      ┌─────────────┐
│   Station   │     Docker API       │   Sandbox   │
│             │◄────────────────────►│  Container  │
│             │  CopyToContainer()   │ /workspace/ │
│             │  CopyFromContainer() │             │
└─────────────┘  ContainerExec()     └─────────────┘
                     │
                     └── No bind mount, no shared FS
```

### File Operation Changes

| Operation | Current Implementation | New Implementation |
|-----------|----------------------|-------------------|
| `WriteFile` | `os.WriteFile(hostPath)` | `docker cp` via `CopyToContainer()` |
| `ReadFile` | `os.ReadFile(hostPath)` | `docker cp` via `CopyFromContainer()` |
| `ListFiles` | `os.ReadDir(hostPath)` | `docker exec ls -la` or `find` |
| `DeleteFile` | `os.Remove(hostPath)` | `docker exec rm` |
| Container Create | Bind mount workspace | No mount, empty `/workspace` |
| Cleanup | `os.RemoveAll` + container remove | Just container remove |

---

## Integration with NATS File Staging

The sandbox system already has NATS Object Store integration for file staging (see `PRD_SANDBOX_FILE_STAGING.md`):

```
┌──────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   User   │────►│  NATS OS    │────►│   Station   │────►│   Sandbox   │
│          │     │ (staging)   │     │             │     │  Container  │
│ upload   │     │             │     │ stage_file  │     │             │
│ download │◄────│ files/f_XXX │◄────│ publish_file│◄────│ /workspace/ │
└──────────┘     └─────────────┘     └─────────────┘     └─────────────┘
```

### Tools Integration

| Tool | Source | Destination | Transport |
|------|--------|-------------|-----------|
| `sandbox_stage_file` | NATS Object Store | Sandbox `/workspace` | Docker API |
| `sandbox_publish_file` | Sandbox `/workspace` | NATS Object Store | Docker API |
| `sandbox_fs_write` | Tool input (LLM) | Sandbox `/workspace` | Docker API |
| `sandbox_fs_read` | Sandbox `/workspace` | Tool output (LLM) | Docker API |

---

## Implementation Plan

### Phase 1: Docker API File Operations

Create helper functions for Docker-based file I/O:

**File**: `internal/services/sandbox_docker_io.go`

```go
package services

import (
    "archive/tar"
    "bytes"
    "context"
    "io"
    "path/filepath"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
)

// CopyToContainer writes content to a path inside a container using docker cp
func CopyToContainer(ctx context.Context, cli *client.Client, containerID, destPath string, content []byte, mode int64) error {
    // Create tar archive with single file
    var buf bytes.Buffer
    tw := tar.NewWriter(&buf)
    
    header := &tar.Header{
        Name: filepath.Base(destPath),
        Mode: mode,
        Size: int64(len(content)),
    }
    tw.WriteHeader(header)
    tw.Write(content)
    tw.Close()
    
    // Copy to container
    return cli.CopyToContainer(ctx, containerID, filepath.Dir(destPath), &buf, types.CopyToContainerOptions{})
}

// CopyFromContainer reads content from a path inside a container using docker cp
func CopyFromContainer(ctx context.Context, cli *client.Client, containerID, srcPath string) ([]byte, error) {
    reader, _, err := cli.CopyFromContainer(ctx, containerID, srcPath)
    if err != nil {
        return nil, err
    }
    defer reader.Close()
    
    // Extract from tar
    tr := tar.NewReader(reader)
    _, err = tr.Next()
    if err != nil {
        return nil, err
    }
    
    return io.ReadAll(tr)
}

// ExecInContainer runs a command and returns output
func ExecInContainer(ctx context.Context, cli *client.Client, containerID string, cmd []string) (string, string, int, error) {
    execConfig := container.ExecOptions{
        Cmd:          cmd,
        AttachStdout: true,
        AttachStderr: true,
    }
    
    execResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
    if err != nil {
        return "", "", -1, err
    }
    
    attachResp, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
    if err != nil {
        return "", "", -1, err
    }
    defer attachResp.Close()
    
    // Read output...
    // Return stdout, stderr, exitCode, nil
}
```

### Phase 2: Update DockerBackend

**File**: `internal/services/sandbox_docker_backend.go`

1. **Remove bind mount from `CreateSession`**:

```go
func (b *DockerBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    // ... validation ...
    
    sessionID := fmt.Sprintf("sbx_%s", generateShortID())
    
    // NO workspace directory creation on host
    // NO bind mount
    
    containerConfig := &container.Config{
        Image:      opts.Image,
        WorkingDir: "/workspace",  // Always /workspace inside container
        Env:        env,
        Cmd:        []string{"sleep", "infinity"},
    }
    
    hostConfig := &container.HostConfig{
        // NO Mounts - empty workspace created by Dockerfile or mkdir
        AutoRemove:  false,
        NetworkMode: "none",
    }
    
    // Create and start container
    resp, err := b.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, sessionID)
    // ...
    
    // Initialize /workspace directory inside container
    b.execSimple(ctx, resp.ID, []string{"mkdir", "-p", "/workspace"})
    
    return &Session{
        ID:          sessionID,
        ContainerID: resp.ID,
        Workdir:     "/workspace",
        // NO WorkspacePath - we don't use host filesystem
    }, nil
}
```

2. **Replace filesystem operations**:

```go
func (b *DockerBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error {
    session, err := b.GetSession(ctx, sessionID)
    if err != nil {
        return err
    }
    
    path = normalizeWorkspacePath(path)
    destPath := filepath.Join("/workspace", path)
    
    // Create parent directory
    dir := filepath.Dir(destPath)
    if dir != "/workspace" {
        b.execSimple(ctx, session.ContainerID, []string{"mkdir", "-p", dir})
    }
    
    // Use Docker API to copy file into container
    return CopyToContainer(ctx, b.client, session.ContainerID, destPath, content, int64(mode))
}

func (b *DockerBackend) ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, bool, error) {
    session, err := b.GetSession(ctx, sessionID)
    if err != nil {
        return nil, false, err
    }
    
    path = normalizeWorkspacePath(path)
    srcPath := filepath.Join("/workspace", path)
    
    // Use Docker API to copy file from container
    content, err := CopyFromContainer(ctx, b.client, session.ContainerID, srcPath)
    if err != nil {
        return nil, false, err
    }
    
    // Truncate if needed
    truncated := len(content) > maxBytes
    if truncated {
        content = content[:maxBytes]
    }
    
    return content, truncated, nil
}

func (b *DockerBackend) ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error) {
    session, err := b.GetSession(ctx, sessionID)
    if err != nil {
        return nil, err
    }
    
    path = normalizeWorkspacePath(path)
    targetPath := filepath.Join("/workspace", path)
    
    // Use find command for listing
    var cmd []string
    if recursive {
        cmd = []string{"find", targetPath, "-printf", "%P\t%s\t%m\t%T@\t%y\n"}
    } else {
        cmd = []string{"find", targetPath, "-maxdepth", "1", "-printf", "%P\t%s\t%m\t%T@\t%y\n"}
    }
    
    stdout, _, exitCode, err := ExecInContainer(ctx, b.client, session.ContainerID, cmd)
    if err != nil || exitCode != 0 {
        return nil, err
    }
    
    // Parse find output into []FileEntry
    return parseFileEntries(stdout), nil
}

func (b *DockerBackend) DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error {
    session, err := b.GetSession(ctx, sessionID)
    if err != nil {
        return err
    }
    
    path = normalizeWorkspacePath(path)
    targetPath := filepath.Join("/workspace", path)
    
    cmd := []string{"rm"}
    if recursive {
        cmd = append(cmd, "-rf")
    }
    cmd = append(cmd, targetPath)
    
    _, _, exitCode, err := ExecInContainer(ctx, b.client, session.ContainerID, cmd)
    if err != nil || exitCode != 0 {
        return fmt.Errorf("delete failed: exit code %d", exitCode)
    }
    
    return nil
}
```

3. **Simplify cleanup**:

```go
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
    
    // NO os.RemoveAll - no host filesystem to clean up
    
    return nil
}
```

### Phase 3: Configuration

Add detection for containerized mode:

**File**: `internal/services/sandbox_code_types.go`

```go
type CodeModeConfig struct {
    Enabled          bool
    // REMOVED: WorkspaceBaseDir - no longer needed
    UseDockerAPI     bool          // Always true now, kept for backwards compat
    DefaultTimeout   time.Duration
    IdleTimeout      time.Duration
    CleanupInterval  time.Duration
    MaxStdoutBytes   int
    AllowedImages    []string
    DockerHost       string
}

func DefaultCodeModeConfig() CodeModeConfig {
    return CodeModeConfig{
        Enabled:         true,
        UseDockerAPI:    true,  // Default to Docker API for universal support
        DefaultTimeout:  5 * time.Minute,
        IdleTimeout:     30 * time.Minute,
        CleanupInterval: 5 * time.Minute,
        MaxStdoutBytes:  1024 * 1024, // 1MB
        AllowedImages:   []string{"python:3.11-slim", "node:20-slim", "ubuntu:22.04"},
    }
}
```

### Phase 4: Testing

**File**: `internal/services/sandbox_docker_backend_test.go`

```go
func TestDockerBackend_WriteReadFile_NoBindMount(t *testing.T) {
    backend := setupTestBackend(t)
    
    session, err := backend.CreateSession(ctx, SessionOptions{
        Image: "python:3.11-slim",
    })
    require.NoError(t, err)
    defer backend.DestroySession(ctx, session.ID)
    
    // Write file using Docker API
    content := []byte("hello world")
    err = backend.WriteFile(ctx, session.ID, "test.txt", content, 0644)
    require.NoError(t, err)
    
    // Read file using Docker API
    data, truncated, err := backend.ReadFile(ctx, session.ID, "test.txt", 1024)
    require.NoError(t, err)
    assert.False(t, truncated)
    assert.Equal(t, content, data)
    
    // Verify via exec
    result, err := backend.Exec(ctx, session.ID, ExecRequest{
        Cmd: []string{"cat", "/workspace/test.txt"},
    })
    require.NoError(t, err)
    assert.Equal(t, "hello world", result.Stdout)
}

func TestDockerBackend_ListFiles_NoBindMount(t *testing.T) {
    // Test ListFiles works via docker exec
}

func TestDockerBackend_DeleteFile_NoBindMount(t *testing.T) {
    // Test DeleteFile works via docker exec
}

func TestDockerBackend_LargeFile_NoBindMount(t *testing.T) {
    // Test multi-MB file transfer via Docker API
}
```

---

## Migration Path

### Backwards Compatibility

The change is transparent to users:
- Same tool interface (`sandbox_fs_write`, `sandbox_fs_read`, etc.)
- Same behavior from agent perspective
- Works in more environments

### Performance Considerations

| Operation | Bind Mount | Docker API | Notes |
|-----------|------------|------------|-------|
| Write 1KB | ~0.1ms | ~5ms | Tar creation + API call |
| Write 1MB | ~1ms | ~50ms | Acceptable for most use cases |
| Write 100MB | ~50ms | ~2s | Large files should use NATS staging |
| Read 1KB | ~0.1ms | ~5ms | Similar to write |
| Exec | Same | Same | No change |

For large files, use the NATS file staging system (`sandbox_stage_file` / `sandbox_publish_file`).

---

## Future Enhancements

### Cloud Sandbox Fallback

For environments without Docker (ECS Fargate, K8s, Fly.io):

```go
type SandboxBackend interface {
    CreateSession(ctx context.Context, opts SessionOptions) (*Session, error)
    Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error)
    WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error
    ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, bool, error)
    ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error)
    DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error
    DestroySession(ctx context.Context, sessionID string) error
}

// Implementations:
type DockerAPIBackend struct { ... }  // Uses Docker API (this PRD)
type E2BBackend struct { ... }        // Uses E2B cloud sandboxes
type ModalBackend struct { ... }      // Uses Modal.com
```

Auto-detection:
```go
func NewSandboxBackend(cfg Config) (SandboxBackend, error) {
    if dockerAvailable() {
        return NewDockerAPIBackend(cfg)
    }
    if cfg.E2BAPIKey != "" {
        return NewE2BBackend(cfg.E2BAPIKey)
    }
    return nil, errors.New("no sandbox backend available")
}
```

---

## Success Criteria

- [ ] `sandbox_open` works in `stn up` containerized mode
- [ ] `sandbox_exec` works in `stn up` containerized mode
- [ ] `sandbox_fs_write` / `sandbox_fs_read` work in `stn up` mode
- [ ] `sandbox_stage_file` / `sandbox_publish_file` work end-to-end
- [ ] All existing sandbox tests pass
- [ ] New tests for Docker API file operations
- [ ] Performance acceptable (<100ms for 1MB file operations)

---

## Testing Commands

```bash
# Rebuild and test containerized
cd station
make rebuild-all
stn down && stn up default --dev

# Test agent execution with sandbox
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "coder", "task": "Create a hello.py file that prints hello world, then run it"}'

# Check logs
stn logs -f

# Verify sandbox was created
docker ps -a | grep sbx_
```

---

## References

- [Docker SDK for Go - CopyToContainer](https://pkg.go.dev/github.com/docker/docker/client#Client.CopyToContainer)
- [Docker SDK for Go - CopyFromContainer](https://pkg.go.dev/github.com/docker/docker/client#Client.CopyFromContainer)
- [PRD: Sandbox File Staging](./PRD_SANDBOX_FILE_STAGING.md)
- [Station Sandbox Architecture](./station/sandbox-architecture.md)
