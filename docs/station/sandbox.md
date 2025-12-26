# Sandbox: Isolated Code Execution for Agents

Station provides sandboxed environments that allow agents to execute code in isolated containers. This enables agents to perform deterministic computations, data transformations, and complex analysis without affecting the host system.

## Two Modes

| Mode | Backend | Lifecycle | Use Case |
|------|---------|-----------|----------|
| **Compute** (default) | Dagger | Ephemeral per-call | Quick calculations, data processing |
| **Code** | Docker | Persistent per-workflow | Full Linux environment, iterative development |

## Why Use Sandbox?

| Use Case | Without Sandbox | With Sandbox |
|----------|-----------------|--------------|
| Parse large JSON/CSV | LLM processes in context (slow, expensive, error-prone) | Python parses efficiently in container |
| Compute statistics | LLM "calculates" (often wrong) | Python/NumPy computes correctly |
| Transform data | MCP tool or host execution (security risk) | Isolated container (safe) |
| Run scripts | Not possible | Full Python/Node/Bash support |

## Quick Start

### 1. Enable Sandbox in Agent

Add `sandbox:` to your agent's dotprompt frontmatter:

```yaml
---
model: openai/gpt-4o
metadata:
  name: "Data Processor"
  description: "Processes data using Python in a sandbox"
sandbox: python
---

You are a data processing agent. Use the sandbox_run tool to execute Python code
for any data transformations, calculations, or analysis tasks.

{{role "user"}}
{{userInput}}
```

### 2. Configure Station (Optional)

Enable sandbox in your environment:

```bash
export STATION_SANDBOX_ENABLED=true
```

### 3. Run Your Agent

```bash
stn agent run "Data Processor" "Calculate the sum of numbers 1 to 100"
```

The agent will use the `sandbox_run` tool to execute Python code in an isolated container.

## Configuration Options

### Simple Form (String)

Use the runtime name directly:

```yaml
sandbox: python    # or: node, bash
```

### Structured Form (Object)

For advanced configuration:

```yaml
sandbox:
  runtime: python
  image: "python:3.11-slim"
  timeout_seconds: 300
  max_stdout_bytes: 200000
  allow_network: false
  pip_packages:
    - pandas
    - pyyaml
    - requests
```

### Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `runtime` | string | (required) | `python`, `node`, or `bash` |
| `image` | string | Auto-selected | Container image to use |
| `timeout_seconds` | int | 120 | Maximum execution time |
| `max_stdout_bytes` | int | 100000 | Truncate stdout after this many bytes |
| `allow_network` | bool | false | Allow network access in container |
| `pip_packages` | []string | [] | Python packages to install |
| `npm_packages` | []string | [] | Node.js packages to install |

### Default Images

| Runtime | Default Image |
|---------|---------------|
| Python | `python:3.11-slim` |
| Node.js | `node:20-slim` |
| Bash | `ubuntu:22.04` |

## Tool Schema

When an agent has sandbox enabled, it receives the `sandbox_run` tool:

### Input Parameters

```json
{
  "runtime": "python",
  "code": "print('Hello, World!')",
  "args": ["--verbose"],
  "env": {"DEBUG": "true"},
  "files": {
    "data.json": "{\"key\": \"value\"}"
  },
  "timeout_seconds": 60
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `code` | string | Yes | Source code to execute |
| `runtime` | string | No | Override default runtime |
| `args` | []string | No | Command-line arguments |
| `env` | object | No | Environment variables |
| `files` | object | No | Files to create in `/work` directory |
| `timeout_seconds` | int | No | Override default timeout |

### Output

```json
{
  "ok": true,
  "runtime": "python",
  "exit_code": 0,
  "duration_ms": 1250,
  "stdout": "Hello, World!\n",
  "stderr": "",
  "error": ""
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ok` | bool | Whether execution succeeded (exit_code == 0) |
| `runtime` | string | Runtime that was used |
| `exit_code` | int | Process exit code |
| `duration_ms` | int | Execution time in milliseconds |
| `stdout` | string | Standard output (truncated if needed) |
| `stderr` | string | Standard error (truncated if needed) |
| `error` | string | Error message if tool failed to execute |

## Examples

### Example 1: JSON Processing

**Agent prompt** (`json-processor.prompt`):
```yaml
---
model: openai/gpt-4o-mini
metadata:
  name: "JSON Processor"
sandbox: python
input:
  schema:
    type: object
    properties:
      userInput:
        type: string
      json_data:
        type: string
    required: [userInput]
---

You are a JSON processing agent. When given JSON data, use the sandbox_run tool
to parse and transform it using Python.

{{role "user"}}
{{userInput}}

Data to process:
```json
{{json_data}}
```
```

**Running the agent**:
```bash
stn agent run "JSON Processor" "Find the top 3 items by price" \
  --json-data '[{"name": "Apple", "price": 1.50}, {"name": "Banana", "price": 0.75}, {"name": "Orange", "price": 2.00}, {"name": "Grape", "price": 3.50}]'
```

**Agent uses sandbox_run**:
```python
import json

data = json.loads('''[{"name": "Apple", "price": 1.50}, {"name": "Banana", "price": 0.75}, {"name": "Orange", "price": 2.00}, {"name": "Grape", "price": 3.50}]''')
sorted_items = sorted(data, key=lambda x: x['price'], reverse=True)[:3]
print(json.dumps(sorted_items, indent=2))
```

**Output**:
```json
[
  {"name": "Grape", "price": 3.5},
  {"name": "Orange", "price": 2.0},
  {"name": "Apple", "price": 1.5}
]
```

### Example 2: Log Analysis with Pandas

**Agent prompt** (`log-analyzer.prompt`):
```yaml
---
model: openai/gpt-4o
metadata:
  name: "Log Analyzer"
sandbox:
  runtime: python
  pip_packages:
    - pandas
  timeout_seconds: 300
---

You are a log analysis expert. Use the sandbox_run tool to analyze log data
with pandas for accurate statistics and pattern detection.

{{role "user"}}
{{userInput}}
```

**Example sandbox execution**:
```python
import pandas as pd
from collections import Counter

log_lines = """
2025-01-15 10:00:01 ERROR Database connection failed
2025-01-15 10:00:02 INFO Request processed successfully
2025-01-15 10:00:03 ERROR Database connection failed
2025-01-15 10:00:04 WARN High memory usage detected
2025-01-15 10:00:05 ERROR Timeout waiting for response
2025-01-15 10:00:06 INFO Request processed successfully
""".strip().split('\n')

levels = [line.split()[2] for line in log_lines]
counts = Counter(levels)

print(f"Log Level Summary:")
print(f"  ERROR: {counts['ERROR']}")
print(f"  WARN:  {counts['WARN']}")
print(f"  INFO:  {counts['INFO']}")
print(f"\nTotal entries: {len(log_lines)}")
print(f"Error rate: {counts['ERROR']/len(log_lines)*100:.1f}%")
```

**Output**:
```
Log Level Summary:
  ERROR: 3
  WARN:  1
  INFO:  2

Total entries: 6
Error rate: 50.0%
```

### Example 3: Multi-file Processing

**Sandbox with input files**:
```python
# Agent sends files via the 'files' parameter
sandbox_run({
  "runtime": "python",
  "code": """
import json

# Files are available in /work directory
with open('/work/config.json') as f:
    config = json.load(f)

with open('/work/data.csv') as f:
    lines = f.readlines()

print(f"Config: {config['setting']}")
print(f"Data rows: {len(lines)}")
""",
  "files": {
    "config.json": '{"setting": "production"}',
    "data.csv": "id,value\n1,100\n2,200\n3,300"
  }
})
```

**Output**:
```
Config: production
Data rows: 4
```

---

## Code Mode: Persistent Linux Sandbox

Code Mode provides a **full Linux environment** that persists across multiple tool calls within a workflow. Unlike Compute Mode (ephemeral), Code Mode lets agents work like developers: install packages, compile code, iterate on files, and maintain state.

### Enable Code Mode

```yaml
---
model: openai/gpt-4o
metadata:
  name: "Developer Agent"
sandbox:
  mode: code
  session: workflow  # share container across workflow steps
---

You are a developer with access to a full Linux sandbox.
You can install packages, compile code, and run any shell commands.
```

### Code Mode Tools

When `mode: code` is set, agents receive these tools instead of `sandbox_run`:

| Tool | Description |
|------|-------------|
| `sandbox_open` | Get or create a persistent sandbox session |
| `sandbox_exec` | Execute any shell command |
| `sandbox_fs_write` | Write files to the sandbox |
| `sandbox_fs_read` | Read files from the sandbox |
| `sandbox_fs_list` | List directory contents |
| `sandbox_fs_delete` | Delete files or directories |
| `sandbox_close` | Explicitly close the session (optional) |

### Example: Install Packages and Compile Code

```bash
# Agent opens sandbox (ubuntu:22.04 by default)
sandbox_open({})

# Install build tools
sandbox_exec({"command": "apt-get update && apt-get install -y build-essential"})

# Write C code
sandbox_fs_write({
  "path": "hello.c",
  "content": "#include <stdio.h>\nint main() { printf(\"Hello!\\n\"); return 0; }"
})

# Compile and run
sandbox_exec({"command": "gcc -o hello hello.c && ./hello"})
# Output: Hello!
```

### Session Scoping

Sessions can be scoped to workflows (shared across steps) or agents (isolated per run):

```yaml
sandbox:
  mode: code
  session: workflow  # Container shared across all agents in workflow
  # session: agent   # New container per agent run
```

**Workflow scope example:**

```
Workflow: build-and-test
├── Step 1: code-writer agent
│   └── sandbox_fs_write → creates source files
├── Step 2: tester agent  
│   └── sandbox_exec → runs tests (files still there!)
└── Workflow completes → container destroyed
```

### Docker Image Options

Specify an image directly or use shortcuts:

```yaml
sandbox:
  mode: code
  runtime: linux     # ubuntu:22.04 (default)
  # runtime: python  # python:3.11-slim
  # runtime: node    # node:20-slim
  # image: golang:1.22  # any Docker image
```

### Code Mode Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `compute` | `compute` or `code` |
| `session` | string | `workflow` | `workflow` or `agent` |
| `runtime` | string | `linux` | `linux`, `python`, `node`, or custom |
| `image` | string | Auto | Override container image |
| `timeout_seconds` | int | 60 | Command execution timeout |
| `limits.max_file_size_bytes` | int | 10MB | Max file size |
| `limits.max_files` | int | 100 | Max files in sandbox |

### Requirements

Code Mode requires Docker:

```bash
# Docker must be available
docker ps

# Station needs Docker socket access
export DOCKER_HOST=unix:///var/run/docker.sock
```

---

## Deployment

### Docker Compose (Recommended for Development)

Mount the Docker socket:

```yaml
services:
  station:
    image: cloudshipai/station:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - STATION_SANDBOX_ENABLED=true
```

### Kubernetes / ECS

Use a Dagger engine sidecar:

```yaml
services:
  station:
    environment:
      - STATION_SANDBOX_ENABLED=true
      - DAGGER_HOST=tcp://dagger-engine:8080
  
  dagger-engine:
    image: registry.dagger.io/engine:latest
```

### Cloud Run / Serverless

Use Dagger Cloud or a remote Dagger engine:

```bash
export STATION_SANDBOX_ENABLED=true
export DAGGER_HOST=tcp://your-dagger-engine:8080
# Or use Dagger Cloud credentials
```

## Security Considerations

### Isolation

- Each sandbox execution runs in a **fresh container**
- No access to host filesystem
- No access to Station secrets by default
- Network disabled by default (`allow_network: false`)

### Resource Limits

- Execution timeout enforced (default: 120s)
- Output truncated at configured limits
- Container resources limited by Dagger engine configuration

### Image Policy

Only approved base images are allowed:
- `python:3.11-slim`
- `node:20-slim`
- `ubuntu:22.04`

Custom images can be enabled via `STATION_SANDBOX_ALLOWED_IMAGES`.

## Troubleshooting

### Sandbox Not Available

```
Error: sandbox_run tool not found
```

**Fix**: Ensure `STATION_SANDBOX_ENABLED=true` and the agent has `sandbox:` in frontmatter.

### Dagger Connection Failed

```
Error: failed to connect to Dagger engine
```

**Fix**: 
- Docker Compose: Mount `/var/run/docker.sock`
- Kubernetes: Ensure Dagger sidecar is running
- Check `DAGGER_HOST` environment variable

### Execution Timeout

```
Error: execution timed out after 120s
```

**Fix**: Increase `timeout_seconds` in sandbox configuration:

```yaml
sandbox:
  runtime: python
  timeout_seconds: 600  # 10 minutes
```

### Package Installation Failed

```
Error: pip install failed
```

**Fix**: Ensure `allow_network: true` if packages need to be downloaded:

```yaml
sandbox:
  runtime: python
  allow_network: true
  pip_packages:
    - pandas
```

## Best Practices

1. **Use for deterministic tasks**: Calculations, parsing, transformations
2. **Keep code simple**: Complex multi-file projects are harder to debug
3. **Handle errors gracefully**: Check `ok` and `exit_code` in output
4. **Set appropriate timeouts**: Don't use defaults for long-running tasks
5. **Minimize network access**: Only enable when absolutely necessary
6. **Use structured output**: Return JSON for easy parsing by the agent

## Next Steps

- [Creating Agents](../agents/CREATING_AGENTS.md) - Agent development guide
- [MCP Integration](../agents/MCP_INTEGRATION.md) - Using MCP tools with agents
- [Deployment Modes](./deployment-modes.md) - Station deployment options
