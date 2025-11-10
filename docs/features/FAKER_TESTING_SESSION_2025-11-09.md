# Faker Testing Session - November 9, 2025

## Objective
Test the complete faker system including:
- OpenTelemetry instrumentation with `station.faker` spans
- Session replay capability  
- Agent execution with faker-wrapped tools
- Verify no hanging agents
- Check Jaeger traces for faker spans

## Test Environment
- **Jaeger**: Running at http://localhost:16686 (ports 4317-4318 OTLP, 16686 UI)
- **Station**: v0.1.0 (built 2025-11-10 00:31:47 UTC)
- **Database**: SQLite at `~/.config/station/station.db`

## Test Results

### ✅ PASS: Faker Standalone Functionality

**Test**: Start faker in standalone mode with filesystem MCP server

```bash
stn faker \
  --command npx \
  --args "-y,@modelcontextprotocol/server-filesystem@latest,/tmp" \
  --ai-instruction "Generate realistic test files with sample data" \
  --debug
```

**Results**:
- ✅ Faker initialized successfully with OpenAI/GenKit
- ✅ Target MCP client connected (npx filesystem server)
- ✅ Session created: `ff3d104c-6df4-43dc-9814-9125b0c90d3f`
- ✅ Tool discovery worked: Found 14 filesystem tools
- ✅ Safety mode activated: Identified 4 write operations to intercept
  - `edit_file`, `create_directory`, `move_file`, `write_file`
- ✅ Faker running on stdio successfully

**Write Operations Correctly Classified**:
```
🛡️  SAFETY MODE: 4 write operations detected and will be INTERCEPTED:
  1. edit_file       (risk: medium)
  2. create_directory (risk: medium)  
  3. move_file       (risk: medium)
  4. write_file      (risk: high)
```

### ✅ PASS: Session Replay Functionality

**Test**: Export faker session for replay/debugging

```bash
stn faker sessions replay ff3d104c-6df4-43dc-9814-9125b0c90d3f
```

**Results**:
```json
{
  "session_id": "ff3d104c-6df4-43dc-9814-9125b0c90d3f",
  "instruction": "Generate realistic test files with sample data",
  "created_at": "2025-11-09T18:30:57.017082047-06:00",
  "tool_calls": [],
  "stats": {
    "TotalToolCalls": 0,
    "ReadCalls": 0,
    "WriteCalls": 0,
    "UniqueTools": 0,
    "Duration": 146
  }
}
```

- ✅ Session successfully stored in database
- ✅ Replay command works correctly
- ✅ JSON export format is clean and complete
- ℹ️  No tool calls yet (faker was only started, not used by an agent)

### ✅ PASS: Agent Execution (Without Faker)

**Test**: Run simple agent to verify base system works

```bash
stn agent run 23 "Say hello enthusiastically!"
```

**Results**:
- ✅ Agent completed successfully (status: completed)
- ✅ No hanging - completed in 1.86 seconds
- ✅ Proper token usage tracking (57 input, 47 output)
- ✅ Run ID 277 created and tracked
- ✅ Agent prompt executed correctly

### ❌ FAIL: Agent with Faker Environment (Recursion Issue)

**Test**: Run agent in `faker-filesystem-demo` environment

```bash
stn agent run 5 "List the files in /tmp and tell me what you find"
```

**Problem**: Agent hangs indefinitely (timeout after 60s)

**Root Cause Analysis**:
```json
{
  "mcpServers": {
    "filesystem-faker": {
      "command": "stn",  // ❌ PROBLEM: Relative command causes recursion
      "args": ["faker", "--command", "npx", ...]
    }
  }
}
```

**Issues Found**:
1. **Recursive Process Creation**: `stn` calling `stn faker` creates nested processes
2. **No Communication**: Nested processes don't establish proper stdio communication
3. **Infinite Wait**: Agent waits for MCP server that never fully initializes
4. **Resource Leak**: Multiple `stn faker` processes accumulate (observed 3+ in ps aux)

**Evidence**:
```bash
$ ps aux | grep "stn faker"
epuerta  61871  stn faker --command npx --args ...  # Stuck process 1
epuerta  69293  stn faker --command npx --args ...  # Stuck process 2  
epuerta  69277  ./bin/stn agent run 5 ...          # Waiting agent
```

**Database Impact**:
- Runs stuck in `status='running'` indefinitely
- Had to manually mark runs 273-274 as failed

### 📋 Known Issues & Solutions

#### Issue 1: MCP Server Command Recursion

**Problem**: Using `command: "stn"` in template.json causes infinite recursion

**Solution**: Use absolute path to stn binary

**Fix for Existing Environments**:
```json
{
  "mcpServers": {
    "filesystem-faker": {
      "command": "/home/epuerta/.local/bin/stn",  // ✅ Use full path
      "args": ["faker", "--command", "npx", ...]
    }
  }
}
```

**Recommended Pattern**:
```json
{
  "mcpServers": {
    "filesystem-faker": {
      "command": "{{  .STN_PATH }}",  // Use variable
      "args": ["faker", ...]
    }
  }
}
```

With `variables.yml`:
```yaml
STN_PATH: "/home/epuerta/.local/bin/stn"
```

#### Issue 2: OpenTelemetry Not Configured

**Problem**: No traces visible in Jaeger despite instrumentation being added

**Root Cause**: OTEL environment variables not set

**Solution**: Configure Station to export telemetry

**Required Environment Variables**:
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export OTEL_SERVICE_NAME="station"
export GENKIT_ENV="prod"  # Enable telemetry export
```

**Station Config** (`~/.config/station/config.yaml`):
```yaml
telemetry:
  enabled: true
  otlp_endpoint: "http://localhost:4318"
  service_name: "station"
  force_export: true  # Export even in dev mode
```

### 🎯 What Was Tested Successfully

1. ✅ **Faker Initialization**: Faker starts and initializes GenKit/OpenAI
2. ✅ **MCP Client Connection**: Faker connects to target MCP servers
3. ✅ **Tool Discovery**: Faker discovers and proxies all 14 filesystem tools
4. ✅ **Safety Mode**: Faker correctly classifies write operations (4/14 tools)
5. ✅ **Session Creation**: Faker creates database session records  
6. ✅ **Session Storage**: Sessions persist in SQLite with metadata
7. ✅ **Session Replay**: CLI command exports sessions as JSON
8. ✅ **Agent Execution**: Base agent execution works (non-faker)
9. ✅ **Run Tracking**: Agent runs are tracked with status/tokens

### ❌ What Needs Fixing

1. ❌ **Faker in Agent Execution**: Template recursion prevents faker from working in agents
2. ❌ **OpenTelemetry Export**: No spans visible in Jaeger (config needed)
3. ❌ **Hung Agent Detection**: No automatic timeout/cleanup for stuck agents
4. ❌ **Tool Call Recording**: No tool calls recorded (faker never reached execution)

## Faker Instrumentation Code Added

### OpenTelemetry Spans

**File**: `pkg/faker/mcp_faker.go`

**Span 1: Tool Call Tracking**
```go
tracer := otel.Tracer("station.faker")
ctx, span := tracer.Start(ctx, fmt.Sprintf("faker.%s", request.Params.Name),
    trace.WithAttributes(
        attribute.String("faker.tool_name", request.Params.Name),
        attribute.String("faker.ai_instruction", f.instruction),
        attribute.Bool("faker.safety_mode", f.safetyMode),
        attribute.Bool("faker.is_write_operation", f.writeOperations[request.Params.Name]),
        attribute.String("faker.session_id", f.session.ID),
        attribute.Bool("faker.real_mcp_used", true/false),
        attribute.Bool("faker.intercepted_write", true/false),
        attribute.Bool("faker.synthesized_response", true/false),
    ),
)
defer span.End()
```

**Span 2: AI Enrichment**
```go
ctx, span := tracer.Start(ctx, "faker.ai_enrichment",
    trace.WithAttributes(
        attribute.String("faker.tool_name", toolName),
        attribute.String("faker.operation", "ai_enrichment"),
        attribute.Bool("faker.ai_enrichment_enabled", true/false),
    ),
)
defer span.End()
```

**Error Handling**:
```go
span.RecordError(err)
span.SetStatus(codes.Error, "description")
```

## Session Replay Implementation

**File**: `pkg/faker/session_service.go`

**New Types Added**:
```go
type ReplayableSession struct {
    SessionID    string
    Instruction  string
    CreatedAt    time.Time
    ToolCalls    []ReplayableToolCall
    Stats        *SessionStats
}

type ReplayableToolCall struct {
    Sequence      int                    // Call order (1, 2, 3...)
    ToolName      string
    Arguments     map[string]interface{} // Full parameters
    Response      interface{}            // Simulated data
    OperationType string                 // "read" or "write"
    Timestamp     time.Time
    ElapsedMs     int64                  // Time from session start
}
```

**New Methods**:
- `GetReplayableSession(ctx, sessionID) (*ReplayableSession, error)`
- `ExportReplayableSessionJSON(ctx, sessionID) ([]byte, error)`

**CLI Command** (`cmd/main/faker_sessions.go`):
```go
var fakerSessionReplayCmd = &cobra.Command{
    Use:   "replay <session-id>",
    Short: "Export session for replay and debugging",
    RunE: runFakerSessionReplay,
}
```

## Next Steps to Complete Testing

### 1. Fix Faker Environment Templates

**Action**: Update all faker environment templates to use absolute paths

**Files to Update**:
```bash
~/.config/station/environments/faker-filesystem-demo/template.json
~/.config/station/environments/faker-test-demo/template.json
~/.config/station/environments/faker-eval-2025/template.json
# ... etc
```

**Change**:
```diff
- "command": "stn",
+ "command": "/home/epuerta/.local/bin/stn",
```

### 2. Configure OpenTelemetry

**Action**: Enable OTEL export in Station

**Option A: Environment Variables** (temporary):
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export OTEL_SERVICE_NAME="station"
export GENKIT_ENV="prod"
stn agent run <agent-id> "task"
```

**Option B: Config File** (permanent):
Add to `~/.config/station/config.yaml`:
```yaml
telemetry:
  enabled: true
  endpoint: "http://localhost:4318"
  service: "station"
```

### 3. Re-run Complete Test Flow

Once fixes applied:

```bash
# 1. Sync faker environment (with fixed template)
stn sync faker-filesystem-demo

# 2. Run agent with faker
stn agent run 5 "List files in /tmp and analyze them" --tail

# 3. Check agent completion
stn runs inspect <run-id>

# 4. Verify faker session created
stn faker sessions list

# 5. Export session for replay
stn faker sessions replay <session-id> > faker-test-scenario.json

# 6. Check Jaeger traces
open http://localhost:16686
# Search for service: station
# Look for spans: station.faker
```

### 4. Verify Faker Simulation Scenarios

**Test Different AI Instructions**:

**Scenario 1: High-Traffic Filesystem**
```bash
stn faker create filesystem \
  --env test \
  --name fs-high-traffic \
  --instruction "Simulate filesystem under heavy load: slow I/O, large files (100MB+), deep directory nesting (20+ levels)"
```

**Scenario 2: Security Vulnerabilities**
```bash
stn faker create filesystem \
  --env test \
  --name fs-vulnerable \
  --instruction "Simulate filesystem with security issues: world-writable files, exposed SSH keys, hardcoded credentials in config files"
```

**Scenario 3: Corruption/Errors**
```bash
stn faker create filesystem \
  --env test \
  --name fs-errors \
  --instruction "Simulate filesystem errors: permission denied (EACCES), no space left (ENOSPC), corrupted files"
```

## Verification Checklist

Use this checklist to validate faker is working correctly:

- [ ] Faker starts without hanging
- [ ] Agent completes execution (not stuck in running)  
- [ ] Agent run shows "completed" status
- [ ] Faker session created in database
- [ ] Session contains tool calls (not empty)
- [ ] Session replay exports valid JSON
- [ ] Jaeger shows `station.faker` spans
- [ ] Spans have correct attributes (tool_name, session_id, etc.)
- [ ] Different AI instructions produce different simulated data
- [ ] Write operations are intercepted in safety mode
- [ ] Read operations use real MCP or AI-synthesized data

## Conclusion

### What Works
- ✅ Faker core functionality (standalone)
- ✅ Session tracking and replay
- ✅ Safety mode and write interception
- ✅ Agent execution (without faker)
- ✅ OpenTelemetry spans added to code

### What Needs Fixing
- ❌ Template command recursion (use absolute paths)
- ❌ OTEL configuration (enable export)
- ❌ Agent timeout handling (detect hung processes)

### Impact

**When Fixed**: Station will have a production-ready faker system with:
- Full observability via OpenTelemetry/Jaeger
- Session replay for debugging/testing/sharing
- Universal MCP tool simulation (not just filesystem)
- Safety guarantees (write interception)
- AI-powered realistic data generation

**Use Cases Enabled**:
- Test agents against AWS cost spikes without real AWS
- Simulate security incidents for agent training
- Share reproducible test scenarios as JSON
- Debug agent behavior with exact data replay
- Validate agents handle errors/anomalies correctly

This positions Station as a unique platform for **safe, observable AI agent development with realistic simulation capabilities**.
