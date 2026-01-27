# Harness Developer Experience Proposal

## Overview

This document proposes improvements to the Station harness developer experience, making it easier to create, test, debug, and deploy harness agents.

## Current Gaps

1. No dedicated `stn harness` command group
2. No scaffolding/templates for common agent patterns
3. No REPL/interactive development mode
4. Unclear workflow integration
5. Limited debugging/inspection tools

## Proposed CLI Commands

### 1. `stn harness init` - Scaffold New Harness Agent

```bash
# Interactive mode
stn harness init
? Agent name: code-reviewer
? Description: Reviews code for security and best practices
? Sandbox mode: docker
? Docker image: python:3.11-slim
? Max steps: 50
? Timeout: 30m
? Select tools: [x] read [x] write [x] bash [x] glob [x] grep [x] edit
✓ Created environments/default/agents/code-reviewer.prompt
✓ Synced environment

# One-liner
stn harness init code-reviewer \
  --template coding \
  --sandbox docker \
  --image python:3.11
```

### 2. `stn harness run` - Execute Harness Agent

```bash
# Basic execution
stn harness run code-reviewer "Review the auth module for security issues"

# With workspace
stn harness run code-reviewer "Fix the tests" --workspace ./my-project

# With variables
stn harness run code-reviewer "Analyze {{path}}" --var path=./src

# Streaming output
stn harness run code-reviewer "task" --stream

# Resume from checkpoint
stn harness run code-reviewer --resume <session-id>
```

### 3. `stn harness repl` - Interactive Development Mode

```bash
stn harness repl code-reviewer

╭──────────────────────────────────────────────────╮
│ Harness REPL: code-reviewer                      │
│ Type /help for commands, Ctrl+C to exit          │
╰──────────────────────────────────────────────────╯

[code-reviewer] > Analyze the main.py file for bugs

[Step 1/50] Reading main.py...
✓ read: main.py (245 lines)

[Step 2/50] Analyzing code...
I found 3 potential issues:
1. SQL injection on line 42
2. Missing input validation on line 78
3. Hardcoded secret on line 15

[code-reviewer] > Fix issue #1

[Step 3/50] Editing main.py...
✓ edit: main.py (1 replacement)
Fixed SQL injection using parameterized query.

[code-reviewer] > /checkpoint save
✓ Saved checkpoint: cp_abc123

[code-reviewer] > /tools
Available tools:
  read   - Read file contents
  write  - Write/create files
  edit   - Edit with search/replace
  bash   - Execute shell commands
  glob   - Find files by pattern
  grep   - Search file contents

[code-reviewer] > /exit
Session saved: session_xyz789
```

### 4. `stn harness debug` - Debug/Inspect Runs

```bash
# List recent harness runs
stn harness runs
ID          AGENT           STATUS    STEPS   DURATION
run_abc123  code-reviewer   success   12      2m34s
run_def456  code-reviewer   error     5       45s
run_ghi789  test-runner     success   8       1m12s

# Inspect a run
stn harness inspect run_abc123

╭─ Run: run_abc123 ─────────────────────────────────╮
│ Agent:    code-reviewer                           │
│ Status:   success                                 │
│ Steps:    12/50                                   │
│ Tokens:   4,521                                   │
│ Duration: 2m34s                                   │
│ Sandbox:  docker (python:3.11)                    │
╰───────────────────────────────────────────────────╯

Step Timeline:
  1. [0.2s] read → main.py
  2. [0.1s] read → utils.py
  3. [1.2s] LLM reasoning
  4. [0.3s] edit → main.py (1 change)
  5. [0.8s] bash → pytest tests/
  ...

# Stream logs
stn harness logs run_abc123 --follow

# Replay from step
stn harness replay run_abc123 --from-step 5
```

### 5. `stn harness test` - Test Harness Agents

```bash
# Run test suite
stn harness test code-reviewer

Running tests for code-reviewer...

  ✓ Can read files in workspace
  ✓ Can edit files safely
  ✓ Blocks dangerous commands
  ✓ Respects max_steps limit
  ✓ Handles timeout gracefully
  ✓ Doom loop detection works

6/6 tests passed

# Test with specific scenarios
stn harness test code-reviewer --scenario security-scan

# Test with custom task
stn harness test code-reviewer --task "Review code" --expect-tool read
```

## Proposed File Format Enhancements

### Input Schema with Examples

```yaml
---
name: code-reviewer
description: Reviews code for issues
harness: agentic
harness_config:
  max_steps: 50
  sandbox:
    mode: docker
    image: python:3.11

# NEW: Input schema with examples for testing
input:
  schema:
    type: object
    properties:
      path:
        type: string
        description: Path to review
      focus:
        type: string
        enum: [security, performance, style]
  examples:
    - path: ./src
      focus: security
    - path: ./tests
      focus: style

# NEW: Expected behaviors for testing
behaviors:
  - name: reads-before-editing
    description: Agent should read files before editing
    expect:
      - tool: read
        before: edit

  - name: blocks-dangerous-commands
    description: Agent should not run rm -rf
    expect:
      - tool: bash
        input_not_contains: "rm -rf"

tools:
  - read
  - write
  - bash
---
System prompt...
```

### Workflow Integration

```yaml
# workflows/code-review-pipeline.yaml
id: code-review-pipeline
name: Code Review Pipeline
trigger:
  - type: webhook
    path: /pr-opened

states:
  - name: review_code
    type: harness_agent          # NEW: Dedicated harness state type
    agent: code-reviewer
    harness:
      workspace: "{{ ctx.repo_path }}"
      timeout: 15m
      sandbox:
        mode: docker
        network: false
    input:
      path: "{{ ctx.changed_files }}"
      focus: security
    resultPath: review_result
    next: post_comment

  - name: post_comment
    type: agent
    agent: github-commenter
    input:
      pr_number: "{{ ctx.pr_number }}"
      comment: "{{ steps.review_result.findings }}"
    end: true
```

## Entrypoints Summary

| Entrypoint | Command | Use Case |
|------------|---------|----------|
| CLI One-shot | `stn harness run agent "task"` | Quick execution |
| CLI REPL | `stn harness repl agent` | Interactive development |
| Workflow | `type: harness_agent` state | Automated pipelines |
| MCP | `call_agent` with harness config | AI-native integration |
| API | `POST /api/v1/harness/run` | External systems |
| Cron | `stn schedule set` | Scheduled execution |

## Implementation Priority

### Phase 1: Core DX (High Priority)
1. `stn harness init` - Scaffold with templates
2. `stn harness run` - Direct execution with workspace
3. `stn harness inspect` - Run inspection

### Phase 2: Development Tools (Medium Priority)
4. `stn harness repl` - Interactive mode
5. `stn harness test` - Testing framework
6. `stn harness logs` - Log streaming

### Phase 3: Advanced Features (Lower Priority)
7. Workflow `harness_agent` state type
8. Checkpoint/resume capabilities
9. Behavior assertions for testing

## Migration Path

Existing agents with `--harness-config` continue to work. The new commands are additive:

```bash
# Old way (still works)
stn agent create my-agent --harness-config '{"max_steps": 50}'
stn agent run my-agent "task"

# New way (proposed)
stn harness init my-agent --template coding
stn harness run my-agent "task"
```

## Questions to Resolve

1. **Workspace Ownership**: When harness runs in Docker, who owns created files?
2. **Session Persistence**: How long do REPL sessions persist?
3. **Multi-Agent Handoffs**: Can one harness agent spawn another?
4. **Cost Tracking**: How to track token costs per harness run?
5. **Streaming**: How to stream tool outputs in real-time?
