# Station Faker E2E Testing Plan

## Status: Ready for Execution

All test environments created and configured. Ready to validate faker with OpenTelemetry observability.

## What We Built

### ✅ Complete Test Suite
Three comprehensive E2E test scenarios validating faker across increasing complexity:

1. **Test 1: Single Faker** (`faker-test-1-single`)
   - 1 faker MCP server
   - 1 agent using faker tools
   - Validates basic faker functionality

2. **Test 2: Dual Faker** (`faker-test-2-dual`)
   - 2 faker MCP servers
   - 1 agent using both fakers
   - Validates multiple faker tools in one agent

3. **Test 3: Multi-Agent Hierarchy** (`faker-test-3-hierarchy`)
   - 3 faker MCP servers
   - 4 agents (1 orchestrator + 3 leaf specialists)
   - Validates faker in hierarchical agent architectures

### ✅ OpenTelemetry Integration
- OTEL configured via `OTEL_EXPORTER_OTLP_ENDPOINT`
- Jaeger running at http://localhost:16686
- station.faker spans added to all faker tool calls
- AI enrichment spans for realistic data generation

### ✅ AWS Simulation Scenarios
Each faker simulates realistic AWS operational scenarios:
- **CloudWatch**: High CPU, connection exhaustion, slow queries
- **Billing**: Cost spikes, oversized instances, budget alerts
- **CloudTrail**: Audit events, IAM users, configuration changes

## Test Execution Steps

### 1. Prerequisites Check

```bash
# Verify Jaeger is running
docker ps | grep jaeger
# If not running:
docker compose -f docker-compose.otel.yml up -d

# Verify stn is in PATH
which stn
# Should return: /home/epuerta/.local/bin/stn

# Verify OpenAI API key (or configure alternative AI provider)
echo $OPENAI_API_KEY
```

### 2. Run Automated Test Suite

```bash
cd /home/epuerta/projects/hack/station/test-environments

# Enable OTEL and run all tests
OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318" \
GENKIT_ENV="prod" \
./run-faker-e2e-tests.sh
```

The script will:
1. Sync each test environment
2. Run agents with faker tools
3. Verify traces in Jaeger
4. Display run IDs and results

### 3. Manual Verification

For each test, verify:

**Agent Completion**:
```bash
stn runs inspect <run-id>
# Should show: status=completed, not status=running
```

**Faker Sessions**:
```bash
stn faker sessions list
# Should show new sessions with tool calls

stn faker sessions replay <session-id>
# Should export JSON with simulated data
```

**Jaeger Traces**:
1. Open http://localhost:16686
2. Search for service: `station`
3. Look for operations containing `station.faker`
4. Verify spans show:
   - `faker.tool_name`
   - `faker.session_id`
   - `faker.ai_instruction`
   - `faker.real_mcp_used`

## Expected Results

### Test 1: Single Faker

**Agent**: CloudWatch Analyzer

**Expected Behavior**:
- ✅ Lists /tmp/aws-logs/ directory (faker generates file list)
- ✅ Reads log files (faker generates AWS CloudWatch logs)
- ✅ Agent identifies high CPU issues from simulated data
- ✅ Completes in < 10 seconds

**Jaeger Traces Should Show**:
```
station: agent_execution_engine.execute
  └─ station: dotprompt.execute
      ├─ station.faker: list_directory
      │   ├─ faker.tool_name=list_directory
      │   ├─ faker.session_id=<uuid>
      │   └─ faker.real_mcp_used=true
      └─ station.faker: read_text_file
          ├─ station.faker: ai_enrichment
          │   └─ faker.ai_enrichment_enabled=true
          └─ faker.tool_name=read_text_file
```

### Test 2: Dual Faker

**Agent**: AWS Incident Investigator

**Expected Behavior**:
- ✅ Reads from TWO different faker MCP servers
- ✅ Correlates CloudWatch logs + billing data
- ✅ Identifies Lambda cost spike and throttling
- ✅ Completes in < 15 seconds

**Jaeger Traces Should Show**:
```
station: agent_execution_engine.execute
  └─ station: dotprompt.execute
      ├─ station: mcp.server.start (cloudwatch-faker)
      ├─ station.faker: list_directory (from cloudwatch-faker)
      ├─ station.faker: read_text_file (from cloudwatch-faker)
      ├─ station: mcp.server.start (billing-faker)
      ├─ station.faker: list_directory (from billing-faker)
      └─ station.faker: read_text_file (from billing-faker)
```

**Key Validation**: Two separate faker sessions created (one per MCP server)

### Test 3: Multi-Agent Hierarchy

**Agents**: Orchestrator → 3 Leaf Specialists

**Expected Behavior**:
- ✅ Orchestrator calls 3 leaf agents successfully
- ✅ Each leaf agent uses different faker MCP server
- ✅ Metrics Specialist analyzes CloudWatch
- ✅ Cost Specialist analyzes billing
- ✅ Audit Specialist analyzes CloudTrail
- ✅ Orchestrator synthesizes findings
- ✅ Completes in < 30 seconds

**Jaeger Traces Should Show**:
```
station: agent_execution_engine.execute (Orchestrator)
  └─ station: dotprompt.execute
      ├─ station: agent.call (Metrics Specialist)
      │   └─ station.faker: read_text_file (cloudwatch-faker)
      │       └─ station.faker: ai_enrichment
      ├─ station: agent.call (Cost Specialist)
      │   └─ station.faker: read_text_file (billing-faker)
      │       └─ station.faker: ai_enrichment
      └─ station: agent.call (Audit Specialist)
          └─ station.faker: read_text_file (cloudtrail-faker)
              └─ station.faker: ai_enrichment
```

**Key Validation**: Hierarchical trace showing orchestrator → leaf flow with faker spans at leaf level

## Success Criteria

### Must Have ✅
- [ ] All 3 agents complete with status=completed (no hanging)
- [ ] No agents stuck in status=running after 60 seconds
- [ ] Faker sessions created in database for each test
- [ ] `station.faker` spans visible in Jaeger
- [ ] Agent outputs contain simulated AWS data

### Should Have ✅
- [ ] Each faker span has correct attributes (tool_name, session_id)
- [ ] AI enrichment spans show faker generated realistic data
- [ ] Multi-agent hierarchy shows parent-child relationship in traces
- [ ] Run inspection shows tool calls: `stn runs inspect <id> -v`

### Nice to Have ✨
- [ ] Faker session replay exports valid JSON
- [ ] Different AI instructions produce different simulated data
- [ ] Safety mode intercepts write operations
- [ ] Trace duration < 30s for all tests

## Troubleshooting Guide

### Issue: `stn` Command Not Found

**Symptom**: MCP server fails to start with "command not found: stn"

**Solution**:
```bash
# Verify stn is in PATH
which stn

# If not, add to PATH
export PATH="$HOME/.local/bin:$PATH"

# Rebuild and install
cd /home/epuerta/projects/hack/station
make build
go install ./cmd/main
```

### Issue: No station.faker Spans in Jaeger

**Symptom**: Jaeger shows `station` traces but no `station.faker` operations

**Root Causes**:
1. OTEL not configured
2. Agent didn't use faker tools
3. Faker not initialized

**Debug**:
```bash
# Run with OTEL explicitly enabled
OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318" \
GENKIT_ENV="prod" \
stn agent run <agent-name> "task"

# Check faker initialization in logs
# Should see: "[FAKER] GenKit initialized successfully"
```

### Issue: Agent Hangs

**Symptom**: Agent stuck in "Initializing MCP server pool" for > 60s

**Root Cause**: Faker subprocess not starting properly

**Solution**:
```bash
# Kill stuck processes
pkill -9 -f "stn faker"

# Mark stuck runs as failed
sqlite3 ~/.config/station/station.db \
  "UPDATE agent_runs SET status='failed' WHERE status='running';"

# Retry with debug output
stn sync faker-test-1-single -v
```

## Next Steps After Testing

### If All Tests Pass ✅
1. **Document Best Practices**: Add faker usage guide to main docs
2. **Package as Bundles**: Convert test envs to installable bundles
3. **CI/CD Integration**: Add to GitHub Actions workflow
4. **Registry Publishing**: Publish faker bundles to Station registry

### If Tests Fail ❌
1. **Capture Logs**: Save agent run output and Jaeger traces
2. **Check Faker Sessions**: Verify sessions were created
3. **Inspect Database**: Check agent_runs table for errors
4. **Review Code**: Check mcp_faker.go for OTEL span creation

## File Locations

- **Test Environments**: `/home/epuerta/projects/hack/station/test-environments/`
- **Test Script**: `/home/epuerta/projects/hack/station/test-environments/run-faker-e2e-tests.sh`
- **Faker Code**: `/home/epuerta/projects/hack/station/pkg/faker/mcp_faker.go`
- **OTEL Plugin**: `/home/epuerta/projects/hack/station/internal/telemetry/otel_plugin.go`

## Documentation

- Faker Architecture: `/docs/features/FAKER_ARCHITECTURE.md`
- Testing Session: `/docs/features/FAKER_TESTING_SESSION_2025-11-09.md`
- E2E Test README: `/test-environments/README.md`

## Timeline

- **Setup**: Complete ✅
- **Execution**: Ready to run
- **Validation**: Pending test results
- **Documentation**: Update after successful tests

---

**Ready to Execute**: All environments configured, test script ready, Jaeger running. Execute `./run-faker-e2e-tests.sh` to begin validation.
