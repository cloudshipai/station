# Station Zero-to-Hero Development PRD

**Version**: 1.2
**Last Updated**: 2025-12-25
**Status**: In Progress

---

## Executive Summary

This PRD outlines a comprehensive strategy for building a developer-friendly, production-ready workflow for Station agent development from "zero to hero". The focus is on rapid iteration, robust testing, proper observability, and seamless CICD integration.

## Vision

Enable developers to go from idea to production-ready agents in minutes with:
- Interactive development with `stn develop` + Genkit Developer UI
- Comprehensive agent evaluation framework with real and mocked MCP tools
- Full OpenTelemetry tracing for debugging and performance analysis
- Seamless CICD integration with GitHub Actions
- Support for multiple AI providers including OpenAI-compatible endpoints (Llama, local models)

---

## Testing Protocol

**⚠️ CRITICAL: For all testing phases, follow this process:**

1. **Unit Testing First**: Run `go test ./...` to validate code changes
2. **Integration Testing**: For full system tests or UAT:
   - Run `make local-install-ui` to build new binary with embedded UI
   - Start server in background: `stn serve &`
   - Test the complete workflow end-to-end
3. **Manual UAT**: Test real-world scenarios with actual agents and MCP servers

---

## Section 1: GenKit Development Environment (`stn develop`)

### Current State
- ✅ `stn develop` command exists (`cmd/main/develop.go`)
- ✅ Loads agents from environment's `agents/` directory
- ✅ Connects to MCP servers and registers tools
- ✅ Initializes GenKit with OpenAI and GoogleAI plugins
- ✅ **Interactive variable prompting via Station UI during sync** (COMPLETED v0.9.2)
  - Real-time sync progress with modal UI
  - Multi-variable detection and prompting in single interaction
  - Monaco Editor integration with Tokyo Night theme
  - Automatic UI refresh after completion
  - Backend: Custom VariableResolver, enhanced DeclarativeSync service
- ⚠️ GenKit Developer UI requires manual launch with `genkit start`
- ⚠️ No built-in eval framework integration

### Completed: Interactive Sync Flow ✅

**What Was Implemented:**
The Station UI now provides a complete interactive workflow for handling missing template variables during `stn sync`:

1. **Backend Variable Resolution** (`internal/services/declarative_sync.go`):
   - Custom `VariableResolver` detects missing variables in `template.json`
   - Enhanced `DeclarativeSync` service coordinates UI-based variable prompting
   - Graceful handling of 404 errors when template files don't exist

2. **Frontend Sync Modal** (`ui/src/components/SyncModal.tsx`):
   - Real-time sync progress tracking
   - Dynamic form generation for missing variables
   - Monaco Editor integration for `variables.yml` editing
   - Tokyo Night theme for consistency
   - Uncontrolled inputs for better performance

3. **API Integration** (`internal/api/handlers/sync.go`):
   - WebSocket-based progress updates
   - Variable prompting endpoint
   - Error handling with clear user feedback

**User Experience:**
```bash
# Developer runs sync
stn sync my-environment

# If variables are missing:
# 1. Station UI modal opens automatically
# 2. Form shows all missing variables with descriptions
# 3. Developer fills in values (e.g., PROJECT_ROOT=/workspace)
# 4. Click "Save and Continue"
# 5. Sync completes automatically
# 6. UI refreshes to show synced agents
```

**Technical Achievement:**
- No more manual `variables.yml` editing required
- Handles multiple missing variables in single interaction
- Seamless integration with existing sync workflow
- Production-ready error handling and user feedback

### Goals

**G1.1: One-Command Interactive Development**
- `stn develop --env default` starts complete development environment
- **TODO**: GenKit Developer UI auto-launches at `http://localhost:4000`
- All agents and tools visible in UI for immediate testing
- **TODO**: Hot-reload support for `.prompt` file changes

**G1.2: Multi-Provider AI Support**
- ✅ OpenAI (official plugin)
- ✅ Google Gemini (official plugin)
- 🆕 OpenAI-compatible endpoints (Llama, Ollama, local models)
- 🆕 Provider switching without code changes

**G1.3: Enhanced Developer Experience**
- Real-time MCP tool registration in UI
- Agent input schema validation in UI
- Live execution logs with tool call details
- Error handling with clear debugging information

### Technical Implementation

**T1.1: OpenAI-Compatible Provider Support**
```go
// internal/services/genkit_provider.go
type OpenAICompatibleProvider struct {
    BaseURL string // e.g., http://localhost:11434/v1 for Ollama
    APIKey  string
    Model   string
}

// Support in config
AI_PROVIDER=openai-compatible
AI_BASE_URL=http://localhost:11434/v1
AI_MODEL=llama3.2
```

**T1.2: Auto-Launch GenKit UI**
```go
// cmd/main/develop.go
func launchGenkitUI(port int) error {
    cmd := exec.Command("genkit", "start", "-o", "--port", strconv.Itoa(port))
    return cmd.Start()
}
```

**T1.3: Hot-Reload for Prompt Changes**
```go
// Watch agents/ directory for .prompt file changes
// Reload affected agents without restarting MCP connections
```

### Acceptance Criteria

**Development Environment:**
- [ ] `stn develop` auto-launches GenKit UI at configurable port
- [x] OpenAI-compatible endpoints work with Llama/Ollama (✅ Tested with Llama-4-Maverick)
- [x] Multi-model support documented (✅ docs/MULTI_MODEL_SUPPORT.md)
- [ ] All MCP tools appear in GenKit Developer UI
- [ ] Agent schemas render correctly in UI
- [ ] Hot-reload works for prompt file modifications
- [ ] Clear error messages for misconfigured providers

**Interactive Sync (COMPLETED v0.9.2):**
- [x] ✅ UI-based variable prompting during `stn sync`
- [x] ✅ Real-time sync progress with WebSocket updates
- [x] ✅ Multi-variable detection and form generation
- [x] ✅ Monaco Editor integration for `variables.yml`
- [x] ✅ Automatic UI refresh after sync completion
- [x] ✅ Graceful error handling with user feedback

### Testability

**Unit Tests**:
- Test OpenAI-compatible provider initialization
- Test agent prompt loading from directory
- Test MCP tool registration

**Integration Tests**:
- Run `make local-install-ui && stn serve &`
- Execute `stn develop --env test-env`
- Verify GenKit UI launches and shows agents
- Test agent execution with mock MCP server

**Manual UAT**:
- Test with Ollama running Llama3.2
- Test with OpenAI GPT-4o-mini
- Test with Google Gemini
- Verify MCP tools work in all configurations

---

## Section 2: Agent Evaluation Framework

### Current State
- ✅ Mock MCP servers exist (`pkg/mocks/`)
- ✅ EntropyHelper for variable mock data (`pkg/mocks/entropy.go`)
- ✅ Multiple mock implementations (AWS, Datadog, GitHub, Security)
- ❌ No structured eval framework
- ❌ No eval metrics tracking
- ❌ No eval reporting

### Goals

**G2.1: Comprehensive Eval Framework**
- Run evals locally with `stn eval run <agent-name>`
- Run evals in CICD automatically
- Support for both real and mocked MCP tools
- Eval results stored in database with metrics

**G2.2: Eval Metrics Tracking**
- Success rate per eval scenario
- Token usage and cost estimation
- Tool call patterns and frequency
- Response quality scoring
- Performance benchmarks (latency, throughput)

**G2.3: Faker Proxy Integration for Evals**
- Use faker proxy to enrich real MCP responses
- Learn schemas from production tools
- Generate realistic test data for evals
- Offline eval capability with cached schemas

### Technical Implementation

**T2.1: Eval Runner Service**
```go
// internal/services/eval_runner.go
type EvalRunner struct {
    agent       Agent
    scenarios   []EvalScenario
    mcpMode     string // "real", "mock", "faker"
}

type EvalScenario struct {
    Name        string
    Description string
    Input       string
    Expected    EvalExpectation
    MCPTools    []string // Required tools
}

type EvalExpectation struct {
    MinToolCalls    int
    MaxToolCalls    int
    RequiredTools   []string
    OutputValidator func(string) (bool, error)
}
```

**T2.2: Eval Database Schema**
```sql
CREATE TABLE eval_runs (
    id INTEGER PRIMARY KEY,
    agent_id INTEGER,
    scenario_name TEXT,
    mcp_mode TEXT, -- real, mock, faker
    success BOOLEAN,
    token_usage JSONB,
    tool_calls JSONB,
    duration_ms INTEGER,
    error TEXT,
    created_at TIMESTAMP
);
```

**T2.3: Eval CLI Commands**
```bash
# Run all evals for agent
stn eval run "Security Scanner" --mode mock

# Run specific eval scenario
stn eval run "Security Scanner" --scenario cve-detection --mode faker

# View eval history
stn eval list --agent "Security Scanner"

# Compare eval runs
stn eval compare run-123 run-456
```

**T2.4: Faker Proxy for Evals**
```bash
# Use faker proxy in eval template.json
{
  "mcpServers": {
    "aws-faker": {
      "command": "stn",
      "args": ["faker", "--command", "stn", "--args", "mock,aws-guardduty"]
    }
  }
}
```

### Acceptance Criteria

- [ ] `stn eval run` executes agent with eval scenarios
- [ ] Eval results stored with full metrics
- [ ] Faker proxy works seamlessly in eval mode
- [ ] Eval reports show success/failure per scenario
- [ ] Token usage tracked and reported
- [ ] CICD integration runs evals automatically
- [ ] Eval comparison shows metric deltas

### Testability

**Unit Tests**:
- Test EvalRunner with mock scenarios
- Test eval metric calculation
- Test output validators
- Test faker proxy integration

**Integration Tests**:
- Run `make local-install-ui && stn serve &`
- Create test agent with eval scenarios
- Run `stn eval run test-agent --mode mock`
- Verify results stored in database
- Verify faker proxy enriches responses

**Manual UAT**:
- Run evals with real AWS tools
- Run evals with mocked AWS tools
- Run evals with faker proxy
- Compare results across modes
- Verify metrics accuracy

---

## Section 3: OpenTelemetry Tracing Integration

### Current State ✅ COMPLETED
- ✅ OTEL telemetry service implemented (`internal/services/telemetry.go`)
- ✅ Integration wired into main.go with global lifecycle management
- ✅ Agent execution engine OTEL support with SetTelemetryService()
- ✅ Jaeger integration working (localhost:16686)
- ✅ Tests passing (6 OTEL tests including nil safety)
- ✅ Documentation complete (`docs/OTEL_SETUP.md` - 640+ lines)
- ✅ GenKit native spans + Station custom spans captured
- ✅ Trace correlation across agent runs and MCP tool calls

### Goals

**G3.1: Full Distributed Tracing**
- Export all agent executions to OTEL collector
- Trace correlation across MCP tool calls
- Performance profiling for agent execution
- Tool call timing and latency analysis

**G3.2: Jaeger Integration**
- Easy local setup with `make jaeger`
- Agent traces viewable in Jaeger UI
- Trace search by agent name, run ID, tool used
- Span attributes for debugging

**G3.3: GenKit v1.0.1 OTEL Compatibility**
- Fix broken `RegisterSpanProcessor` calls
- Use GenKit v1.0.1 telemetry APIs correctly
- Export trace data to OTLP endpoint

### Technical Implementation

**T3.1: Fix GenKit v1.0.1 OTEL Integration**
```go
// internal/telemetry/otel_plugin.go (FIXED)
// Remove broken RegisterSpanProcessor call
// Use GenKit's native telemetry export

func SetupOpenTelemetryWithGenkit(ctx context.Context, g *genkit.Genkit, cfg OTelConfig) error {
    // Create OTLP exporter
    exporter, err := otlptracehttp.New(ctx,
        otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
        otlptracehttp.WithInsecure(),
    )

    // Create trace provider
    tracerProvider := trace.NewTracerProvider(
        trace.WithSpanProcessor(trace.NewBatchSpanProcessor(exporter)),
        trace.WithResource(resource.NewWithAttributes(...)),
    )

    otel.SetTracerProvider(tracerProvider)
    return nil
}
```

**T3.2: Agent Execution Tracing**
```go
// Wrap agent execution with OTEL spans
func (e *AgentExecutionEngine) ExecuteWithTracing(ctx context.Context, agent Agent, task string) {
    tracer := otel.Tracer("station.agent")
    ctx, span := tracer.Start(ctx, "agent.execute")
    defer span.End()

    span.SetAttributes(
        attribute.String("agent.name", agent.Name),
        attribute.Int64("agent.id", agent.ID),
        attribute.String("task", task),
    )

    // Execute agent...
}
```

**T3.3: MCP Tool Call Tracing**
```go
// Trace each MCP tool call
func (m *MCPConnectionManager) CallToolWithTracing(ctx context.Context, tool string, params map[string]interface{}) {
    tracer := otel.Tracer("station.mcp")
    ctx, span := tracer.Start(ctx, "mcp.tool.call")
    defer span.End()

    span.SetAttributes(
        attribute.String("tool.name", tool),
        attribute.String("tool.params", fmt.Sprintf("%v", params)),
    )

    // Call MCP tool...
}
```

### Acceptance Criteria

- [x] `make jaeger` starts Jaeger with OTLP support
- [x] Agent executions appear in Jaeger UI
- [x] MCP tool calls shown as child spans
- [x] Trace search works by agent name
- [x] Span attributes include useful debugging info
- [x] No GenKit v1.0.3 compatibility errors
- [x] OTEL export works with `stn serve`
- [x] Documentation covers Jaeger, Tempo, Datadog, Honeycomb, AWS X-Ray, New Relic, Azure Monitor

### Testability

**Unit Tests**:
- Test OTEL span creation
- Test span attribute setting
- Test trace context propagation

**Integration Tests**:
- Start Jaeger: `make jaeger`
- Run `make local-install-ui && stn serve &`
- Enable OTEL: `export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`
- Execute agent: `stn agent run test-agent "test task"`
- Verify trace in Jaeger UI at http://localhost:16686

**Manual UAT**:
- Run complex agent with multiple tool calls
- View trace in Jaeger
- Verify span hierarchy is correct
- Check span attributes for debugging info
- Measure agent execution bottlenecks

---

## Section 4: Faker Proxy Production Readiness

### Current State
- ✅ Faker proxy implemented (`pkg/faker/`)
- ✅ Schema learning and caching
- ✅ Enricher with pattern detection
- ✅ CLI command `stn faker`
- ✅ 50+ instruction templates for domain-specific scenarios
- ⚠️ Limited to stdio mode
- ⚠️ Hardcoded to Google GenAI (should use Station's multi-model system)
- ❌ No testing with real MCP servers requiring credentials
- ❌ No end-to-end validation with real agent execution
- ❌ No per-tool schema tracking
- ❌ No schema seeding capability

### Goals

**G4.1: Multi-Model AI Provider Integration** 🔴 CRITICAL
- Replace hardcoded Google GenAI with Station's GenKitProvider system
- Support same AI providers as agent execution (OpenAI, Llama, Gemini, Ollama)
- Use same config mechanism: AIProvider, AIBaseURL, AIAPIKey, AIModel
- No separate AI configuration for faker

**G4.2: Real MCP Server Testing** 🔴 CRITICAL
- Test faker proxy with real MCP servers requiring credentials (AWS, Datadog, Stripe)
- Verify schema learning from actual API responses
- Validate AI enrichment produces structurally correct responses
- Prove agent receives and processes enriched data correctly

**G4.3: End-to-End Faker Workflow Validation** 🔴 CRITICAL
- Full flow: agent calls tool → faker proxies to real MCP → captures schema → AI enriches → agent receives response
- Verify enriched responses match real tool response structures
- Enable realistic scenario testing: high traffic, alerts, security vulnerabilities, log analysis
- Test agent behavior and tool selection with enriched data

**G4.4: Per-Tool Schema Tracking**
- Learn schemas per MCP tool (not generic)
- Store tool-specific response patterns
- Better mock data accuracy per tool

**G4.5: Schema Seeding from JSON Schema**
- Import JSON Schema definitions
- Seed faker cache without real API calls
- Enable offline-first development

**G4.6: Production-Grade Proxy Features**
- Request/response logging with OTEL tracing
- Error handling and retry logic
- Schema versioning
- Manual schema editing UI

### Technical Implementation

**T4.1: Multi-Model AI Provider Integration** 🔴 CRITICAL
```go
// pkg/faker/enricher.go - BEFORE (Hardcoded Google GenAI)
client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: config.APIKey})

// pkg/faker/enricher.go - AFTER (Use Station's GenKitProvider)
import "github.com/cloudship-ai/station/internal/services"

type Enricher struct {
    config       *Config
    genkitEngine *services.GenKitProvider  // Use Station's AI provider
    schemaCache  *SchemaCache
}

func NewEnricher(cfg *Config, stationConfig *config.Config) (*Enricher, error) {
    // Use Station's existing AI provider system
    genkitProvider, err := services.NewGenKitProvider(stationConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create GenKit provider: %w", err)
    }

    return &Enricher{
        config:       cfg,
        genkitEngine: genkitProvider,
        schemaCache:  NewSchemaCache(cfg.CacheDir),
    }, nil
}

// Enrichment now uses Station's multi-model system
func (e *Enricher) EnrichWithAI(ctx context.Context, schema Schema, instruction string) (interface{}, error) {
    prompt := e.buildPrompt(schema, instruction)
    response, err := e.genkitEngine.Generate(ctx, prompt)
    if err != nil {
        return nil, err
    }
    return e.parseResponse(response)
}
```

**T4.2: Real MCP Server Testing** 🔴 CRITICAL
```bash
# Test with AWS Cost Explorer MCP server
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."

# Start faker proxy with real AWS MCP server
stn faker \
  --command "npx" \
  --args "@aws-sdk/mcp-server-cost-explorer" \
  --ai-enabled \
  --ai-template "financial" \
  --debug

# Verify schema learning from real AWS responses
cat ~/.cache/station/faker/aws_get_cost_and_usage.json

# Test with Datadog MCP server
export DD_API_KEY="..."
export DD_APP_KEY="..."

stn faker \
  --command "datadog-mcp-server" \
  --ai-enabled \
  --ai-template "monitoring" \
  --debug
```

**T4.3: End-to-End Faker Validation** 🔴 CRITICAL
```go
// Create test environment with faker proxy
// ~/.config/station/environments/faker-test/template.json
{
  "mcpServers": {
    "aws-faker": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "npx",
        "--args", "@aws-sdk/mcp-server-cost-explorer",
        "--ai-enabled",
        "--ai-template", "financial"
      ],
      "env": {
        "AWS_ACCESS_KEY_ID": "{{ .AWS_ACCESS_KEY_ID }}",
        "AWS_SECRET_ACCESS_KEY": "{{ .AWS_SECRET_ACCESS_KEY }}"
      }
    }
  }
}

// Test agent execution with faker proxy
stn agent call "AWS Cost Analyzer" \
  "Analyze cost spike in production environment" \
  --env faker-test \
  --tail

// Verification points:
// 1. Agent calls aws_get_cost_and_usage tool
// 2. Faker proxy forwards to real AWS API
// 3. Faker captures real response schema
// 4. AI enriches response with realistic data
// 5. Agent receives structurally correct response
// 6. Agent makes informed tool decisions based on enriched data
```

**T4.4: Per-Tool Schema Storage**
```go
// pkg/faker/schema.go
type ToolSchema struct {
    ToolName     string
    SampleCount  int
    LastUpdated  time.Time
    RequestSchema  map[string]interface{}
    ResponseSchema map[string]interface{}
}

// Cache: ~/.cache/station/faker/<tool-name>.json
```

**T4.2: Schema Seeding**
```bash
# Seed schema from JSON Schema definition
stn faker seed --tool aws_list_instances --schema ./schemas/aws_list_instances.json

# Export learned schemas
stn faker export --output ./learned-schemas/
```

**T4.3: Enhanced Enricher**
```go
// Use tool-specific schema for enrichment
func (e *Enricher) EnrichResponse(toolName string, response interface{}) interface{} {
    schema := e.schemaCache.GetToolSchema(toolName)
    if schema == nil {
        return e.EnrichGeneric(response)
    }
    return e.EnrichWithSchema(response, schema)
}
```

### Acceptance Criteria

**Critical (Must Have)**:
- [ ] 🔴 Faker uses Station's GenKitProvider (supports OpenAI, Llama, Gemini, Ollama)
- [ ] 🔴 No hardcoded AI provider dependencies
- [ ] 🔴 Tested with real AWS Cost Explorer MCP server
- [ ] 🔴 Tested with real Datadog MCP server
- [ ] 🔴 Schema learning works from real API responses
- [ ] 🔴 AI enrichment produces structurally correct responses
- [ ] 🔴 End-to-end flow validated: agent → faker → real MCP → enriched response → agent
- [ ] 🔴 Agent makes correct tool decisions with enriched data
- [ ] 🔴 OTEL tracing captures faker proxy spans

**Important (Should Have)**:
- [ ] Faker proxy learns per-tool schemas
- [ ] Schema cache organized by tool name
- [ ] Schema seeding from JSON Schema works
- [ ] Enriched responses match tool patterns
- [ ] Schema export for sharing/versioning
- [ ] Debug mode shows schema learning in real-time

**Nice to Have**:
- [ ] Realistic scenario testing: high traffic, alerts, vulnerabilities
- [ ] Schema versioning support
- [ ] Manual schema editing UI

### Testability

**Unit Tests**:
- Test EvalRunner with mock scenarios
- Test eval metric calculation
- Test output validators
- Test faker proxy integration

**Integration Tests**:
- Run `make local-install-ui && stn serve &`
- Create test agent with eval scenarios
- Run `stn eval run test-agent --mode mock`
- Verify results stored in database
- Verify faker proxy enriches responses

**Manual UAT**:
- Run evals with real AWS tools
- Run evals with mocked AWS tools
- Run evals with faker proxy
- Compare results across modes
- Verify metrics accuracy

---

## Section 5: CICD Pipeline Strategy

### Current State
- ✅ GitHub Actions CI exists (`.github/workflows/ci.yml`)
- ✅ Basic test and build workflow
- ❌ No agent eval integration
- ❌ No bundle testing
- ❌ No agent team validation

### Goals

**G5.1: Agent Evaluation in CICD**
- Run agent evals on every PR
- Block merge on eval failures
- Report eval metrics in PR comments
- Track eval performance over time

**G5.2: Bundle Lifecycle Testing**
- Validate bundle structure
- Test agent installation from bundle
- Verify MCP server connections
- End-to-end bundle testing

**G5.3: Developer-Friendly Workflow**
- Fast feedback loop (<5 min)
- Clear error messages in PR comments
- One-click retry for flaky evals
- Parallel eval execution

### Technical Implementation

**T5.1: GitHub Actions Eval Workflow**
```yaml
# .github/workflows/agent-evals.yml
name: Agent Evaluations

on:
  pull_request:
    paths:
      - 'environments/*/agents/**'
      - 'internal/services/**'

jobs:
  eval-agents:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Build Station
        run: make build

      - name: Run Agent Evals
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: |
          stn eval run --all --mode mock --output eval-results.json

      - name: Comment PR with Results
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const results = JSON.parse(fs.readFileSync('eval-results.json'));
            const comment = formatEvalResults(results);
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: comment
            });
```

**T5.2: Bundle Testing Workflow**
```yaml
# .github/workflows/bundle-test.yml
name: Bundle Testing

on:
  pull_request:
    paths:
      - 'bundles/**'

jobs:
  test-bundle:
    runs-on: ubuntu-latest
    steps:
      - name: Validate Bundle Structure
        run: stn bundle validate ${{ matrix.bundle }}

      - name: Install Bundle
        run: stn template install ./bundles/${{ matrix.bundle }}.tar.gz

      - name: Run Bundle Tests
        run: stn eval run --bundle ${{ matrix.bundle }} --mode mock
```

**T5.3: Agent Team Testing**
```bash
# Test multi-agent workflows
stn team test security-pipeline --scenario full-scan
```

### Acceptance Criteria

- [ ] Agent evals run automatically on PR
- [ ] Eval results posted as PR comment
- [ ] Bundle validation prevents broken bundles
- [ ] CICD completes in <5 minutes
- [ ] Flaky test detection and retry
- [ ] Eval metrics tracked over time

### Testability

**Unit Tests**:
- Test eval result formatting
- Test PR comment generation
- Test bundle validation logic

**Integration Tests**:
- Create test PR with agent changes
- Verify eval workflow triggers
- Check PR comment format
- Verify merge blocking on failures

**Manual UAT**:
- Create real PR with agent modification
- Watch CICD run and complete
- Verify eval results in PR comment
- Test merge blocking on failure

---

## Section 6: Zero-to-Hero Developer Journey

### User Story: New Developer Onboarding

**As a new developer**, I want to build and deploy my first AI agent in 30 minutes with zero prior knowledge.

### Journey Steps

**Step 1: Installation (2 minutes)**
```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
stn --version
```

**Step 2: Initialize Environment (3 minutes)**
```bash
stn init
stn env create my-first-agent
cd ~/.config/station/environments/my-first-agent
```

**Step 3: Add MCP Tools (5 minutes)**
```bash
# Add filesystem tools
stn mcp add filesystem

# View available tools
stn tools list
```

**Step 4: Create Agent (10 minutes)**
```bash
# Create agent prompt file
cat > agents/file-analyzer.prompt <<EOF
---
metadata:
  name: "File Analyzer"
  description: "Analyzes project files and provides insights"
  tags: ["filesystem", "analysis"]
model: gpt-4o-mini
max_steps: 5
tools:
  - "__read_text_file"
  - "__list_directory"
  - "__search_files"
---

{{role "system"}}
You are a helpful file analyzer that examines project structures and provides insights.

{{role "user"}}
{{userInput}}
EOF

# Sync agent to database
stn sync my-first-agent
```

**Step 5: Test Interactively (5 minutes)**
```bash
# Launch GenKit Developer UI
stn develop --env my-first-agent

# Open http://localhost:4000
# Test agent in UI
```

**Step 6: Run Evals (3 minutes)**
```bash
# Create eval scenario
stn eval create file-analyzer --scenario analyze-python-project

# Run eval
stn eval run file-analyzer --mode mock
```

**Step 7: Deploy to CICD (2 minutes)**
```bash
# Export as bundle
stn bundle create my-first-agent

# Push to GitHub
git add .
git commit -m "feat: Add file analyzer agent"
git push origin main
```

### Success Metrics

- [ ] Installation completes in <2 minutes
- [ ] First agent running in <30 minutes
- [ ] Developer understands eval framework
- [ ] CICD integration setup in <5 minutes
- [ ] Clear error messages guide debugging

---

## Implementation Roadmap

### Phase 1: Faker Proxy Production Readiness 🔴 CRITICAL PRIORITY
- [ ] 🔴 Replace Google GenAI with Station's GenKitProvider
- [ ] 🔴 Test with real AWS Cost Explorer MCP server
- [ ] 🔴 Test with real Datadog MCP server
- [ ] 🔴 End-to-end validation: agent → faker → real MCP → enriched response
- [ ] 🔴 OTEL tracing for faker proxy spans
- [ ] Per-tool schema tracking
- [ ] Schema seeding from JSON
- [ ] Schema export/import

### Phase 2: Development Experience (Week 2-3)
- [ ] Implement `stn develop` auto-launch
- [ ] Add hot-reload for prompt files
- [ ] GenKit Developer UI integration
- [ ] Enhanced debugging experience

### Phase 3: Eval Framework (Week 4-5)
- [ ] Build eval runner service
- [ ] Implement eval database schema
- [ ] Create eval CLI commands
- [ ] Integrate faker proxy with evals
- [ ] LLM-as-judge for response quality

### Phase 4: CICD Integration (Week 6)
- [ ] GitHub Actions eval workflow
- [ ] Bundle testing workflow
- [ ] PR comment integration
- [ ] Performance tracking

### Phase 5: Benchmarking & Advanced Evals (Week 7-8)
- [ ] Agent team benchmarking framework
- [ ] Realistic scenario generators (high-traffic, security-vuln, alert-storm)
- [ ] Comparative performance analysis
- [ ] Multi-agent coordination testing

### Phase 6: Documentation & Onboarding (Week 9)
- [ ] Zero-to-hero tutorial
- [ ] Video walkthroughs
- [ ] Example agent bundles
- [ ] Troubleshooting guides

---

## Completed Phases

### ✅ Core Observability (COMPLETED)
- [x] Fix GenKit v1.0.3 OTEL integration
- [x] Add Jaeger integration
- [x] Implement agent execution tracing
- [x] Add MCP tool call tracing
- [x] Documentation for Jaeger, Tempo, Datadog, Honeycomb, AWS X-Ray

### ✅ Multi-Model Support (COMPLETED)
- [x] Add OpenAI-compatible provider support (Llama, Ollama)
- [x] Multi-model AI support documentation (340 lines)
- [x] Tested with Llama-4-Maverick-17B-128E-Instruct-FP8

### ✅ Workflow Engine V1 (COMPLETED - 2025-12-25)

**Bug Fixes Committed:**
- [x] Made `RecordStepStart()` idempotent to prevent duplicate step errors (commit `27c2e018`)
- [x] Added graceful handling for stale NATS messages and missing runs
- [x] Fixed `isUniqueConstraintError()` helper for SQLite constraint detection
- [x] Fixed NATS JetStream consumer using ephemeral pull-based consumer (commit `d2194fea`)
- [x] Fixed cron-triggered workflows skipping trigger step to start from executable step (2025-12-25)

**Workflow Engine Testing Results:**

| Step Type | Status | Notes |
|-----------|--------|-------|
| `inject` | ✅ Working | Data injection into context works correctly |
| `parallel` | ✅ Working | 4 parallel branches executed successfully |
| `operation` | ✅ Working | Agent execution via `agent.run` task type |
| `switch` | ✅ Working | Conditional routing based on context data |
| `try_catch` | ✅ Defined | Error handling structure validated |
| `timer` | ✅ Defined | Delay/wait functionality available |
| `foreach` | ✅ Defined | Iteration over collections supported |
| `cron` | ✅ Working | Scheduled workflow execution (fixed: skip trigger step) |

**DevOps Workflow Examples Created:**

1. **`production-incident-response`** (11 states)
   - Pattern: inject → parallel(4 agents) → operation → switch → try_catch → operation
   - Tests: Parallel diagnostics, severity-based routing, human approval, error handling

2. **`daily-health-check`** (9 states)
   - Pattern: cron → foreach → parallel → switch → operation
   - Tests: Scheduled execution, service iteration, conditional alerting

3. **`deployment-validation`** (9 states)
   - Pattern: inject → parallel → operation(approval) → try_catch(foreach) → parallel → operation
   - Tests: Pre/post deployment checks, approval gates, rollback handling

**API Validation:**
```bash
# Workflow creation
POST /api/v1/workflows ✅ (with correct payload format)

# Workflow execution
POST /api/v1/workflows/:id/runs ✅

# Run monitoring
GET /api/v1/workflow-runs/:runId ✅
GET /api/v1/workflow-runs/:runId/steps ✅
```

**Documentation Created:**
- `docs/TESTING_PLAYBOOK.md` - Comprehensive testing guide
- `docs/workflows/devops-examples/` - 3 production-ready workflow templates

---

## Risk Mitigation

**R1: GenKit v1.0.1 API Changes**
- Risk: Breaking changes require significant refactoring
- Mitigation: Test with GenKit v1.0.1 early, maintain compatibility layer

**R2: OTEL Export Performance**
- Risk: Tracing overhead slows agent execution
- Mitigation: Batch span export, sampling in production

**R3: Faker Proxy Schema Accuracy**
- Risk: Generated mock data doesn't match real patterns
- Mitigation: Schema seeding, per-tool schemas, manual validation

**R4: CICD Eval Flakiness**
- Risk: Non-deterministic agent behavior causes flaky tests
- Mitigation: Retry logic, deterministic mock data, eval timeouts

---

## Success Criteria

**Overall Project Success**:
- [ ] `stn develop` provides world-class DX
- [ ] Agent evals run reliably in CICD
- [ ] Full OTEL tracing to Jaeger works
- [ ] Faker proxy enables offline development
- [ ] New developers productive in <30 minutes
- [ ] Production agents deployed via CICD

**Quantitative Metrics**:
- Agent development time: <30 min (from idea to eval)
- CICD feedback time: <5 min
- Eval pass rate: >95%
- Developer onboarding: <30 min
- Test coverage: >80%

---

## Appendix: Key File Locations

**Development Commands**:
- `cmd/main/develop.go` - stn develop implementation
- `cmd/main/faker.go` - stn faker implementation

**Services**:
- `internal/services/genkit_provider.go` - AI provider configuration
- `internal/services/agent_execution_engine.go` - Agent execution
- `internal/telemetry/otel_plugin.go` - OTEL integration
- `internal/telemetry/genkit_telemetry_client.go` - Telemetry capture

**Faker Proxy**:
- `pkg/faker/proxy.go` - Proxy implementation
- `pkg/faker/enricher.go` - Response enrichment
- `pkg/faker/schema.go` - Schema learning
- `pkg/mocks/entropy.go` - Variable mock data

**CICD**:
- `.github/workflows/ci.yml` - Current CI workflow
- `.github/workflows/agent-evals.yml` - (To be created)
- `.github/workflows/bundle-test.yml` - (To be created)

---

**End of PRD**
