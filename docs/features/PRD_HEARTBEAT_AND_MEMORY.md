# PRD: Heartbeat & Memory Improvements for Agentic Harness

> Enhancing Station's proactive agent capabilities with structured heartbeat polling and improved memory persistence.

## Cross-Reference: Integration Points

This PRD maps directly to existing Station architecture. **No major refactoring required.**

| Feature | Existing Code | Integration Point | Change Type |
|---------|---------------|-------------------|-------------|
| Heartbeat Config | `pkg/harness/prompt/loader.go:42-49` | Add `Heartbeat *HeartbeatConfig` to `HarnessConfig` struct | Additive |
| Heartbeat Emission | `pkg/harness/executor.go:553` | Hook into `emitStepComplete()` | Wrapper |
| Memory Files | `pkg/harness/memory/middleware.go:110-116` | Extend `DefaultMemorySources()` | Additive |
| Memory Flush | `pkg/harness/executor.go:231` | Add flush before `cleanup()` | Insert |
| Memory Write | `pkg/harness/memory/middleware.go` | Add `WriteMemory()` method | New method |
| Heartbeat State | `pkg/harness/session/history_store.go:44` | Use existing `Metadata` field | Reuse |
| Scheduler Integration | `internal/services/scheduler.go:275` | Wrap `ExecuteAgentWithRunID()` | Wrapper |

## Executive Summary

This PRD proposes two interconnected improvements to Station's agentic harness system:

1. **Heartbeat System** - Periodic agent polling with checklist-driven awareness
2. **Memory Improvements** - Enhanced memory persistence with automatic flush and workspace-based storage

These features enable truly autonomous agents that can maintain context across sessions, proactively check on tasks, and persist important information without explicit user commands.

---

## Problem Statement

### Current Limitations

**Scheduling:**
- Agents can be scheduled via cron, but only with static task strings and variables
- No mechanism for agents to "check in" periodically with context awareness
- Scheduled runs are isolated - no continuity between runs

**Memory:**
- Current memory implementation is naive (basic context injection)
- No automatic persistence before session ends
- Memory is not structured for different purposes (daily logs vs long-term facts)
- No workspace-based memory files that agents can read/update

### User Needs

1. **Proactive Awareness**: Agents should periodically check on things (queues, alerts, status) without explicit user requests
2. **Context Continuity**: Scheduled runs should be able to reference previous context
3. **Durable Memory**: Important information should persist automatically, not just when explicitly told
4. **Self-Updating Checklists**: Agents should be able to maintain their own task lists

---

## Proposed Solution

### 1. Heartbeat System

A heartbeat is a **periodic agent turn** that runs in the main session with full context, driven by a checklist file in the workspace.

#### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HEARTBEAT CYCLE                          │
│                                                             │
│  ┌─────────┐    ┌──────────────┐    ┌───────────────────┐  │
│  │ Timer   │───▶│ Read         │───▶│ Inject checklist  │  │
│  │ (30m)   │    │ HEARTBEAT.md │    │ as user message   │  │
│  └─────────┘    └──────────────┘    └─────────┬─────────┘  │
│                                               │             │
│                                               ▼             │
│                                    ┌───────────────────┐   │
│                                    │ Agent Turn        │   │
│                                    │ (full harness     │   │
│                                    │  with tools)      │   │
│                                    └─────────┬─────────┘   │
│                                               │             │
│                         ┌─────────────────────┼──────────┐  │
│                         ▼                     ▼          │  │
│              ┌─────────────────┐   ┌─────────────────┐   │  │
│              │ HEARTBEAT_OK    │   │ Action/Message  │   │  │
│              │ (silent ack)    │   │ (notify user)   │   │  │
│              └─────────────────┘   └─────────────────┘   │  │
└─────────────────────────────────────────────────────────────┘
```

#### HEARTBEAT.md File

Agents read `HEARTBEAT.md` from their workspace on each heartbeat cycle:

```markdown
# Heartbeat Checklist

## Quick Checks
- Scan for any pending workflow approvals
- Check if any scheduled agents failed recently
- Review any alerts or notifications

## Periodic Tasks
- If idle for 4+ hours during work hours, send brief status update
- Summarize any completed background tasks

## Rules
- Do NOT notify for routine/expected events
- If nothing needs attention, reply with HEARTBEAT_OK
- Keep notifications concise and actionable
```

#### Heartbeat Configuration

```yaml
# In agent .prompt file
---
harness:
  max_steps: 50
  timeout: 30m
  heartbeat:
    enabled: true
    every: 30m              # Interval (supports: Nm, Nh)
    active_hours:           # Optional: only run during these hours
      start: "08:00"
      end: "22:00"
      timezone: "America/Chicago"  # or "local" or "UTC"
    session: main           # main (with context) or isolated (fresh)
    notify:
      channel: webhook      # Optional: where to send alerts
      url: "..."            # Webhook URL for notifications
---
```

#### Response Contract

- **`HEARTBEAT_OK`** - Nothing needs attention, suppress delivery
- **Any other response** - Deliver to configured notification channel
- Token detection: `HEARTBEAT_OK` at start or end of response triggers suppression

#### CLI Commands

```bash
# Check heartbeat status
stn harness heartbeat status --agent my-coder

# Trigger immediate heartbeat
stn harness heartbeat wake --agent my-coder

# View heartbeat history
stn harness heartbeat history --agent my-coder --limit 10
```

### 2. Memory Improvements

Enhanced memory system with workspace-based storage, automatic persistence, and structured organization.

#### Memory File Layout

```
~/.config/station/environments/<env>/agents/<agent>/
├── workspace/
│   ├── MEMORY.md              # Long-term curated memory
│   ├── HEARTBEAT.md           # Heartbeat checklist
│   └── memory/
│       ├── 2026-01-26.md      # Daily log (append-only)
│       ├── 2026-01-27.md      # Today's log
│       └── ...
```

#### Memory Types

| File | Purpose | Loaded When |
|------|---------|-------------|
| `MEMORY.md` | Curated long-term facts, preferences, decisions | Session start |
| `memory/YYYY-MM-DD.md` | Daily running notes, today + yesterday | Session start |
| `HEARTBEAT.md` | Heartbeat checklist | Heartbeat runs |

#### Automatic Memory Flush

Before a session ends or reaches context limits, trigger a silent agent turn to persist important information:

```
┌─────────────────────────────────────────────────────────────┐
│                 PRE-COMPACTION MEMORY FLUSH                 │
│                                                             │
│  Session approaching context limit                          │
│           │                                                 │
│           ▼                                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ Silent agent turn with flush prompt:                │   │
│  │ "Session ending. Store any durable memories to      │   │
│  │  memory/YYYY-MM-DD.md. Reply NO_REPLY when done."   │   │
│  └─────────────────────────────────────────────────────┘   │
│           │                                                 │
│           ▼                                                 │
│  Agent writes to memory files using workspace tools         │
│           │                                                 │
│           ▼                                                 │
│  Session compacts/ends normally                             │
└─────────────────────────────────────────────────────────────┘
```

#### Memory Flush Configuration

```yaml
harness:
  memory:
    enabled: true
    workspace_path: "./workspace"  # Relative to agent directory
    auto_flush:
      enabled: true
      threshold_tokens: 80000      # Flush when context reaches this
      prompt: |
        Session nearing end. Store any important notes, decisions,
        or context to memory/YYYY-MM-DD.md. Reply NO_REPLY when done.
```

#### Memory Tools

Agents have access to memory-specific tools:

```go
// Built-in memory tools (automatically available)
memory_read(path string) string           // Read memory file
memory_write(path string, content string) // Write/append to memory file
memory_list() []string                    // List memory files
```

#### Memory Injection

At session start, inject relevant memory into system context:

```go
type MemoryContext struct {
    LongTerm   string   // Contents of MEMORY.md
    Recent     []string // Contents of last 2 days' daily logs
    Heartbeat  string   // Contents of HEARTBEAT.md (for heartbeat runs)
}
```

### 3. Integration: Heartbeat + Memory

The heartbeat and memory systems work together:

1. **Heartbeat reads HEARTBEAT.md** from workspace
2. **Heartbeat has access to memory context** (recent daily logs, long-term memory)
3. **Heartbeat can update memory** (add notes to daily log, update MEMORY.md)
4. **Memory persists across heartbeat runs** (continuity)

#### Example Flow

```
Heartbeat runs at 10:30 AM
    │
    ├─► Reads HEARTBEAT.md checklist
    ├─► Reads memory/2026-01-27.md (today) + memory/2026-01-26.md (yesterday)
    ├─► Reads MEMORY.md (long-term context)
    │
    ├─► Agent checks: "Any pending approvals?"
    │   └─► Uses workflow tools to check
    │
    ├─► Agent finds: "Deployment approval pending for 2 hours"
    │   └─► Writes to memory/2026-01-27.md: "10:30 - Notified user about pending approval"
    │
    └─► Agent responds: "Deployment approval for prod-release pending since 8:30 AM"
        └─► Notification sent to configured channel
```

---

---

## Detailed Integration Map

### 1. Config Extension (Zero Breaking Changes)

**File: `pkg/harness/prompt/loader.go`**

Current `HarnessConfig` struct (lines 42-49):
```go
type HarnessConfig struct {
    Enabled    bool              `yaml:"enabled,omitempty"`
    MaxSteps   int               `yaml:"max_steps,omitempty"`
    Timeout    string            `yaml:"timeout,omitempty"`
    DoomLoop   *DoomLoopConfig   `yaml:"doom_loop,omitempty"`
    Compaction *CompactionConfig `yaml:"compaction,omitempty"`
    Progress   *ProgressConfig   `yaml:"progress,omitempty"`
}
```

**Add (no changes to existing fields):**
```go
type HarnessConfig struct {
    // ... existing fields unchanged ...
    Heartbeat  *HeartbeatConfig  `yaml:"heartbeat,omitempty"`  // NEW
}

type HeartbeatConfig struct {
    Enabled     bool              `yaml:"enabled,omitempty"`
    Every       string            `yaml:"every,omitempty"`       // "30m", "1h"
    ActiveHours *ActiveHoursConfig `yaml:"active_hours,omitempty"`
    Session     string            `yaml:"session,omitempty"`     // "main" or "isolated"
}

type ActiveHoursConfig struct {
    Start    string `yaml:"start,omitempty"`    // "08:00"
    End      string `yaml:"end,omitempty"`      // "20:00"
    Timezone string `yaml:"timezone,omitempty"` // "local", "UTC", or IANA
}
```

**DX Impact**: Existing `.prompt` files work unchanged. Heartbeat is opt-in.

---

### 2. Memory Middleware Extension

**File: `pkg/harness/memory/middleware.go`**

Current `LoadMemory()` (lines 29-50) reads files but has **no write capability**.

**Add `WriteMemory()` method:**
```go
// WriteMemory appends content to a memory file
func (m *MemoryMiddleware) WriteMemory(source string, content string) error {
    expandedPath := expandPath(source)

    // Ensure directory exists
    dir := filepath.Dir(expandedPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create memory directory: %w", err)
    }

    // Append to file (create if not exists)
    f, err := os.OpenFile(expandedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("failed to open memory file: %w", err)
    }
    defer f.Close()

    if _, err := f.WriteString(content + "\n"); err != nil {
        return fmt.Errorf("failed to write to memory file: %w", err)
    }

    m.logger.Debug("wrote to memory", "source", expandedPath, "bytes", len(content))
    return nil
}
```

**Extend `DefaultMemorySources()` (lines 110-116):**
```go
func DefaultMemorySources(envPath string, agentWorkspace string) []string {
    sources := []string{
        "~/.config/station/AGENTS.md",
        filepath.Join(envPath, "memory", "AGENTS.md"),
        ".station/AGENTS.md",
    }

    // Add workspace-based memory if workspace provided
    if agentWorkspace != "" {
        sources = append(sources,
            filepath.Join(agentWorkspace, "MEMORY.md"),
            filepath.Join(agentWorkspace, "memory", time.Now().Format("2006-01-02")+".md"),
            filepath.Join(agentWorkspace, "memory", time.Now().AddDate(0, 0, -1).Format("2006-01-02")+".md"),
        )
    }

    return sources
}
```

**DX Impact**: Existing memory loading works unchanged. New workspace sources are additive.

---

### 3. Executor Hooks

**File: `pkg/harness/executor.go`**

**Hook 1: Memory flush before cleanup (around line 231)**

Current:
```go
// Final cleanup
e.cleanup(ctx, result)
```

**Add:**
```go
// Memory flush before cleanup
if e.config.Harness != nil && e.config.Harness.Compaction != nil {
    e.flushMemoryIfNeeded(ctx, result)
}

// Final cleanup
e.cleanup(ctx, result)
```

**Hook 2: Heartbeat emission at step complete (around line 553)**

Current:
```go
e.emitStepComplete(ctx, step, toolResults, stepTokens, stepDuration)
```

**Add (wrapper approach):**
```go
e.emitStepComplete(ctx, step, toolResults, stepTokens, stepDuration)

// Emit heartbeat if configured
if e.heartbeatEnabled() {
    e.emitHeartbeat(ctx, step, totalTokens)
}
```

**DX Impact**: No changes to existing execution flow. Heartbeat is conditional.

---

### 4. Session State Storage

**File: `pkg/harness/session/history_store.go`**

Current `SessionHistory` struct already has `Metadata map[string]interface{}` (line 44) - **unused**.

**Reuse for heartbeat state:**
```go
// In executor or heartbeat service
func (e *Executor) updateHeartbeatState(session *SessionHistory, step int, tokens int) {
    if session.Metadata == nil {
        session.Metadata = make(map[string]interface{})
    }

    session.Metadata["heartbeat"] = map[string]interface{}{
        "last_step":   step,
        "last_tokens": tokens,
        "last_update": time.Now().Unix(),
        "status":      "running",
    }
}
```

**DX Impact**: Zero changes to SessionHistory struct. Uses existing field.

---

### 5. Scheduler Integration (Optional Enhancement)

**File: `internal/services/scheduler.go`**

Current scheduled execution (around line 275):
```go
runCtx := context.Background()
runResult, err := s.agentService.ExecuteAgentWithRunID(runCtx, run.ID, run.AgentID, task, envName, variables)
```

**Wrap for heartbeat monitoring (optional):**
```go
runCtx := context.Background()

// Start heartbeat monitor for long-running scheduled agents
heartbeatCtx, cancelHeartbeat := context.WithCancel(runCtx)
defer cancelHeartbeat()

if agent.HeartbeatEnabled() {
    go s.heartbeatMonitor(heartbeatCtx, run.ID)
}

runResult, err := s.agentService.ExecuteAgentWithRunID(runCtx, run.ID, run.AgentID, task, envName, variables)
```

**DX Impact**: Scheduled agents gain heartbeat monitoring without config changes.

---

### 6. HEARTBEAT.md Token Handling

**New file: `pkg/harness/heartbeat/tokens.go`**

```go
package heartbeat

import "strings"

const (
    HeartbeatOKToken = "HEARTBEAT_OK"
    NoReplyToken     = "NO_REPLY"
)

// ShouldSuppressDelivery checks if response indicates no notification needed
func ShouldSuppressDelivery(response string) bool {
    trimmed := strings.TrimSpace(response)

    // Check start
    if strings.HasPrefix(trimmed, HeartbeatOKToken) {
        return true
    }

    // Check end
    if strings.HasSuffix(trimmed, HeartbeatOKToken) {
        return true
    }

    // Check NO_REPLY (for memory flush)
    if trimmed == NoReplyToken {
        return true
    }

    return false
}

// StripHeartbeatToken removes HEARTBEAT_OK from response
func StripHeartbeatToken(response string) string {
    response = strings.TrimPrefix(response, HeartbeatOKToken)
    response = strings.TrimSuffix(response, HeartbeatOKToken)
    return strings.TrimSpace(response)
}
```

---

## Implementation Plan

### Phase 1: Memory Middleware Enhancement (2-3 days)

**Goal**: Add write capability and workspace-based memory

| Task | File | Change |
|------|------|--------|
| Add `WriteMemory()` method | `pkg/harness/memory/middleware.go` | New method (~20 lines) |
| Add `DailyLogPath()` helper | `pkg/harness/memory/middleware.go` | New method (~5 lines) |
| Extend `DefaultMemorySources()` | `pkg/harness/memory/middleware.go:110` | Add workspace paths |
| Create workspace memory dirs | `pkg/harness/workspace/host.go` | Add to `Initialize()` |

**No changes to existing behavior** - all additive.

### Phase 2: Memory Flush Integration (2-3 days)

**Goal**: Automatic memory persistence before session ends

| Task | File | Change |
|------|------|--------|
| Add `flushMemoryIfNeeded()` | `pkg/harness/executor.go` | New method (~30 lines) |
| Hook before `cleanup()` | `pkg/harness/executor.go:231` | Insert call |
| Add `NO_REPLY` token handling | `pkg/harness/stream/publisher.go` | Add suppression check |
| Add flush config | `pkg/harness/prompt/loader.go:57-63` | Extend `CompactionConfig` |

**Minimal touchpoints** - one insert, one config extension.

### Phase 3: Heartbeat Config & Types (1-2 days)

**Goal**: Define heartbeat configuration in dotprompt

| Task | File | Change |
|------|------|--------|
| Add `HeartbeatConfig` struct | `pkg/harness/prompt/loader.go` | New struct (~15 lines) |
| Add `ActiveHoursConfig` struct | `pkg/harness/prompt/loader.go` | New struct (~5 lines) |
| Add `Heartbeat` to `HarnessConfig` | `pkg/harness/prompt/loader.go:42` | One field |
| Add getter methods | `pkg/harness/prompt/loader.go` | `GetHeartbeatInterval()`, `IsHeartbeatEnabled()` |

**Zero breaking changes** - optional config only.

### Phase 4: Heartbeat Runner (3-4 days)

**Goal**: Core heartbeat execution loop

| Task | File | Change |
|------|------|--------|
| Create `HeartbeatService` | `pkg/harness/heartbeat/service.go` | New file (~150 lines) |
| Create `tokens.go` | `pkg/harness/heartbeat/tokens.go` | New file (~40 lines) |
| Integrate with scheduler | `internal/services/scheduler.go` | Optional wrapper |
| Add heartbeat state to session | `pkg/harness/session/` | Use existing `Metadata` field |

**New package, minimal integration points**.

### Phase 5: CLI & Polish (2-3 days)

**Goal**: CLI commands and documentation

| Task | File | Change |
|------|------|--------|
| Add `stn harness heartbeat` command | `cmd/main/harness_commands.go` | New subcommand group |
| Add heartbeat status display | `cmd/main/harness_commands.go` | `heartbeat status` |
| Add manual wake command | `cmd/main/harness_commands.go` | `heartbeat wake` |
| Update HARNESS_CLI.md | `docs/harness/HARNESS_CLI.md` | Add heartbeat section |
| Add example `.prompt` files | `docs/examples/` | Heartbeat examples |

**Total estimated: ~10-15 days** (conservative, can parallelize)

---

## DX Preservation Checklist

### What Stays the Same

| Aspect | Guarantee |
|--------|-----------|
| **Existing `.prompt` files** | Work unchanged, heartbeat is opt-in |
| **CLI commands** | All existing commands unchanged |
| **Memory injection** | Existing AGENTS.md loading unchanged |
| **Scheduler behavior** | Cron scheduling works as before |
| **Session persistence** | Same storage format, `Metadata` field reused |
| **Tool execution** | No changes to tool calling interface |

### Opt-In Defaults

All new features default to **disabled**:

```yaml
# Minimal agent - works exactly like before
---
model: gpt-4o-mini
harness:
  max_steps: 20
---
```

```yaml
# Full agent with new features - explicit opt-in
---
model: gpt-4o-mini
harness:
  max_steps: 20
  heartbeat:
    enabled: true      # Must explicitly enable
    every: 30m
  compaction:
    memory_flush: true # Must explicitly enable
---
```

### Progressive Enhancement

Users can adopt features incrementally:

1. **Week 1**: Just use workspace memory files manually
2. **Week 2**: Enable auto-flush before session ends
3. **Week 3**: Add HEARTBEAT.md for simple monitoring
4. **Week 4**: Enable full heartbeat with active hours

### Error Handling

New features fail gracefully:

```go
// Memory flush fails? Log and continue
if err := e.flushMemory(ctx); err != nil {
    e.logger.Warn("memory flush failed, continuing", "error", err)
}

// Heartbeat can't read file? Skip this cycle
if _, err := os.ReadFile(heartbeatPath); err != nil {
    e.logger.Debug("HEARTBEAT.md not found, skipping")
    return nil
}
```

---

## Configuration Reference

### Full Agent Configuration

```yaml
---
metadata:
  name: "ops-agent"
  description: "Operations monitoring agent"
  tags: ["ops", "monitoring"]
model: gpt-4o-mini
max_steps: 25

harness:
  timeout: 15m

  heartbeat:
    enabled: true
    every: 30m
    active_hours:
      start: "08:00"
      end: "20:00"
      timezone: "local"
    session: main           # main or isolated
    notify:
      channel: webhook
      url: "https://hooks.slack.com/..."

  memory:
    enabled: true
    auto_flush:
      enabled: true
      threshold_tokens: 80000

tools:
  - read
  - write
  - bash
  - memory_read
  - memory_write
---
```

### Heartbeat-Specific Prompt Injection

When running a heartbeat turn, the system injects:

```
[HEARTBEAT RUN - {{timestamp}}]

Your heartbeat checklist:
{{contents of HEARTBEAT.md}}

Recent context:
{{contents of memory/today.md and memory/yesterday.md}}

Long-term memory:
{{contents of MEMORY.md}}

Instructions:
- Follow your checklist strictly
- If nothing needs attention, reply with HEARTBEAT_OK
- Keep any notifications concise and actionable
```

---

## Success Metrics

1. **Heartbeat Reliability**: 99%+ of scheduled heartbeats execute successfully
2. **Memory Persistence**: Zero data loss from session timeouts
3. **Response Quality**: <10% false positive notifications (unnecessary alerts)
4. **Performance**: Heartbeat execution adds <5s to normal agent turn latency

---

## Out of Scope

The following are explicitly NOT included in this PRD:

1. **Model/thinking overrides per heartbeat** - Station uses consistent model config
2. **Vector/semantic memory search** - May be added in future PRD
3. **Multi-channel notifications** - Single webhook channel for now
4. **Session memory indexing** - File-based only for this iteration
5. **Memory deduplication/compaction** - Manual curation for now

---

## Open Questions

1. **Heartbeat session isolation**: Should heartbeats always run in main session, or allow isolated mode for specific use cases?

2. **Memory file size limits**: Should we enforce limits on memory file sizes to prevent prompt bloat?

3. **Heartbeat failure handling**: What happens if a heartbeat run fails (error, timeout)? Retry? Skip?

4. **Memory file format**: Plain markdown, or structured YAML frontmatter + markdown body?

---

## References

- [Agentic Harness PRD](./PRD_AGENTIC_HARNESS.md)
- [Harness CLI Documentation](../harness/HARNESS_CLI.md)
- [ClawdBot Heartbeat](https://docs.clawd.bot/gateway/heartbeat) (inspiration)
- [ClawdBot Memory](https://docs.clawd.bot/concepts/memory) (inspiration)
