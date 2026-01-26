# Agentic Harness - Advanced Agent Execution Engine

The **Agentic Harness** is Station's Claude Agent SDK-like execution engine for agents that need full agentic capabilities: multi-turn reasoning, persistent tool execution, and isolated environments.

## Overview

The harness provides:
- **Multi-turn execution** with configurable step limits and doom loop protection
- **Sandboxed environments** for isolated tool execution (host, Docker, or E2B)
- **Built-in tools** for file operations, bash, and more
- **Streaming output** with real-time progress

## Quick Start

Create an agent with harness enabled:

```yaml
---
name: coding-agent
description: An agent that can write and modify code
harness: agentic
harness_config:
  max_steps: 50
  doom_loop_threshold: 5
  timeout: 30m
  sandbox:
    mode: docker
    image: python:3.11-slim
tools:
  - read
  - write
  - bash
  - glob
  - grep
  - edit
---
You are a coding agent. Help users write, debug, and improve code.

When given a task:
1. Understand the requirements
2. Plan your approach
3. Implement the solution
4. Verify it works

Be thorough but efficient.
```

## Frontmatter Configuration

### Basic Structure

```yaml
---
name: agent-name
description: What this agent does
harness: agentic              # Enable agentic harness
harness_config:               # Harness-specific configuration
  max_steps: 50               # Max reasoning steps (default: 25)
  doom_loop_threshold: 5      # Consecutive similar outputs before abort (default: 3)
  timeout: 30m                # Overall execution timeout (default: 15m)
  sandbox:                    # Execution environment (optional)
    mode: host                # host | docker | e2b
    # ... mode-specific options
tools:
  - read
  - write
  - bash
---
System prompt here...
```

### Harness Config Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_steps` | int | 25 | Maximum agentic reasoning steps |
| `doom_loop_threshold` | int | 3 | Abort after N consecutive similar outputs |
| `timeout` | duration | 15m | Overall execution timeout |
| `sandbox` | object | null | Sandbox configuration (see below) |

## Sandbox Modes

The `harness_config.sandbox` determines WHERE tools execute:

### Host Mode (Default)

Tools execute directly on the host machine. Fast but no isolation.

```yaml
harness_config:
  sandbox:
    mode: host
```

### Docker Mode

Tools execute inside a Docker container with workspace volume mounting.

```yaml
harness_config:
  sandbox:
    mode: docker
    image: python:3.11-slim    # Container image
    network: false             # Disable network access
    timeout: 5m                # Per-command timeout
    memory: 4g                 # Memory limit
    cpu: 2                     # CPU limit
    environment:               # Environment variables
      PYTHONPATH: /workspace
```

**Docker Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `image` | string | python:3.11-slim | Docker image |
| `network` | bool | false | Enable network access |
| `timeout` | duration | 5m | Per-command timeout |
| `memory` | string | 4g | Memory limit (Docker format) |
| `cpu` | int | 2 | CPU core limit |
| `environment` | map | {} | Environment variables |

**How Docker Mode Works:**
- Workspace is mounted as a volume (`-v /host/path:/workspace`)
- Files persist across container destroys
- Each tool execution gets a fresh container but sees same files
- Great for reproducible, isolated execution

### E2B Mode (Experimental)

Tools execute in E2B cloud VMs.

```yaml
harness_config:
  sandbox:
    mode: e2b
    template: base             # E2B template
    timeout: 10m               # VM timeout
```

> **Note:** E2B mode is experimental. Data does not persist across sandbox destroys - the harness would need CopyIn/CopyOut sync between agent steps for stateful workflows. Use Docker mode for production workloads.

## Built-in Tools

The harness provides these tools by default when enabled:

| Tool | Description |
|------|-------------|
| `read` | Read file contents |
| `write` | Write/create files |
| `edit` | Edit files with search/replace |
| `bash` | Execute shell commands |
| `glob` | Find files by pattern |
| `grep` | Search file contents |

Add them to your agent's `tools:` list:

```yaml
tools:
  - read
  - write
  - bash
  - glob
  - grep
  - edit
```

## Harness vs Legacy Sandbox

**Important:** There are two sandbox concepts in Station:

| Feature | Legacy `sandbox:` | Harness `harness_config.sandbox:` |
|---------|-------------------|-----------------------------------|
| Purpose | One-shot code execution via `sandbox_run` tool | Persistent environment for agentic tools |
| Location | Root frontmatter | Nested under `harness_config` |
| Runtime | Dagger-based | Docker or E2B |
| Use Case | Single script execution | Multi-turn agent workflows |

**Legacy sandbox (still works for non-harness agents):**
```yaml
---
name: simple-agent
sandbox:                    # OLD - for sandbox_run tool
  mode: compute
  runtime: python
tools:
  - sandbox_run
---
```

**Harness sandbox:**
```yaml
---
name: coding-agent
harness: agentic
harness_config:
  sandbox:                  # NEW - execution environment
    mode: docker
tools:
  - read
  - write
  - bash
---
```

## Example Agents

### Python Development Agent

```yaml
---
name: python-dev
description: Python development assistant
harness: agentic
harness_config:
  max_steps: 100
  timeout: 1h
  sandbox:
    mode: docker
    image: python:3.11
    environment:
      PIP_NO_CACHE_DIR: "1"
tools:
  - read
  - write
  - edit
  - bash
  - glob
  - grep
---
You are a Python development assistant.

You can:
- Write and modify Python code
- Run tests with pytest
- Install packages with pip
- Debug issues

Always:
1. Read existing code before modifying
2. Run tests after changes
3. Explain what you changed and why
```

### Infrastructure Agent

```yaml
---
name: infra-agent
description: Infrastructure and DevOps tasks
harness: agentic
harness_config:
  max_steps: 50
  sandbox:
    mode: docker
    image: hashicorp/terraform:latest
    network: true  # Need network for cloud APIs
tools:
  - read
  - write
  - bash
  - glob
---
You are an infrastructure assistant specializing in Terraform and cloud resources.

Help users:
- Write Terraform configurations
- Plan and validate changes
- Debug infrastructure issues
```

## Creating Harness Agents

### Via MCP (create_agent tool)

```json
{
  "name": "my-harness-agent",
  "description": "Agent with agentic harness",
  "prompt": "You are a helpful agent...",
  "environment_id": "1",
  "harness_config": {
    "max_steps": 50,
    "doom_loop_threshold": 5,
    "timeout": "30m",
    "sandbox": {
      "mode": "docker",
      "image": "python:3.11"
    }
  },
  "tool_names": ["read", "write", "bash", "glob", "grep", "edit"]
}
```

### Via CLI

```bash
stn agent create my-agent \
  --prompt "You are a helpful agent..." \
  --description "Agent with harness" \
  --harness-config '{"max_steps": 50, "sandbox": {"mode": "docker"}}' \
  --tools read,write,bash,glob,grep,edit
```

### Via .prompt File

Create `environments/default/agents/my-agent.prompt`:

```yaml
---
name: my-agent
description: Agent with harness
harness: agentic
harness_config:
  max_steps: 50
  sandbox:
    mode: docker
tools:
  - read
  - write
  - bash
---
Your system prompt here...
```

Then sync the environment:

```bash
stn sync default
```

## Execution Flow

1. **Task Received** - Agent receives user task
2. **Sandbox Created** - If configured, sandbox environment is provisioned
3. **Tool Registry Built** - Tools are registered with sandbox executor
4. **Agentic Loop** - Agent reasons and calls tools iteratively
5. **Doom Loop Check** - Detects repetitive behavior
6. **Step Limit Check** - Enforces max_steps
7. **Timeout Check** - Enforces overall timeout
8. **Result Returned** - Final response or error

## Troubleshooting

### "Doom loop detected"

Agent is producing repetitive outputs. Solutions:
- Improve system prompt with clearer instructions
- Increase `doom_loop_threshold` if behavior is legitimate
- Check if the task is achievable

### Docker container fails to start

- Ensure Docker is running: `docker ps`
- Check image exists: `docker pull <image>`
- Verify memory/CPU limits are reasonable

### E2B timeout

- E2B VMs have session limits
- Consider Docker mode for longer tasks
- Check E2B API key is valid

### Tools not found

- Ensure tools are listed in `tools:` array
- Harness tools (read, write, bash, etc.) must be explicitly listed
- MCP tools require the MCP server to be running

## Best Practices

1. **Start with host mode** for development, switch to Docker for production
2. **Set reasonable timeouts** - longer isn't always better
3. **Use doom loop protection** - prevents runaway agents
4. **List only needed tools** - fewer tools = better focus
5. **Test in isolation** - use Docker for reproducible results
