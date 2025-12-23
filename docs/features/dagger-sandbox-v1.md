# PRD: CloudShip Station - Dagger Sandbox (Isolated Compute for Agents)

> **Status**: Draft  
> **Created**: 2025-12-23  
> **Layer**: Primitives (Foundation)

## 1) Overview

### Architecture Context

The Sandbox is at the **primitives layer** of Station's architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                    WORKFLOW LAYER (Highest)                      │
│  - Orchestrates agents via agent.run executor                    │
│  - Does NOT call sandbox directly                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                 AGENTS-AS-TOOLS LAYER (Middle)                   │
│  - Agents can use sandbox_run tool for compute                   │
│  - Agents can call other agents as tools                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   PRIMITIVES LAYER (Foundation)                  │
│  - MCP Servers (tools, secrets)                                 │
│  - Agents (input/output schemas, dotprompt)                      │
│  - Sandbox (Dagger compute) ← THIS PRD                          │
└─────────────────────────────────────────────────────────────────┘
```

### Problem Statement

Station agents frequently need to **transform, correlate, and compute over data** (logs, JSON/YAML, metrics, CSVs, Kubernetes manifests, etc.). Today, agents can:
- Reason in the LLM (slow/expensive for large inputs, error-prone for deterministic transforms)
- Call MCP tools (can be network-latent, timeout-prone, not ideal for long-running compute)
- Use host tools (unsafe: execution can modify host filesystem, leak secrets)

We need a **safe, isolated execution environment** that agents can use for deterministic compute (Python/Node/Bash) without affecting the host.

### Goals (V1)

1. Provide a **Dagger-based sandbox** for running agent-requested code in isolated containers.
2. Expose sandbox execution as a **GenKit native tool** (not MCP) to avoid timeout issues.
3. Allow agents to **opt-in via dotprompt frontmatter** (`sandbox: python`).
4. Support **multiple runtimes**: Python (primary), Node.js, Bash.
5. Ensure **observability parity** with other Station execution paths (Jaeger/OTEL traces, structured logs).
6. Work in containerized deployments: **Docker Compose**, **ECS**, **Cloud Run/GKE**, **Fly.io**.

### Non-goals (V1)

- Full "remote notebook" experience (interactive REPL, step-debugging).
- Arbitrary privileged container execution (no Docker-in-Docker inside sandbox).
- Guaranteed hermetic builds with pinned dependency resolution.
- Multi-node distributed execution.

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Tool type | **GenKit native tool** | Avoids MCP timeouts, enables long-running execution |
| Opt-in | **Frontmatter** (`sandbox: python`) | One-line enablement, follows dotprompt patterns |
| Engine | **Dagger** | Already used in Station/Ship, container-based isolation |

---

## 2) User Stories

### Primary Personas

- **DevOps/SRE operators** using Station agents to diagnose incidents
- **Platform engineers** building/operating Station deployments
- **Agent authors** writing dotprompt files and wanting safe compute

### Stories

1. **As an agent author**, I want to enable a sandbox for my agent so it can run Python to parse large JSON and return summarized results.

2. **As an SRE**, I want the agent to compute correlations (e.g., join two datasets, compute top-N offenders) without the LLM hallucinating math.

3. **As a platform engineer**, I need sandbox runs to be traceable in Jaeger with clear spans, durations, and exit codes.

4. **As a security-conscious operator**, I need sandbox runs to have resource limits, filesystem restrictions, and minimal/no network by default.

5. **As an operator on ECS/Cloud Run**, I need a supported deployment mode (socket mount vs remote engine) that works without privileged containers.

---

## 3) Technical Design

### 3.1 Architecture: SandboxService

**Components**:
- **SandboxService (Go)**: Connects to Dagger engine, creates containers, injects code, executes, captures output
- **SandboxToolFactory**: Builds GenKit tool (`ai.Tool`) with agent-specific defaults from frontmatter

**Package layout**:
```
station/internal/services/
├── sandbox_service.go       # Core service
├── sandbox_config.go        # Configuration
├── sandbox_tool.go          # GenKit tool factory
└── sandbox_policy.go        # Image allowlist, limits
```

### 3.2 GenKit Tool Definition

**Tool name**: `sandbox_run`

**Why GenKit native tool (not MCP)**:
- No MCP server hop
- No MCP protocol timeouts
- Full control over execution, logging, retries

**Tool registration**:
```go
func NewSandboxRunTool(svc *SandboxService, defaults SandboxDefaults) ai.Tool {
    schema := map[string]any{
        "type": "object",
        "properties": map[string]any{
            "runtime": map[string]any{
                "type": "string",
                "enum": []string{"python", "node", "bash"},
            },
            "code": map[string]any{
                "type": "string",
                "description": "Source code to run in the sandbox.",
            },
            "args": map[string]any{
                "type": "array",
                "items": map[string]any{"type": "string"},
            },
            "env": map[string]any{
                "type": "object",
                "additionalProperties": map[string]any{"type": "string"},
            },
            "files": map[string]any{
                "type": "object",
                "additionalProperties": map[string]any{"type": "string"},
                "description": "Map of path -> file contents to create in /work",
            },
            "timeout_seconds": map[string]any{
                "type": "integer",
                "minimum": 1,
                "maximum": 3600,
            },
        },
        "required": []string{"code"},
    }

    return ai.NewToolWithInputSchema(
        "sandbox_run",
        "Execute code in an isolated Dagger container sandbox.",
        schema,
        func(ctx context.Context, input map[string]any) (any, error) {
            req := ParseSandboxRunRequest(input, defaults)
            return svc.Run(ctx, req)
        },
    )
}
```

### 3.3 Frontmatter Integration

Agents opt-in to sandbox via frontmatter:

**Simple form (string)**:
```yaml
---
model: openai/gpt-4o
metadata:
  name: "Data Analyst"
sandbox: python
---
```

**Structured form (object)**:
```yaml
---
model: openai/gpt-4o
metadata:
  name: "Data Correlator"
sandbox:
  runtime: python
  image: "python:3.11-slim"
  timeout_seconds: 300
  max_stdout_bytes: 200000
  allow_network: false
  pip_packages:
    - pandas
    - pyyaml
---
```

**Go struct** (in `pkg/dotprompt/types.go`):
```go
type SandboxFrontmatter struct {
    Runtime        string   `yaml:"runtime,omitempty"`
    Image          string   `yaml:"image,omitempty"`
    TimeoutSeconds int      `yaml:"timeout_seconds,omitempty"`
    AllowNetwork   bool     `yaml:"allow_network,omitempty"`
    PipPackages    []string `yaml:"pip_packages,omitempty"`
    NpmPackages    []string `yaml:"npm_packages,omitempty"`
}
```

**Behavior**: If `sandbox` is present in frontmatter, Station appends `sandbox_run` tool to the agent's tool list.

### 3.4 Input/Output Schemas

**Tool input** (`SandboxRunRequest`):
| Field | Type | Description |
|-------|------|-------------|
| `runtime` | string | python, node, bash (default from frontmatter) |
| `code` | string | Source code to execute (required) |
| `args` | []string | Command-line arguments |
| `env` | map | Environment variables |
| `files` | map | Files to create in /work |
| `timeout_seconds` | int | Execution timeout |

**Tool output**:
```json
{
  "ok": true,
  "runtime": "python",
  "exit_code": 0,
  "duration_ms": 1842,
  "stdout": "top offenders: ...",
  "stderr": "",
  "artifacts": [
    {
      "path": "output/result.json",
      "size_bytes": 12034,
      "content_base64": "eyJrZXkiOiAiLi4uIn0="
    }
  ],
  "limits": {
    "timeout_seconds": 300,
    "max_stdout_bytes": 200000
  }
}
```

**Caps** (security + stability):
- `stdout` and `stderr` truncated at configurable byte limits
- `artifacts` returned only if size <= cap

### 3.5 Dagger Container Lifecycle

**Per sandbox tool call**:

1. `dagger.Connect(ctx, ...)` via shared client in SandboxService
2. Select runtime base image:
   - Python: `python:3.11-slim`
   - Node: `node:20-slim`
   - Bash: `ubuntu:22.04`
3. Create working directory (`/work`)
4. Inject files:
   - `WithNewFile("/work/main.py", code)`
   - For `files` map: `WithNewFile("/work/<path>", contents)`
5. Execute command:
   - Python: `python /work/main.py <args>`
   - Node: `node /work/main.js <args>`
   - Bash: `bash /work/main.sh <args>`
6. Capture stdout/stderr
7. Export artifacts if requested

**Example (Python)**:
```go
ctr := client.Container().From(image).
    WithWorkdir("/work").
    WithNewFile("/work/main.py", req.Code)

for path, contents := range req.Files {
    ctr = ctr.WithNewFile("/work/" + path, contents)
}

execArgs := append([]string{"python", "/work/main.py"}, req.Args...)
ctr = ctr.WithExec(execArgs)

stdout, _ := ctr.Stdout(ctx)
```

---

## 4) Deployment Considerations

Dagger clients talk to a **Dagger Engine** (BuildKit-based). Deployment mode depends on platform.

### Mode 1: Docker Socket Mount

**How**: Mount `/var/run/docker.sock:/var/run/docker.sock`

**Pros**: Simple, local performance, caching works well  
**Cons**: Docker socket is root-equivalent; must be protected

**Recommended for**: Docker Compose, ECS on EC2, Fly.io

### Mode 2: Engine Sidecar Container

**How**: Run `dagger-engine` as sidecar, connect via `DAGGER_HOST=tcp://dagger-engine:PORT`

**Pros**: Better isolation than raw socket mount  
**Cons**: Requires explicit networking/resource allocation

**Recommended for**: GKE, ECS Fargate (if supported)

### Mode 3: Dagger Cloud / Remote Engine

**How**: Connect to remote Dagger engine endpoint

**Pros**: Works where no Docker daemon available (Cloud Run)  
**Cons**: Needs credentials, network egress, latency

**Recommended for**: Cloud Run, restricted environments

### Platform Matrix

| Platform | Recommended Mode | Notes |
|----------|-----------------|-------|
| Docker Compose | Mode 1 (socket) or Mode 2 (sidecar) | Document security implications |
| ECS on EC2 | Mode 1 (socket) | If allowed by security posture |
| ECS Fargate | Mode 3 (remote) | No socket access |
| GKE | Mode 2 (sidecar) | With NetworkPolicy |
| Cloud Run | Mode 3 (remote) | No socket, limited privileges |
| Fly.io | Mode 1 or 3 | Depends on Fly setup |

### Configuration

```bash
STATION_SANDBOX_ENABLED=true
STATION_SANDBOX_ENGINE_MODE=docker_socket|sidecar|remote
STATION_SANDBOX_ALLOWED_IMAGES=python:3.11-slim,node:20-slim,ubuntu:22.04
STATION_SANDBOX_DEFAULT_TIMEOUT_SECONDS=120
STATION_SANDBOX_MAX_STDOUT_BYTES=200000
STATION_SANDBOX_MAX_STDERR_BYTES=200000
STATION_SANDBOX_MAX_ARTIFACT_BYTES=10000000
STATION_SANDBOX_ALLOW_NETWORK_DEFAULT=false
```

---

## 5) Observability

### 5.1 Tracing (Jaeger/OTEL)

Sandbox executions appear as **tool spans** in GenKit traces:

```
AgentExecute
└── sandbox_run (tool span)
    ├── sandbox.connect
    ├── sandbox.prepare_files
    ├── sandbox.exec
    ├── sandbox.collect_outputs
    └── sandbox.collect_artifacts
```

**Span attributes**:
- `sandbox.runtime`: python|node|bash
- `sandbox.image`
- `sandbox.timeout_seconds`
- `sandbox.exit_code`
- `sandbox.stdout_bytes`, `sandbox.stderr_bytes`
- `sandbox.artifacts_count`
- `error` and status if exec fails

### 5.2 Logging

Structured log events:
- `sandbox_run_started`: run correlation ID, runtime, image
- `sandbox_run_finished`: duration, exit code, output sizes

### 5.3 Metrics (V1 minimal)

| Metric | Type | Labels |
|--------|------|--------|
| `station_sandbox_runs_total` | Counter | runtime, ok |
| `station_sandbox_run_duration_ms` | Histogram | runtime |

---

## 6) Security

### 6.1 Resource Limits

**V1 enforcement**:
- Hard timeout via `context.WithTimeout(ctx, timeout)`
- Output caps: `max_stdout_bytes`, `max_stderr_bytes`
- Artifact caps: `max_artifact_bytes`, `max_artifacts_count`

**Platform-level**: CPU/memory limits on Station container and dagger engine container.

### 6.2 Network Isolation

**Goal**: Sandbox should default to **no network**.

**Implementation**:
- Deployment-level network policies (Kubernetes NetworkPolicy, ECS security groups)
- Tool-level policy: `allow_network` default false
- If network required, must be explicitly enabled in frontmatter + admin config

### 6.3 Filesystem Restrictions

- No host filesystem mounts by default
- Only inject: code, explicitly provided files
- No access to Station secrets (no env pass-through except allowlist)

### 6.4 Image Policy

- Default images pinned and curated
- Allowlist config: `STATION_SANDBOX_ALLOWED_IMAGES`
- Non-allowed images → fail fast with clear error

---

## 7) API Design

### 7.1 Tool Schema (to LLM)

**Tool**: `sandbox_run`

**Inputs**:
- `runtime` (optional): `"python" | "node" | "bash"`
- `code` (required): string
- `args` (optional): string[]
- `env` (optional): object
- `files` (optional): object (path → content)
- `timeout_seconds` (optional): integer

**Outputs**:
- `ok`: boolean
- `exit_code`: integer
- `stdout`: string (truncated)
- `stderr`: string (truncated)
- `duration_ms`: integer
- `artifacts`: list (optional)
- `error`: string (if tool-level failure)

### 7.2 Frontmatter Options

**Minimal**:
```yaml
sandbox: python
```

**Advanced**:
```yaml
sandbox:
  runtime: python
  image: python:3.11-slim
  timeout_seconds: 300
  allow_network: false
  pip_packages: ["pandas", "pyyaml"]
```

---

## 8) Implementation Plan

### Phase 0 - Design + Scaffolding (1-4h) ✅ COMPLETE

- [x] Add `SandboxService` skeleton with config parsing
- [x] Add tool factory skeleton returning GenKit tool
- [x] Add frontmatter parsing (`sandbox: string|object`)

**Deliverables**:
- `internal/services/sandbox_service.go` - SandboxService with config, Run(), MergeDefaults()
- `internal/services/sandbox_tool.go` - SandboxToolFactory with GenKit tool creation
- `pkg/dotprompt/types.go` - SandboxConfig struct with custom UnmarshalYAML
- `pkg/dotprompt/types_test.go` - Tests for frontmatter parsing (string and object forms)

### Phase 1 - Python MVP (1-2d) 🔄 IN PROGRESS

- [x] Implement `sandbox_run` for Python runtime (`executeInDagger()` with full Dagger SDK)
- [x] Inject code and files (via `WithNewFile`)
- [x] Execute and capture stdout/stderr
- [x] Enforce timeout and output caps (truncation helper)
- [x] Integrate into `GenKitExecutor.ExecuteAgent` flow
  - [x] Add `sandboxService` and `sandboxToolFactory` to `AgentExecutionEngine`
  - [x] Add `parseSandboxConfigFromAgent()` to parse dotprompt frontmatter
  - [x] Conditionally inject `sandbox_run` tool when frontmatter has `sandbox:` config
- [ ] Add tests (unit + integration)

**Files implemented**:
- `internal/services/sandbox_service.go` - Full `executeInDagger()` with Dagger Go SDK
- `internal/services/sandbox_tool.go` - GenKit tool factory with schema
- `internal/services/agent_execution_engine.go` - Integration with frontmatter parsing

### Phase 2 - Node + Bash (1-2d)

- [ ] Add runtime mapping for Node and Bash
- [ ] Add examples in prompt templates
- [ ] Documentation

### Phase 3 - Deployment Modes + Docs (1-2d)

- [ ] Document Docker Compose (socket) pattern
- [ ] Document Engine sidecar pattern
- [ ] Cloud Run guidance (remote engine)
- [ ] Configuration validation at startup

### Phase 4 - Observability Hardening (1-4h)

- [ ] Add spans for sandbox internals
- [ ] Ensure tool outputs in traces (truncated)
- [ ] Add metrics

---

## 9) Success Metrics

### Functional

- 95%+ of sandbox executions complete successfully for supported scripts under timeout
- Agents can process "too big for LLM" datasets (5-50MB JSON) using sandbox

### Operational

- 100% of sandbox tool calls produce GenKit tool span with duration/exit code
- No incidents where sandbox modifies host filesystem or reads host secrets

### Adoption

- Measurable increase in "tool-assisted deterministic transforms" vs LLM-only reasoning

---

## 10) Open Questions

1. **Engine strategy for Cloud Run**: Is Dagger Cloud the officially supported path? Credential management?

2. **Network isolation guarantees**: Require deployment-level enforcement (NetworkPolicy) to claim "isolated by default"?

3. **Artifact handling**: Store on Station disk (ephemeral), in DB, or upload to object storage?

4. **Timeout coordination**: Station's GenKitExecutor has 5-minute timeout. Increase when sandbox enabled?

5. **Dependency installs**: Allow `pip install` (requires network) or provide curated images?

---

## Appendix: Example Agent Using Sandbox

```yaml
---
model: openai/gpt-4o
metadata:
  name: "Log Analyzer"
  description: "Analyzes log files and computes statistics"
input:
  schema:
    type: object
    properties:
      log_data:
        type: string
        description: "Raw log data to analyze"
    required: [log_data]
output:
  schema:
    type: object
    properties:
      total_lines:
        type: integer
      error_count:
        type: integer
      top_errors:
        type: array
        items:
          type: object
          properties:
            message: { type: string }
            count: { type: integer }
sandbox: python
---

You are a log analysis agent. When given log data, use the sandbox_run tool
to execute Python code that parses and analyzes the logs.

Example sandbox usage:
```python
import json
import sys
from collections import Counter

# Read log data from stdin or file
log_data = """{{ log_data }}"""

lines = log_data.strip().split('\n')
errors = [l for l in lines if 'ERROR' in l]
error_counts = Counter(errors)

result = {
    "total_lines": len(lines),
    "error_count": len(errors),
    "top_errors": [
        {"message": msg, "count": count}
        for msg, count in error_counts.most_common(5)
    ]
}

print(json.dumps(result))
```

Return the analysis results in structured format.
```

---

*Created: 2025-12-23*
