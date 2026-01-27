# Harness CLI Commands

The `stn harness` command group provides first-class support for agentic harness agents - multi-turn AI agents that can autonomously execute complex tasks.

## Quick Start

```bash
# Create a new harness agent from template
stn harness init my-coder --template coding

# Sync to activate the agent
stn sync

# Run a task
stn harness run my-coder "Fix the bug in main.go"

# Or use interactive REPL
stn harness repl --agent my-coder
```

## Commands

### `stn harness init` - Scaffold New Agent

Create a new harness agent from a pre-configured template.

```bash
stn harness init <agent-name> [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-t, --template` | Template to use | `minimal` |
| `-e, --env` | Environment to create in | `default` |
| `--sandbox` | Sandbox mode (host, docker) | template default |
| `--image` | Docker image for sandbox | template default |
| `--max-steps` | Maximum execution steps | template default |
| `--timeout` | Execution timeout | template default |
| `--tools` | Additional tools to include | - |
| `--no-sync` | Skip syncing after creation | `false` |

**Available Templates:**

| Template | Description | Max Steps | Timeout | Sandbox |
|----------|-------------|-----------|---------|---------|
| `coding` | Multi-turn coding assistant | 50 | 30m | host |
| `sre` | Site reliability engineering | 30 | 15m | host |
| `security` | Security scanning agent | 40 | 20m | docker |
| `data` | Data analysis agent | 25 | 10m | docker |
| `minimal` | Minimal customization base | 10 | 5m | host |

**Examples:**

```bash
# Create from coding template
stn harness init my-coder --template coding

# Create security scanner with custom image
stn harness init scanner --template security --image python:3.11

# Create with extra tools
stn harness init my-agent --template minimal --tools curl,jq

# Create in specific environment
stn harness init prod-agent --template sre --env production
```

### `stn harness run` - Execute Agent (One-Shot)

Execute a harness agent with a task and wait for completion.

```bash
stn harness run <agent-name> "<task>" [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-e, --env` | Environment name | `default` |
| `--workspace` | Workspace directory | current dir |
| `--var` | Variables (key=value) | - |
| `--stream` | Stream output real-time | `false` |
| `--resume` | Resume session ID | - |
| `--max-steps` | Override max steps | agent config |
| `--timeout` | Override timeout | agent config |
| `--json` | Output as JSON | `false` |

**Examples:**

```bash
# Basic task execution
stn harness run my-coder "Fix the bug in auth.go"

# Run with custom workspace
stn harness run my-coder "Review code" --workspace ./my-project

# Use template variables in task
stn harness run my-agent "Analyze {{path}}" --var path=./src

# Stream output as it happens
stn harness run my-coder "Write tests" --stream

# Resume a previous session
stn harness run my-coder "Continue the refactoring" --resume session_abc123

# JSON output for scripting
stn harness run my-coder "List files" --json
```

### `stn harness repl` - Interactive Development

Start an interactive REPL session for testing and development.

```bash
stn harness repl [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-a, --agent` | Agent name to run | required |
| `-e, --env` | Environment name | `default` |
| `-s, --session` | Session ID to resume | - |
| `--sandbox` | Sandbox mode | agent config |
| `--skills` | Additional skill paths | - |
| `--max-steps` | Max steps per execution | `50` |
| `--timeout` | Execution timeout | `30m` |

**REPL Commands:**
| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/status` | Session status (steps, tokens) |
| `/history` | Conversation history |
| `/skills` | List available skills |
| `/memory` | List loaded memory sources |
| `/tools` | List available tools |
| `/reset` | Reset session (start fresh) |
| `/save FILE` | Save session to file |
| `/load FILE` | Load session from file |
| `/exit` | Exit REPL |

**Example:**

```bash
stn harness repl --agent my-coder

╭─────────────────────────────────────────╮
│       Station Agent REPL                │
╰─────────────────────────────────────────╯

>>> Find bugs in main.go

[Step 1/50] Reading main.go...
✓ read: main.go (245 lines)

I found 2 potential issues:
1. Null pointer on line 42
2. Missing error handling on line 78

>>> Fix issue #1

[Step 2/50] Editing main.go...
✓ edit: main.go (1 replacement)

Fixed null pointer with nil check.

>>> /status
Session Status:
  Steps:  2
  Tokens: 1,234
```

### `stn harness inspect` - Inspect Run

Display detailed information about a harness run.

```bash
stn harness inspect <run-id> [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-v, --verbose` | Show detailed output | `false` |
| `--json` | Output as JSON | `false` |

**Example:**

```bash
stn harness inspect 123 -v

╭─ Run: 123 ────────────────────────────╮
│ Agent:    my-coder                    │
│ Status:   success                     │
│ Duration: 2m34s                       │
│ Steps:    12                          │
╰───────────────────────────────────────╯

Task:
  Fix the authentication bug

Response:
  Fixed the null pointer exception in auth.go...
```

### `stn harness runs` - List Runs

List recent harness agent runs.

```bash
stn harness runs [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--agent` | Filter by agent name | - |
| `--limit` | Max runs to show | `20` |
| `--status` | Filter by status | - |

**Example:**

```bash
stn harness runs --limit 5

ID       AGENT                     STATUS     STEPS  DURATION
123      my-coder                  success    12     2m34s
122      security-scanner          success    8      1m12s
121      my-coder                  error      5      45s
```

### `stn harness templates` - List Templates

Show all available agent templates.

```bash
stn harness templates

Available Harness Templates:
============================

coding
------
  Description: Multi-turn coding assistant with file operations
  Max Steps:   50
  Timeout:     30m
  Sandbox:     host
  Tools:       read, write, edit, bash, glob, grep
```

## Agent Configuration

Harness agents are defined in `.prompt` files with special frontmatter:

```yaml
---
metadata:
  name: "my-coder"
  description: "Multi-turn coding assistant"
  tags:
    - harness
    - coding
model: gpt-4o-mini
max_steps: 50
harness:
  max_steps: 50
  timeout: 30m
  sandbox:
    mode: docker
    image: python:3.11
tools:
  - read
  - write
  - edit
  - bash
  - glob
  - grep
---

{{role "system"}}
You are an expert coding assistant...

{{role "user"}}
{{userInput}}
```

### Harness Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `harness.max_steps` | Maximum tool calls | `50` |
| `harness.timeout` | Execution timeout | `30m` |
| `harness.sandbox.mode` | Isolation mode | `host` |
| `harness.sandbox.image` | Docker image | - |
| `harness.doom_loop_threshold` | Loop detection | `3` |

## Session Management

Harness agents support persistent sessions for continuity:

```bash
# List active sessions
stn session list

# Resume a session
stn harness run my-coder --resume session_abc123

# Delete a session
stn session delete session_abc123

# Cleanup old sessions
stn session cleanup --older-than 7d
```

## Entrypoints

Harness agents can be triggered through multiple entrypoints:

### 1. CLI (Interactive) ✅ Implemented

```bash
# One-shot execution
stn harness run my-agent "Fix the bug"

# Interactive REPL
stn harness repl --agent my-agent

# Resume session
stn harness run my-agent --resume session_abc123
```

### 2. Cron (Scheduled) ✅ Implemented

Harness agents automatically use harness execution when scheduled:

```bash
# Create agent with schedule (6-field cron with seconds)
stn agent create nightly-scanner \
  --description "Nightly security scan" \
  --prompt "Scan the codebase for security issues" \
  --harness-config '{"max_steps":100,"timeout":"1h"}' \
  --schedule "0 0 2 * * *" \
  --schedule-variables '{"repo":"/path/to/repo"}'

# Or use MCP to set schedule on existing agent
# The scheduler automatically detects harness config and uses harness execution
```

**How it works:**
1. Agent is created with `harness_config` in the .prompt file
2. Schedule is set via CLI or MCP (`set_schedule` tool)
3. SchedulerService triggers at cron time
4. AgentService detects harness config and uses `AgenticExecutor`
5. Full multi-turn execution happens with configured tools

### 3. MCP/API (Programmatic) ✅ Implemented

```bash
# Via MCP call_agent tool
call_agent(
  agent_id="code-reviewer",
  task="Review PR #123",
  variables={"pr_number": 123}
)
```

Harness agents are called the same way as regular agents - the execution engine automatically detects `harness_config` and uses the agentic harness.

### 4. Workflow (Pipeline Step) ✅ Implemented

```yaml
id: code-review-pipeline
states:
  - name: analyze
    type: agent
    agent: code-analyzer
    # Agent's harness_config is automatically used
    input:
      path: "${ctx.repo_path}"
    resultPath: analysis
    next: fix

  - name: fix
    type: agent
    agent: code-fixer  # Harness agent
    input:
      issues: "${steps.analysis.issues}"
```

### 5. Webhook (Event-Driven) 🚧 Planned (Phase 8)

Webhook entrypoints are planned for Phase 8:

```yaml
# Future: Agent with webhook trigger
name: pr-reviewer
harness: agentic
webhook:
  path: /hooks/pr-review
  events: ["pull_request.opened"]
  task_template: "Review PR #{{.pull_request.number}}"
```

### 6. Event Subscription (NATS/PubSub) 🚧 Planned (Phase 8)

Event-driven triggers via NATS are planned:

```yaml
# Future: Agent subscribing to events
name: incident-responder
harness: agentic
subscribe:
  - subject: "alerts.pagerduty.triggered"
    task_template: "Investigate: {{.incident.title}}"
```

### Entrypoint Summary

| Entrypoint | Status | Command/Config |
|------------|--------|----------------|
| CLI One-shot | ✅ | `stn harness run` |
| CLI REPL | ✅ | `stn harness repl` |
| Cron Scheduled | ✅ | `stn agent create --schedule` |
| MCP/API | ✅ | `call_agent` tool |
| Workflow | ✅ | `type: agent` state |
| Webhook | 🚧 | Planned Phase 8 |
| Event Sub | 🚧 | Planned Phase 8 |

## Best Practices

1. **Start with templates** - Use `stn harness init` with templates rather than creating from scratch

2. **Use sessions** - For multi-step tasks, use sessions to maintain state

3. **Set appropriate limits** - Configure `max_steps` and `timeout` based on task complexity

4. **Use sandboxing** - For untrusted code, use Docker sandbox mode

5. **Monitor runs** - Use `stn harness runs` and `inspect` to debug issues

## See Also

- [Agentic Harness PRD](../features/PRD_AGENTIC_HARNESS.md) - Full architecture documentation
- [Harness DX Proposal](../HARNESS_DX_PROPOSAL.md) - Developer experience design
- [Session Management](../features/PRD_AGENTIC_HARNESS.md#phase-2-session-and-workspace-management) - Session details
