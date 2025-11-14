# OTEL Integration & Testing Implementation Plan

**Section**: 3 - OpenTelemetry Tracing Integration
**Owner**: Claude Agent A
**Timeline**: 1-2 weeks
**Status**: In Progress

---

## 🎯 Goals

1. **Fix GenKit v1.0.1 OTEL Integration** - Export spans to external collectors
2. **Enable Team Integration** - Easy hookup to existing observability stacks
3. **Programmatic Testing** - Automated verification of OTEL export
4. **Developer Experience** - One-command testing with Jaeger

---

## 📊 Current State Analysis

### ✅ What's Working
- `GenkitTelemetryClient` captures execution data from GenKit traces
- Stores tool calls, token usage, performance metrics in Station DB
- OTLP HTTP exporter configured correctly
- Global TracerProvider set via `otel.SetTracerProvider()`

### ⚠️ What's Broken
- **Line 97-99 in `otel_plugin.go`**: `RegisterSpanProcessor` commented out (GenKit v1.0.1 removed this API)
- No spans exported to external collectors (Jaeger, Tempo, Datadog)
- No test coverage for OTEL export
- No documentation for team integration

### 🔍 Root Cause
GenKit v1.0.1 changed telemetry architecture:
- Removed `genkit.RegisterSpanProcessor()` API
- Internal telemetry system captures spans via `github.com/firebase/genkit/go/core/tracing`
- Station's `GenkitTelemetryClient.Save()` receives `*tracing.Data` but doesn't export to OTLP

---

## 🏗️ Implementation Strategy

### Phase 1: Research & Fix (Days 1-3)

**Task 1.1: Research GenKit v1.0.1 Telemetry**
- Read GenKit v1.0.1 source code for telemetry patterns
- Understand `github.com/firebase/genkit/go/core/tracing` package
- Find how to bridge GenKit traces to OTLP exporter

**Task 1.2: Fix `otel_plugin.go`**
Two approaches to investigate:

**Approach A: Manual Span Creation**
```go
// In agent_execution_engine.go
func (e *AgentExecutionEngine) Execute(ctx context.Context, agent Agent) {
    tracer := otel.Tracer("station.agent")
    ctx, span := tracer.Start(ctx, "agent.execute")
    defer span.End()

    span.SetAttributes(
        attribute.String("agent.name", agent.Name),
        attribute.String("agent.id", fmt.Sprint(agent.ID)),
    )

    // Execute agent...
}
```

**Approach B: Bridge GenKit Traces to OTEL**
```go
// In genkit_telemetry_client.go
func (c *GenkitTelemetryClient) Save(ctx context.Context, trace *tracing.Data) error {
    // Convert GenKit trace to OTEL spans
    tracer := otel.Tracer("station.genkit")

    for _, span := range trace.Spans {
        // Create OTEL span from GenKit span
        ctx, otelSpan := tracer.Start(ctx, span.DisplayName)
        otelSpan.SetAttributes(...)
        otelSpan.End()
    }

    // Also save to Station DB (existing code)
    return c.updateAgentRunWithTelemetry(ctx, executionData)
}
```

**Decision**: Start with Approach A (simpler, proven pattern)

**Task 1.3: Add Span Context Propagation**
```go
// Ensure context carries trace info through execution chain
ctx = trace.ContextWithSpan(ctx, span)
```

---

### Phase 2: Docker Compose Testing Stack (Days 4-5)

**Task 2.1: Create `docker-compose.otel.yml`**
```yaml
version: '3.8'

services:
  # Jaeger all-in-one (collector + UI)
  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: station-jaeger
    ports:
      - "16686:16686"  # Jaeger UI
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    networks:
      - station-otel

  # Station server with OTEL enabled
  station:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: station-server
    depends_on:
      - jaeger
    environment:
      - OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
      - OTEL_SERVICE_NAME=station
      - OTEL_SERVICE_VERSION=dev
      - GENKIT_ENV=production  # Enable export
    ports:
      - "8080:8080"  # Station API
      - "2222:2222"  # Station SSH
    networks:
      - station-otel
    volumes:
      - ./test-data:/data

networks:
  station-otel:
    driver: bridge
```

**Task 2.2: Add Helper Scripts**
```bash
# scripts/otel-stack-up.sh
#!/bin/bash
docker-compose -f docker-compose.otel.yml up -d
echo "✅ OTEL stack started"
echo "📊 Jaeger UI: http://localhost:16686"
echo "🔌 OTLP HTTP: http://localhost:4318"

# scripts/otel-stack-down.sh
#!/bin/bash
docker-compose -f docker-compose.otel.yml down
```

---

### Phase 3: Integration Tests (Days 6-7)

**Task 3.1: Create `internal/telemetry/otel_integration_test.go`**
```go
package telemetry

import (
    "context"
    "encoding/json"
    "net/http"
    "testing"
    "time"
)

// TestOTELExportToJaeger verifies spans are exported to Jaeger
func TestOTELExportToJaeger(t *testing.T) {
    // Skip if Jaeger not available
    if !isJaegerAvailable() {
        t.Skip("Jaeger not available, run 'make otel-stack-up' first")
    }

    // Setup: Start agent execution with OTEL enabled
    ctx := context.Background()

    // Execute test agent
    // This should generate spans

    // Wait for spans to be exported (async)
    time.Sleep(2 * time.Second)

    // Query Jaeger API for traces
    traces := queryJaegerTraces("station")

    // Verify: Spans exist in Jaeger
    if len(traces) == 0 {
        t.Fatal("No traces found in Jaeger")
    }

    // Verify: Trace structure is correct
    assertTraceHasSpan(t, traces[0], "agent.execute")
    assertTraceHasSpan(t, traces[0], "mcp.tool.call")
}

func isJaegerAvailable() bool {
    resp, err := http.Get("http://localhost:16686")
    if err != nil {
        return false
    }
    defer resp.Body.Close()
    return resp.StatusCode == 200
}

func queryJaegerTraces(serviceName string) []JaegerTrace {
    url := fmt.Sprintf("http://localhost:16686/api/traces?service=%s&limit=10", serviceName)
    resp, err := http.Get(url)
    if err != nil {
        return nil
    }
    defer resp.Body.Close()

    var result JaegerQueryResult
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Data
}
```

**Task 3.2: Add Test Make Target**
```makefile
# Test OTEL integration
test-otel:
	@echo "🧪 Testing OTEL integration..."
	@if ! curl -s http://localhost:16686 > /dev/null; then \
		echo "❌ Jaeger not running. Start with 'make otel-stack-up'"; \
		exit 1; \
	fi
	go test -v ./internal/telemetry -run TestOTEL
	@echo "✅ OTEL tests passed"
```

---

### Phase 4: Documentation (Days 8-9)

**Task 4.1: Create `docs/OTEL_INTEGRATION.md`**
```markdown
# OpenTelemetry Integration Guide

## Quick Start

### Local Testing with Jaeger
```bash
# Start Jaeger
make otel-stack-up

# Run Station with OTEL export
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
stn serve

# Execute an agent
stn agent run test-agent "analyze this project"

# View traces in Jaeger UI
open http://localhost:16686
```

## Team Integration Patterns

### Jaeger (Self-Hosted)
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger.company.com:4318
export OTEL_SERVICE_NAME=station-prod
stn serve
```

### Grafana Tempo
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo.company.com:4318
export OTEL_SERVICE_NAME=station-prod
stn serve
```

### Datadog
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://datadog-agent:4318
export OTEL_SERVICE_NAME=station-prod
stn serve
```

### Honeycomb
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io:443
export OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=YOUR_API_KEY"
export OTEL_SERVICE_NAME=station-prod
stn serve
```

### AWS X-Ray (via ADOT Collector)
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://adot-collector:4318
export OTEL_SERVICE_NAME=station-prod
stn serve
```

## Configuration Options

### Environment Variables
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP endpoint (required)
- `OTEL_SERVICE_NAME` - Service name in traces (default: "station")
- `OTEL_SERVICE_VERSION` - Service version (default: "dev")
- `GENKIT_ENV` - Must be "production" to enable export

### Config File (`~/.config/station/config.yml`)
```yaml
telemetry_enabled: true
otel_endpoint: http://localhost:4318
```

## Trace Structure

Station generates these span types:

### Agent Execution Spans
- `agent.execute` - Top-level agent execution
- `agent.step` - Individual agent reasoning step
- `agent.tool_selection` - Tool selection decision

### MCP Tool Spans
- `mcp.tool.call` - MCP tool invocation
- `mcp.tool.response` - MCP tool response processing

### Model Inference Spans
- `model.generate` - LLM generation request
- `model.response` - LLM response processing

## Troubleshooting

### No spans appearing in collector
1. Check GENKIT_ENV is not "dev": `echo $GENKIT_ENV`
2. Verify endpoint is reachable: `curl http://localhost:4318`
3. Check Station logs for OTEL errors
4. Enable debug: `export STN_DEBUG=true`

### Spans missing attributes
- Ensure GenKit v1.0.1 or later
- Check agent execution completes successfully
- Verify telemetry client is initialized

### Performance impact
- OTEL export is asynchronous (minimal impact)
- Batch size: 100 spans
- Batch timeout: 5 seconds
- Consider sampling for high-volume deployments
```

**Task 4.2: Add to Main README**
Add OTEL section to main README.md with link to integration guide.

---

### Phase 5: Makefile Targets (Day 10)

**Task 5.1: Add to `Makefile`**
```makefile
# OpenTelemetry & Observability targets

# Start OTEL stack (Jaeger + Station)
otel-stack-up:
	@echo "🚀 Starting OTEL stack with Jaeger..."
	@docker-compose -f docker-compose.otel.yml up -d
	@echo "✅ OTEL stack started"
	@echo "📊 Jaeger UI: http://localhost:16686"
	@echo "🔌 OTLP HTTP endpoint: http://localhost:4318"
	@echo ""
	@echo "💡 To test OTEL export:"
	@echo "   export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318"
	@echo "   stn serve"

# Stop OTEL stack
otel-stack-down:
	@echo "🛑 Stopping OTEL stack..."
	@docker-compose -f docker-compose.otel.yml down
	@echo "✅ OTEL stack stopped"

# Test OTEL integration
test-otel:
	@echo "🧪 Testing OTEL integration..."
	@if ! curl -s http://localhost:16686 > /dev/null 2>&1; then \
		echo "❌ Jaeger not running. Start with 'make otel-stack-up'"; \
		exit 1; \
	fi
	@go test -v ./internal/telemetry -run TestOTEL
	@echo "✅ OTEL integration tests passed"

# Run agent with OTEL export and verify traces
test-otel-e2e:
	@echo "🎯 Running end-to-end OTEL test..."
	@./scripts/test-otel-e2e.sh

# Verify traces in Jaeger
verify-otel-traces:
	@echo "🔍 Querying Jaeger for Station traces..."
	@curl -s "http://localhost:16686/api/traces?service=station&limit=10" | jq '.data | length' | xargs -I {} echo "Found {} traces"
	@echo "📊 View in UI: http://localhost:16686"

# Clean OTEL data
otel-clean:
	@echo "🧹 Cleaning OTEL data..."
	@docker-compose -f docker-compose.otel.yml down -v
	@echo "✅ OTEL data cleaned"
```

**Task 5.2: Create `scripts/test-otel-e2e.sh`**
```bash
#!/bin/bash
set -e

echo "🎯 End-to-end OTEL test"
echo ""

# Check Jaeger is running
if ! curl -s http://localhost:16686 > /dev/null; then
    echo "❌ Jaeger not running. Start with 'make otel-stack-up'"
    exit 1
fi

echo "✅ Jaeger is running"

# Start Station with OTEL
echo "🚀 Starting Station with OTEL export..."
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_SERVICE_NAME=station-test
export GENKIT_ENV=production

# Start in background
stn serve > /tmp/station-otel-test.log 2>&1 &
STATION_PID=$!
echo "✅ Station started (PID: $STATION_PID)"

# Wait for Station to be ready
sleep 3

# Execute test agent
echo "🤖 Executing test agent..."
stn agent run test-agent "List files in current directory" || true

# Wait for spans to be exported
echo "⏳ Waiting for spans to export..."
sleep 5

# Query Jaeger for traces
echo "🔍 Querying Jaeger for traces..."
TRACE_COUNT=$(curl -s "http://localhost:16686/api/traces?service=station-test&limit=10" | jq '.data | length')

# Cleanup
echo "🧹 Cleaning up..."
kill $STATION_PID 2>/dev/null || true

# Verify
if [ "$TRACE_COUNT" -gt 0 ]; then
    echo "✅ End-to-end OTEL test passed! Found $TRACE_COUNT traces"
    echo "📊 View in Jaeger: http://localhost:16686/search?service=station-test"
    exit 0
else
    echo "❌ No traces found in Jaeger"
    echo "📋 Check Station logs: /tmp/station-otel-test.log"
    exit 1
fi
```

---

## 🧪 Testing Strategy

### Unit Tests
```bash
go test ./internal/telemetry -v
```

### Integration Tests (Requires Jaeger)
```bash
make otel-stack-up
make test-otel
```

### End-to-End Tests
```bash
make otel-stack-up
make test-otel-e2e
```

### Manual Verification
```bash
# 1. Start OTEL stack
make otel-stack-up

# 2. Start Station with OTEL
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
make local-install-ui
stn serve &

# 3. Run an agent
stn agent run test-agent "test task"

# 4. Check Jaeger UI
open http://localhost:16686
# Search for service: "station"
# Verify spans appear
```

---

## 📊 Success Criteria

- [ ] Spans export to Jaeger successfully
- [ ] Trace hierarchy shows: agent → tool calls → model inference
- [ ] Span attributes include agent name, tool names, token usage
- [ ] Integration tests pass programmatically
- [ ] End-to-end test script works
- [ ] Documentation covers 5+ team integration patterns
- [ ] Make targets work on fresh clone
- [ ] Zero breaking changes to existing Station functionality

---

## 🚨 Risks & Mitigations

### Risk 1: GenKit v1.0.1 API Discovery
**Mitigation**: Start with manual span creation (proven pattern), iterate to GenKit integration

### Risk 2: OTEL Export Performance Impact
**Mitigation**: Use batch processor (5s timeout, 100 spans), make it opt-in via GENKIT_ENV

### Risk 3: Integration Test Flakiness
**Mitigation**: Add retry logic, clear wait times, health checks for Jaeger

### Risk 4: Docker Compose Conflicts
**Mitigation**: Use unique container names, separate network, document port requirements

---

## 📅 Timeline

**Days 1-3**: Research GenKit v1.0.1, fix `otel_plugin.go`, add manual span creation
**Days 4-5**: Create Docker Compose stack, test locally
**Days 6-7**: Write integration tests, add test automation
**Days 8-9**: Write documentation, team integration patterns
**Days 10**: Polish Make targets, run full test suite, prepare for merge

---

## 🎯 Next Steps

1. **START**: Research GenKit v1.0.1 telemetry APIs
2. Add manual span creation to `agent_execution_engine.go`
3. Test spans export to local Jaeger
4. Build out Docker Compose stack
5. Write programmatic tests
6. Document team integration

---

**Status**: Ready to begin implementation
**Blocked By**: None
**Blocking**: None (Section 4 can proceed in parallel)
