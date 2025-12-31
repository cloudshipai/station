# PRD: Containerized Sandbox Support

**Status**: Draft  
**Author**: Station Team  
**Date**: 2025-12-31  
**Version**: 0.1

---

## Problem Statement

When Station runs inside a Docker container (via `stn up`), the sandbox system fails with:

```
Error response from daemon: invalid mount config for type "bind": 
bind source path does not exist: /tmp/station-sandboxes/sbx_7999184118503
```

### Root Cause

The `DockerBackend` creates sandboxes by:
1. Creating a workspace directory on the host: `/tmp/station-sandboxes/sbx_XXX`
2. Bind-mounting that directory into a new sandbox container

When Station runs inside a container, it uses **Docker-in-Docker** (DinD) by mounting the Docker socket (`/var/run/docker.sock`). The Docker daemon runs on the **host**, not inside the Station container.

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

## Options Analysis

### Option 1: Docker Volumes (Instead of Bind Mounts)

**Approach**: Replace bind mounts with named Docker volumes that are managed by the Docker daemon.

**Implementation**:
```go
// Instead of bind mount from host path
hostConfig := &container.HostConfig{
    Mounts: []mount.Mount{
        {
            Type:   mount.TypeBind,
            Source: workspacePath,  // Host path that doesn't exist in DinD
            Target: workdir,
        },
    },
}

// Use Docker volume
hostConfig := &container.HostConfig{
    Mounts: []mount.Mount{
        {
            Type:   mount.TypeVolume,
            Source: "station-sandbox-" + sessionID,  // Named volume
            Target: workdir,
        },
    },
}
```

**File I/O Changes**: 
- `WriteFile`, `ReadFile`, `ListFiles`, `DeleteFile` must use Docker API instead of host filesystem
- Use `docker cp` or exec `cat`/`tee` commands

**Pros**:
- Works in any Docker environment (native, DinD, remote daemon)
- No host filesystem access required
- Volumes persist across container restarts

**Cons**:
- File operations become more complex (Docker API instead of direct FS)
- Slower for large file operations
- Requires cleanup of volumes (can accumulate)

**Effort**: Medium (3-5 days)

---

### Option 2: Shared Volume Between Station and Sandboxes

**Approach**: Mount a shared volume in both Station container AND sandbox containers.

**Implementation**:
```yaml
# stn up mounts this volume
station-container:
  volumes:
    - station-sandboxes:/shared-sandboxes

# Sandbox containers also mount it
sandbox-container:
  volumes:
    - station-sandboxes:/workspace
```

```go
// DockerBackend uses the shared volume path
cfg.WorkspaceBaseDir = "/shared-sandboxes"  // Inside volume, not /tmp
```

**Pros**:
- Minimal code changes (just config path)
- Direct filesystem access works
- Fast file operations

**Cons**:
- Station container must know to mount the volume
- All sandboxes share the same volume (isolation via subdirectories)
- Requires coordination in `stn up` command

**Effort**: Low (1-2 days)

---

### Option 3: Sysbox Runtime (Nested Docker)

**Approach**: Use [Sysbox](https://github.com/nestybox/sysbox) runtime to enable true nested containers.

**Implementation**:
```bash
# Run Station container with Sysbox runtime
docker run --runtime=sysbox-runc station-server:latest
```

With Sysbox, the Station container gets its own Docker daemon, so bind mounts work normally.

**Pros**:
- No code changes required
- Full Docker functionality inside container
- Better security isolation

**Cons**:
- Requires Sysbox installation on host
- Not available on all platforms (Linux only, specific kernels)
- Additional runtime dependency
- Heavier resource usage (nested Docker daemon)

**Effort**: Low (code) / High (infrastructure requirements)

---

### Option 4: Remote Docker Host

**Approach**: Configure Station to use a remote Docker daemon for sandboxes.

**Implementation**:
```yaml
# Station config
sandbox:
  docker_host: "tcp://sandbox-docker-host:2375"
  workspace_base_dir: "/sandboxes"  # Path on REMOTE host
```

The remote Docker host has the sandbox workspace directory accessible.

**Pros**:
- Scales sandbox execution to dedicated infrastructure
- Station container stays lightweight
- Can use specialized sandbox hosts (GPU, etc.)

**Cons**:
- Requires separate Docker host infrastructure
- Network latency for Docker API calls
- More complex deployment
- Security considerations for remote Docker

**Effort**: Medium (2-3 days code + infrastructure)

---

### Option 5: exec-based File Operations (Hybrid)

**Approach**: Keep Docker volumes but use `docker exec` for all file operations instead of direct filesystem access.

**Implementation**:
```go
func (b *DockerBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error {
    // Instead of os.WriteFile on host filesystem...
    // Use docker exec to write inside the container
    cmd := []string{"tee", path}
    execResp, _ := b.client.ContainerExecCreate(ctx, session.ContainerID, container.ExecOptions{
        Cmd:         cmd,
        AttachStdin: true,
    })
    // Write content via stdin
}
```

**Pros**:
- Works with any Docker setup
- File operations happen inside sandbox container
- No shared volume coordination needed

**Cons**:
- Slower than direct FS access
- More complex implementation
- exec overhead for each file operation

**Effort**: Medium (3-4 days)

---

### Option 6: E2B / Modal / Cloud Sandboxes

**Approach**: Use a cloud sandbox provider instead of local Docker.

**Services**:
- [E2B](https://e2b.dev/) - Cloud sandboxes for AI agents
- [Modal](https://modal.com/) - Serverless container execution
- [Fly Machines](https://fly.io/docs/machines/) - On-demand VMs

**Implementation**:
```go
type E2BSandboxBackend struct {
    client *e2b.Client
}

func (b *E2BSandboxBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    sandbox, _ := b.client.Sandbox.Create(ctx, e2b.SandboxCreateParams{
        Template: "python",
    })
    return &Session{ID: sandbox.ID, ...}, nil
}
```

**Pros**:
- No Docker infrastructure needed
- Scales automatically
- Works from any environment
- Pre-built templates for common runtimes

**Cons**:
- External dependency (vendor lock-in)
- Cost per sandbox execution
- Network latency
- Requires API keys / auth

**Effort**: Medium (2-3 days for integration)

---

## Recommendation

### Short-term (Quick Fix): Option 2 - Shared Volume

This is the fastest path to get sandboxes working in `stn up`:

1. Modify `stn up` to create and mount a shared volume: `station-sandboxes`
2. Set `WorkspaceBaseDir` to `/shared-sandboxes` inside the container
3. Sandbox containers mount the same volume with a subdirectory target

**Changes Required**:
- `cmd/main/up.go`: Add volume mount for sandboxes
- `internal/services/sandbox_code_types.go`: Make `WorkspaceBaseDir` configurable
- `internal/services/sandbox_docker_backend.go`: Detect containerized mode

### Medium-term: Option 1 - Docker Volumes

For a more robust solution that works in all scenarios:

1. Create named Docker volumes for each sandbox session
2. Implement Docker API-based file operations
3. Add volume cleanup on session destroy

### Long-term: Option 6 - Cloud Sandboxes

For production CloudShip deployments:

1. Integrate E2B or similar for cloud-hosted sandboxes
2. Keep local Docker as fallback for self-hosted
3. Add sandbox provider abstraction layer

---

## Implementation Plan

### Phase 1: Shared Volume (Quick Fix)

**File**: `cmd/main/up.go`

```go
// Add to container creation
dockerArgs = append(dockerArgs,
    "-v", "station-sandboxes:/shared-sandboxes",
)
```

**File**: `internal/services/sandbox_code_types.go`

```go
// Make WorkspaceBaseDir environment-aware
func DefaultCodeModeConfig() CodeModeConfig {
    baseDir := os.Getenv("STATION_SANDBOX_DIR")
    if baseDir == "" {
        baseDir = "/tmp/station-sandboxes"
    }
    return CodeModeConfig{
        WorkspaceBaseDir: baseDir,
        // ...
    }
}
```

**File**: `internal/services/sandbox_docker_backend.go`

```go
// Update CreateSession to handle shared volume
func (b *DockerBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    // ... existing code ...
    
    hostConfig := &container.HostConfig{
        Mounts: []mount.Mount{
            {
                Type:   mount.TypeVolume,  // Use TypeVolume instead of TypeBind
                Source: "station-sandboxes",
                Target: "/shared-sandboxes",
            },
        },
    }
    
    // Session workspace is a subdirectory within the shared volume
    workspacePath := filepath.Join("/shared-sandboxes", sessionID)
}
```

### Phase 2: Docker Volume Backend

Create `sandbox_volume_backend.go` that uses named volumes and Docker API for file ops.

### Phase 3: Cloud Provider Abstraction

Create `sandbox_cloud_backend.go` with E2B integration and provider interface.

---

## Success Criteria

- [ ] `stn up default --dev` can run agents with sandbox tools
- [ ] `sandbox_open`, `sandbox_exec`, `sandbox_fs_*` tools work in containerized mode
- [ ] File staging (`sandbox_stage_file`, `sandbox_publish_file`) works
- [ ] Sandbox containers are properly cleaned up
- [ ] Performance is acceptable (<500ms for file operations)

---

## Testing

```bash
# Test containerized sandbox
stn up default --dev
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "coder", "task": "Write a hello.py and run it"}'

# Verify sandbox was created and destroyed
docker ps -a | grep sbx_
docker volume ls | grep station-sandbox
```

---

## References

- [Docker Volumes Documentation](https://docs.docker.com/storage/volumes/)
- [Docker-in-Docker Best Practices](https://jpetazzo.github.io/2015/09/03/do-not-use-docker-in-docker-for-ci/)
- [Sysbox Runtime](https://github.com/nestybox/sysbox)
- [E2B Sandbox SDK](https://e2b.dev/docs)
