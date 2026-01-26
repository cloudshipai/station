# Agentic Harness Testing Operations Guide

**Last Verified**: 2025-01-26
**Test Results**: 90+ tests passing across harness packages (including Docker sandbox fixes)

## Quick Reference

```bash
# Run all harness unit tests
go test ./pkg/harness/... -v -count=1

# Run with race detection
go test ./pkg/harness/... -race -v

# Run E2E tests (requires real LLM)
HARNESS_E2E_TEST=1 go test ./pkg/harness/... -v -run E2E
```

---

## Test Coverage Summary

| Package | Tests | Status | Notes |
|---------|-------|--------|-------|
| `pkg/harness` | 23 | ✅ PASS | Core executor, config, doom loop |
| `pkg/harness/prompt` | 20 | ✅ PASS | Agent config parsing, frontmatter |
| `pkg/harness/skills` | 9 | ✅ PASS | Skills middleware, parsing |
| `pkg/harness/memory` | 6 | ✅ PASS | Memory config, middleware |
| `pkg/harness/hooks` | 8 | ✅ PASS | Permission checking, wildcard matching |
| `pkg/harness/stream` | 10 | ✅ PASS | Event streaming, publishers |
| `pkg/harness/workspace` | 10 | ✅ PASS | Host workspace operations |
| `pkg/harness/git` | 5 | ✅ PASS | Git manager integration |
| `pkg/harness/nats` | 4 | ✅ PASS | NATS adapters, lattice |
| `pkg/harness/sandbox` | 25+ | ✅ PASS | Docker sandbox fixed (uses docker cp for file ops) |

---

## Unit Tests

### Core Harness Tests (`pkg/harness/`)

```bash
go test ./pkg/harness/ -v -count=1
```

**What's tested:**
- `TestCompactor_*` - Context compaction, token counting, history truncation
- `TestDoomLoopDetector_*` - Doom loop detection via SHA256 hashing
- `TestAgenticExecutor_*` - Executor initialization, sandbox options
- `TestDefaultHarnessConfig` - Default config values
- `TestHookResult_*` - Pre/post hook execution priority

### Prompt/Config Tests (`pkg/harness/prompt/`)

```bash
go test ./pkg/harness/prompt/... -v -count=1
```

**What's tested:**
- `TestParseAgentConfig_*` - Frontmatter parsing (model, tools, harness, sandbox)
- `TestAgentConfig_Defaults` - Default value injection
- `TestBuilder_*` - System prompt building
- `TestAgentConfig_GetSkillSources_*` - Skill source resolution

### Permission/Hook Tests

```bash
go test ./pkg/harness/ -v -run "Hook\|Match\|Permission"
```

**What's tested:**
- `TestMatchWildcard` - Wildcard pattern matching (`git *`, `rm -rf *`)
- `TestCheckBashPermission` - Allow/deny/ask rule evaluation
- `TestHookRegistry_*` - Hook priority (Interrupt > Block > Continue)

---

## E2E Tests

E2E tests require a real LLM API key and are skipped by default.

### Enable E2E Tests

```bash
# Set required environment variables
export HARNESS_E2E_TEST=1
export OPENAI_API_KEY="sk-..."  # Or other provider

# Run all E2E tests
go test ./pkg/harness/... -v -run E2E -timeout 10m
```

### Available E2E Tests

| Test | What It Validates | Requirements |
|------|-------------------|--------------|
| `TestAgenticExecutor_E2E_RealLLM` | Basic file creation task | LLM API |
| `TestAgenticExecutor_E2E_MultiStep` | Multi-step task execution | LLM API |
| `TestAgenticExecutor_E2E_Compaction` | Context compaction triggers | LLM API |
| `TestAgenticExecutor_E2E_WorkflowSimulation` | Multi-agent handoff | LLM API |
| `TestAgenticExecutor_E2E_DockerSandbox` | Docker sandbox isolation | LLM API + Docker |
| `TestAgenticExecutor_E2E_DockerWorkflowHandoff` | Workspace persistence across containers | LLM API + Docker |

### Docker E2E Tests

```bash
# Requires Docker daemon running
HARNESS_E2E_TEST=1 go test ./pkg/harness/... -v -run Docker -timeout 10m
```

---

## Sandbox Tests

### Host Sandbox (Always Works)

```bash
go test ./pkg/harness/sandbox/... -v -run Host
```

### Docker Sandbox (Requires Docker)

```bash
# Run Docker-specific tests
go test ./pkg/harness/sandbox/... -v -run Docker

# Known failures in restricted environments:
# - TestDockerSandbox_CopyOut_AutoCreatesContainer
# - TestDockerSandbox_FileOperations
# These require Docker with proper permissions for container file ops
```

### E2B Sandbox (Mocked by Default)

```bash
# E2B tests use mocks - no API key needed for unit tests
go test ./pkg/harness/sandbox/... -v -run E2B

# Real E2B integration (requires API key)
E2B_API_KEY="..." go test ./pkg/harness/sandbox/... -v -run E2B -tags=integration
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
name: Harness Tests

on:
  push:
    paths:
      - 'pkg/harness/**'
  pull_request:
    paths:
      - 'pkg/harness/**'

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run Unit Tests
        run: go test ./pkg/harness/... -v -count=1 -race

      - name: Run Unit Tests (exclude Docker)
        run: go test ./pkg/harness/... -v -count=1 -skip "Docker"

  e2e-tests:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run E2E Tests
        env:
          HARNESS_E2E_TEST: "1"
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: go test ./pkg/harness/... -v -run E2E -timeout 15m
```

---

## Environment Variables

| Variable | Purpose | Required For |
|----------|---------|--------------|
| `HARNESS_E2E_TEST=1` | Enable E2E tests | All E2E tests |
| `OPENAI_API_KEY` | OpenAI API access | E2E with GPT models |
| `ANTHROPIC_API_KEY` | Anthropic API access | E2E with Claude models |
| `GEMINI_API_KEY` | Google AI access | E2E with Gemini models |
| `E2B_API_KEY` | E2B cloud sandbox | E2B integration tests |

---

## Debugging Test Failures

### Common Issues

**1. Docker tests fail with permission errors**
```
WriteFile failed: failed to write file /workspace/test.txt: exit status 1
```
- **Cause**: Docker container can't write to mounted volume
- **Fix**: Check Docker socket permissions, try `docker run --privileged`

**2. E2E tests timeout**
```
panic: test timed out after 2m0s
```
- **Cause**: LLM API slow or rate limited
- **Fix**: Increase timeout: `go test -timeout 10m`

**3. Doom loop detection triggers incorrectly**
```
Doom loop detected: tool 'bash' has been called 3 times
```
- **Cause**: Test task causes repeated tool calls
- **Fix**: Increase threshold in test config: `DoomLoopThreshold: 5`

### Verbose Debugging

```bash
# Enable all logging
go test ./pkg/harness/... -v -count=1 2>&1 | tee test-output.log

# Run single test with verbose output
go test ./pkg/harness/ -v -run TestDoomLoopDetector_DetectsLoop -count=1
```

---

## Test Data Locations

| Data | Location |
|------|----------|
| E2E prompts | `/tmp/harness-e2e-prompts/` |
| Temp workspaces | `/tmp/harness-e2e-*/` |
| Test configs | Inline in test files |

---

## Session Persistence (NEW)

Session persistence enables REPL-style prolonged conversations with message history.

### Session Tests

```bash
go test ./pkg/harness/session/... -v -count=1
```

| Test | What It Validates |
|------|-------------------|
| `TestHistoryStore_SaveLoad` | Message history save/load to disk |
| `TestHistoryStore_Append` | Appending messages across calls |
| `TestHistoryStore_Clear` | Clearing history |
| `TestSessionManager_Integration` | Full session lifecycle with history persistence |

### How Session Persistence Works

```
REPL Session Flow:
┌─────────────────────────────────────────────────────────────────┐
│  1. StartSession(sessionID)                                      │
│     ├── GetOrCreate session workspace                            │
│     ├── AcquireLock (prevent concurrent access)                  │
│     ├── Initialize workspace directory                           │
│     └── Create sandbox (keep alive for session duration)         │
├─────────────────────────────────────────────────────────────────┤
│  2. Execute(task) - repeat for each user input                   │
│     ├── Load history from .history.json                          │
│     ├── Create AgenticExecutor with loaded history               │
│     ├── Run task                                                 │
│     ├── Append new messages to history                           │
│     └── Save history to disk                                     │
├─────────────────────────────────────────────────────────────────┤
│  3. EndSession()                                                 │
│     ├── Destroy sandbox (or keep for future resume)              │
│     └── ReleaseLock                                              │
└─────────────────────────────────────────────────────────────────┘

Files stored per session:
~/.config/station/workspace/session/{session-id}/
├── .session.meta    # Session metadata (created_at, total_runs)
├── .session.lock    # Lock file (PID, expires_at)
├── .history.json    # Message history (all user/assistant messages)
└── (workspace files created by agent)
```

---

## Coverage Gaps (TODO)

The following areas need additional test coverage:

| Area | Current State | Priority |
|------|---------------|----------|
| Session → AgenticExecutor history injection | ✅ Complete | ~~HIGH~~ |
| REPL Session Persistence | ✅ Complete | ~~HIGH~~ |
| Workflow approval integration | No tests | HIGH |
| NATS streaming E2E | Mocked only | MEDIUM |
| Lattice multi-station handoff | No tests | MEDIUM |
| Memory injection E2E | No tests | MEDIUM |
| Skills progressive disclosure | Basic tests | LOW |

### Recommended New Tests

1. **Workflow Approval E2E**
   ```go
   func TestWorkflowApproval_HumanInTheLoop(t *testing.T) {
       // Test that HookInterrupt properly pauses execution
       // Test approval/rejection flow
   }
   ```

2. **NATS Streaming E2E**
   ```go
   func TestNATSStreaming_EventDelivery(t *testing.T) {
       // Test real NATS event publishing
       // Test subscriber receives events
   }
   ```

3. **Memory Context Injection**
   ```go
   func TestMemoryInjection_E2E(t *testing.T) {
       // Test AGENTS.md loading
       // Test CloudShip memory API integration
   }
   ```

---

## Running Tests Locally

### Full Test Suite (Fast)

```bash
# Excludes E2E and Docker tests
go test ./pkg/harness/... -v -count=1 -skip "E2E|Docker"
```

### Full Test Suite (Complete)

```bash
# All tests including Docker (requires Docker)
HARNESS_E2E_TEST=1 OPENAI_API_KEY="sk-..." \
  go test ./pkg/harness/... -v -timeout 15m
```

### Quick Smoke Test

```bash
# Just the core logic
go test ./pkg/harness/ -v -count=1
```

---

## Maintenance Notes

- **Update this doc** when adding new test files
- **Tag flaky tests** with `t.Skip()` and reason
- **Docker tests** may fail in CI without privileged mode
- **E2E tests** cost money (LLM API calls) - run sparingly
