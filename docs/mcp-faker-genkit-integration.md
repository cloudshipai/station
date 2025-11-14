# MCP Faker Proxy - GenKit Integration Plan

## Current Status

**Foundation Laid**: GenKit AI enricher structure created (`pkg/faker/ai_enricher_genkit.go`) with Station's GenKitProvider integration pattern.

**Blocked**: GenKit v1.0.3 AI API complexity requires additional research for proper integration patterns.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│ Station Agent Execution                                      │
│  ├─ Uses Station's GenKitProvider                           │
│  ├─ Supports: OpenAI, Gemini, Llama, Ollama                │
│  └─ Config: AIProvider, AIModel, AIBaseURL, AIAPIKey       │
└─────────────────────────────────────────────────────────────┘
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ MCP Faker Proxy (CURRENT - No AI)                           │
│  ├─ Bidirectional stdio proxy                              │
│  ├─ Schema learning from real responses                     │
│  ├─ Basic enrichment with gofakeit                         │
│  └─ 50+ instruction templates                              │
└─────────────────────────────────────────────────────────────┘
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ MCP Faker Proxy (TARGET - With Station AI)                  │
│  ├─ Use Station's GenKitProvider                           │
│  ├─ Same AI config as agent execution                       │
│  ├─ AI-enhanced enrichment for realistic data              │
│  └─ Seamless multi-model support                           │
└─────────────────────────────────────────────────────────────┘
```

## Integration Approach

### Option 1: Simple Config Sharing (RECOMMENDED FOR MVP)

**Status**: Feasible, low complexity

```go
// cmd/main/faker.go
func runFaker(cmd *cobra.Command, args []string) error {
    // Load Station config for AI provider settings
    stationConfig, err := config.Load()
    if err != nil {
        return fmt.Errorf("failed to load Station config: %w", err)
    }

    // Pass to enricher config
    enricherCfg := &faker.GenKitAIEnricherConfig{
        Enabled:     aiEnabled,
        Model:       stationConfig.AIModel,     // Use Station's configured model
        Instruction: aiInstruction,
    }

    // Create enricher with Station's AI settings
    enricher, err := faker.NewGenKitAIEnricher(schemaCache, enricherCfg)
    if err != nil {
        return fmt.Errorf("failed to create AI enricher: %w", err)
    }

    // Enricher internally uses Station's GenKitProvider
    // This ensures faker uses same AI config as agents
}
```

**Benefits**:
- Faker automatically uses Station's AI configuration
- No separate AI setup required
- Consistent behavior with agent execution
- Supports all Station AI providers: OpenAI, Gemini, Llama, Ollama

**Testing Priority**: Test with real MCP servers FIRST, then add AI enrichment

### Option 2: Dotprompt-Based Enrichment (FUTURE)

**Status**: Requires additional GenKit research

Use GenKit's dotprompt.Execute() similar to agent execution:

```go
// Create a dotprompt for enrichment
enrichPrompt := `
---
model: {{ .Model }}
---

Generate realistic mock data for {{ .ToolName }} tool response.

Schema:
{{ .Schema }}

Return ONLY valid JSON matching the schema.
`

// Execute with Station's GenKit app
resp, err := prompt.Execute(ctx,
    ai.WithInput(map[string]any{
        "Model":    enricher.config.Model,
        "ToolName": toolName,
        "Schema":   schemaJSON,
    }),
    ai.WithModelName(enricher.stationConfig.AIModel))
```

## Implementation Roadmap

### Phase 1: Real MCP Server Testing (CRITICAL - DO THIS FIRST)

**Goal**: Validate faker proxy works with real MCP servers requiring credentials

**Test Cases**:
1. AWS Cost Explorer MCP Server
   - Requires: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
   - Validate: Schema learning from real AWS API responses
   - Verify: Enrichment produces valid AWS cost data structures

2. Datadog MCP Server
   - Requires: DD_API_KEY, DD_APP_KEY
   - Validate: Schema learning from real Datadog API responses
   - Verify: Enrichment produces valid monitoring data

**Acceptance Criteria**:
- [ ] Faker proxy forwards requests to real AWS MCP server
- [ ] Schema cache captures real AWS response structures
- [ ] Basic gofakeit enrichment produces valid AWS-like data
- [ ] Agent receives faker response and continues execution
- [ ] Same validation for Datadog MCP server

### Phase 2: Station AI Integration (AFTER PHASE 1)

**Goal**: Replace hardcoded Google GenAI with Station's GenKitProvider

**Tasks**:
1. Simplify `ai_enricher_genkit.go` to use dotprompt.Execute()
2. Pass Station config to enricher
3. Test with multiple AI providers (OpenAI, Llama, Gemini)
4. Validate enriched responses match schemas

**Acceptance Criteria**:
- [ ] Faker uses Station's AIProvider config
- [ ] Works with OpenAI, Gemini, Llama-4, Ollama
- [ ] AI enrichment improves data realism over gofakeit
- [ ] No separate AI configuration required

### Phase 3: OTEL Tracing (POLISH)

**Goal**: Add observability to faker proxy operations

**Tasks**:
1. Add faker proxy spans to OTEL traces
2. Track schema learning events
3. Track AI enrichment calls
4. Measure enrichment latency

## Current Code Status

### Created Files

**`pkg/faker/ai_enricher_genkit.go` (370 lines)**
- GenKitAIEnricher struct with Station config integration
- Basic enrichment fallback (working)
- AI enrichment placeholder (needs GenKit API research)
- Schema conversion to simple JSON format (working)

**`pkg/faker/ai_enricher_genkit_test.go` (295 lines)**
- Unit tests for enricher creation
- Tests for basic fallback enrichment
- Tests for schema conversion
- Tests for JSONRPC enrichment
- Tests for field name pattern detection

**`pkg/faker/ai_enricher.go` (LEGACY - 287 lines)**
- Hardcoded Google GenAI client
- Should be deprecated once GenKit version works

### Next Immediate Steps

1. **Test with Real MCP Servers** (Priority 1)
   - Set up AWS Cost Explorer MCP server locally
   - Configure faker proxy to forward to real MCP
   - Validate schema learning from real responses
   - Test agent execution with faker-proxied tools

2. **Simplify GenKit Integration** (Priority 2)
   - Research GenKit v1.0.3 dotprompt.Execute() API
   - Implement dotprompt-based enrichment
   - Test with Station's multi-model support

3. **Document Real-World Usage** (Priority 3)
   - Create examples with real MCP servers
   - Document setup for AWS, Datadog, Stripe
   - Show agent execution with faker proxy

## Testing Real MCP Servers

### AWS Cost Explorer Example

```bash
# 1. Install AWS Cost Explorer MCP server
npm install -g @aws-sdk/mcp-server-cost-explorer

# 2. Set AWS credentials
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."

# 3. Start faker proxy with real AWS MCP
stn faker \
  --command "npx" \
  --args "@aws-sdk/mcp-server-cost-explorer" \
  --ai-enabled \
  --debug

# 4. Create test environment
mkdir -p ~/.config/station/environments/faker-aws-test
cat > ~/.config/station/environments/faker-aws-test/template.json <<EOF
{
  "mcpServers": {
    "aws-faker": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "npx",
        "--args", "@aws-sdk/mcp-server-cost-explorer"
      ],
      "env": {
        "AWS_ACCESS_KEY_ID": "{{ .AWS_ACCESS_KEY_ID }}",
        "AWS_SECRET_ACCESS_KEY": "{{ .AWS_SECRET_ACCESS_KEY }}"
      }
    }
  }
}
EOF

# 5. Sync and test
stn sync faker-aws-test
stn tools list --env faker-aws-test

# 6. Create test agent
cat > ~/.config/station/environments/faker-aws-test/agents/cost-analyzer.prompt <<EOF
---
metadata:
  name: "AWS Cost Analyzer"
model: gpt-4o-mini
max_steps: 3
tools:
  - "aws_get_cost_and_usage"
---
{{role "system"}}
Analyze AWS costs.

{{role "user"}}
{{userInput}}
EOF

# 7. Run agent with faker proxy
stn agent run "AWS Cost Analyzer" \
  "Analyze cost spike" \
  --env faker-aws-test \
  --tail

# 8. Verify schema learned
cat ~/.cache/station/faker/aws_get_cost_and_usage.json

# 9. Check OTEL traces
open http://localhost:16686
```

## Success Criteria

### Phase 1 Success (Real MCP Testing)
- [ ] Faker forwards to real AWS Cost Explorer MCP
- [ ] Schema cache stores real AWS response structure
- [ ] Enrichment produces valid AWS-like data
- [ ] Agent executes successfully with faker-proxied tools
- [ ] Same validation with Datadog MCP server

### Phase 2 Success (Station AI Integration)
- [ ] Faker uses Station's GenKitProvider
- [ ] Works with OpenAI, Gemini, Llama, Ollama
- [ ] AI enrichment improves over gofakeit baseline
- [ ] No hardcoded AI dependencies

### Phase 3 Success (OTEL Tracing)
- [ ] Faker proxy spans visible in Jaeger
- [ ] Schema learning events tracked
- [ ] AI enrichment latency measured
- [ ] End-to-end trace: agent → faker → real MCP → enriched response

## References

- **Station GenKitProvider**: `internal/services/genkit_provider.go`
- **Agent Execution with GenKit**: `pkg/dotprompt/genkit_executor.go`
- **Existing Faker (No AI)**: `pkg/faker/enricher.go`
- **Legacy AI Faker**: `pkg/faker/ai_enricher.go` (Google GenAI hardcoded)
- **GenKit v1.0.3 Docs**: https://firebase.google.com/docs/genkit
- **Multi-Model Config**: `docs/MULTI_MODEL_SUPPORT.md`

---

**Status**: Foundation complete, ready for real MCP server testing
**Next**: Test with AWS Cost Explorer and Datadog MCP servers
**Blocker**: GenKit v1.0.3 AI API requires additional research (can defer to Phase 2)
