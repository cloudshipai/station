# Heartbeat and Memory System

Station's harness includes a proactive scheduling system (heartbeat) and session memory persistence that enables long-running, stateful agent workflows.

## Overview

| Feature | Description |
|---------|-------------|
| **Heartbeat** | Periodic agent polling with configurable intervals and active hours |
| **Memory Flush** | Automatic persistence of session insights before cleanup |
| **Workspace Memory** | Markdown-based memory files (MEMORY.md, daily logs) |

## Heartbeat Service

### Configuration

Add heartbeat configuration to your agent's `.prompt` file:

```yaml
---
harness:
  enabled: true
  max_steps: 50
  timeout: 30m

  heartbeat:
    enabled: true
    every: 30m                    # Check every 30 minutes
    active_hours:
      start: "08:00"              # 24h format
      end: "20:00"
      timezone: "America/New_York" # IANA timezone or "local"/"UTC"
    session: main                  # "main" (shared) or "isolated"
    notify:
      channel: webhook
      url: "https://ntfy.sh/my-alerts"
---
```

### How It Works

1. **Scheduling**: The heartbeat service runs at the configured interval
2. **Active Hours**: Checks are skipped outside the configured time window
3. **HEARTBEAT.md**: The agent receives the contents of `HEARTBEAT.md` as its prompt
4. **Response Tokens**:
   - `HEARTBEAT_OK` - Nothing to report, stay silent
   - Any other response - Notification is sent

### HEARTBEAT.md Template

```markdown
# Heartbeat Checklist

## Quick Checks
- Review any pending tasks or notifications
- Check for alerts or issues that need attention

## Rules
- If nothing needs attention, reply with HEARTBEAT_OK
- Keep notifications concise and actionable
```

### Session Modes

| Mode | Description |
|------|-------------|
| `main` | Heartbeats run in the main session, can see full context |
| `isolated` | Each heartbeat gets a fresh session (no history) |

## Memory System

### Workspace Structure

When workspace memory is initialized, Station creates:

```
workspace/
├── MEMORY.md           # Long-term facts, preferences, decisions
├── HEARTBEAT.md        # Heartbeat checklist for the agent
└── memory/
    ├── 2024-01-27.md   # Today's session log
    └── 2024-01-26.md   # Yesterday's session log
```

### Memory Flush

When `memory_flush` is enabled, Station automatically persists session summaries before cleanup:

```yaml
harness:
  compaction:
    enabled: true
    memory_flush: true    # Enable automatic memory persistence
```

**What gets persisted:**
- Step count and token usage
- Session duration
- Truncated outcome/response (first 500 chars)

**Daily log entry format:**
```markdown
## 2024-01-27 14:30:00
### Session Summary
- Steps: 5, Tokens: 2500
- Duration: 45s
- Outcome: Successfully created the config file and validated...
```

### Memory Sources

Station loads memory from multiple sources (in order):

1. `~/.config/station/AGENTS.md` - Global user preferences
2. `{env}/memory/AGENTS.md` - Environment-specific memory
3. `.station/AGENTS.md` - Project-specific memory
4. `{workspace}/MEMORY.md` - Workspace long-term memory
5. `{workspace}/memory/{today}.md` - Today's session log
6. `{workspace}/memory/{yesterday}.md` - Yesterday's session log

### Memory Guidelines (injected into agent)

The agent receives these guidelines with the memory context:

```markdown
**When to update memory (use edit_file on AGENTS.md):**
- User explicitly asks you to remember something
- User describes role or behavior preferences
- User provides feedback on your work that should persist
- You discover new patterns or preferences

**When NOT to update memory:**
- Temporary or transient information
- One-time task requests
- Simple questions that don't reveal lasting preferences

**Security:**
- NEVER store API keys, passwords, or credentials in memory files
```

## Configuration Reference

### HeartbeatConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable heartbeat service |
| `every` | string | "30m" | Interval between checks (e.g., "30m", "1h") |
| `active_hours` | object | - | Time window for checks |
| `session` | string | "main" | Session mode: "main" or "isolated" |
| `notify` | object | - | Notification configuration |

### ActiveHoursConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `start` | string | - | Start time in 24h format (e.g., "08:00") |
| `end` | string | - | End time in 24h format (e.g., "20:00") |
| `timezone` | string | "local" | IANA timezone or "local"/"UTC" |

### NotifyConfig

| Field | Type | Description |
|-------|------|-------------|
| `channel` | string | Notification channel (currently: "webhook") |
| `url` | string | Webhook URL for notifications |

### MemoryConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sources` | []string | - | Custom memory source paths |
| `workspace_path` | string | - | Workspace path for memory files |
| `auto_flush` | bool | false | Enable automatic memory flush |

## Programmatic Usage

### HeartbeatService

```go
import "station/pkg/harness/heartbeat"

// Create service
config := &prompt.HeartbeatConfig{
    Enabled: true,
    Every:   "30m",
    ActiveHours: &prompt.ActiveHoursConfig{
        Start:    "08:00",
        End:      "20:00",
        Timezone: "UTC",
    },
}
svc := heartbeat.NewService(config, workspacePath)

// Set check function (called on each heartbeat)
svc.SetCheckFunction(func(ctx context.Context, prompt string) (string, error) {
    // Run agent with heartbeat prompt
    result, err := executor.Execute(ctx, "heartbeat", prompt, tools)
    return result.Response, err
})

// Start/stop service
svc.Start(ctx)
defer svc.Stop()

// Manual trigger
response, err := svc.TriggerNow(ctx)
if heartbeat.IsHeartbeatOK(response) {
    // Nothing to report
} else if heartbeat.ShouldNotify(response) {
    // Send notification
}
```

### MemoryMiddleware

```go
import "station/pkg/harness/memory"

// Create middleware with workspace
backend := &memory.FSBackend{}
sources := memory.DefaultMemorySourcesWithWorkspace(envPath, workspacePath)
mw := memory.NewMemoryMiddlewareWithWorkspace(backend, sources, workspacePath)

// Initialize workspace (creates MEMORY.md, HEARTBEAT.md)
mw.InitializeWorkspaceMemory()

// Inject into executor
executor := harness.NewAgenticExecutor(
    genkitApp,
    harnessConfig,
    agentConfig,
    harness.WithMemoryMiddleware(mw),
)

// Flush session memory (called automatically if enabled)
mw.FlushSessionMemory("Session completed task X with result Y")
```

## Token Reference

| Token | Constant | Usage |
|-------|----------|-------|
| `HEARTBEAT_OK` | `heartbeat.HeartbeatOKToken` | Agent has nothing to report |
| `NO_REPLY` | `heartbeat.NoReplyToken` | Memory flush, no user response needed |

## Best Practices

1. **Heartbeat Intervals**: Use 30m-1h intervals to avoid excessive API costs
2. **Active Hours**: Configure to match your working hours to avoid off-hours alerts
3. **Memory Flush**: Enable for long-running sessions to preserve context
4. **HEARTBEAT.md**: Keep checklist focused and actionable
5. **Memory Security**: Never store credentials in memory files

## See Also

- [Harness CLI Reference](./HARNESS_CLI.md)
- [PRD: Heartbeat and Memory](../features/PRD_HEARTBEAT_AND_MEMORY.md)
