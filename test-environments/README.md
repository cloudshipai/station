# Station Faker E2E Test Suite

Comprehensive end-to-end testing of Station's faker system with OpenTelemetry observability.

## Overview

This test suite validates that faker works correctly across three critical scenarios:
1. **Single Agent + One Faker Tool** - Basic faker functionality
2. **Single Agent + Two Faker Tools** - Multiple faker tools in one agent
3. **Multi-Agent Hierarchy + Faker Tools** - Faker tools in leaf agents called by orchestrator

## Test Environments

### Test 1: Single Faker (`faker-test-1-single`)
**Purpose**: Validate basic faker functionality with one MCP tool

**Setup**:
- 1 Faker MCP Server: CloudWatch (simulating high CPU)
- 1 Agent: CloudWatch Analyzer

**Scenario**: AWS CloudWatch showing high CPU utilization on EC2 instances

**Expected Results**:
- ✅ Agent completes successfully
- ✅ Faker session created
- ✅ `station.faker` spans in Jaeger
- ✅ Simulated CloudWatch data with realistic CPU metrics

### Test 2: Dual Faker (`faker-test-2-dual`)
**Purpose**: Validate multiple faker tools in single agent

**Setup**:
- 2 Faker MCP Servers: CloudWatch + Billing
- 1 Agent: AWS Incident Investigator

**Scenario**: Lambda service incident with cost spike

**Expected Results**:
- ✅ Agent uses both faker tools
- ✅ Two separate faker sessions created
- ✅ Multiple `station.faker` spans in Jaeger
- ✅ Agent correlates CloudWatch + cost data

### Test 3: Multi-Agent Hierarchy (`faker-test-3-hierarchy`)
**Purpose**: Validate faker in hierarchical agent architectures

**Setup**:
- 3 Faker MCP Servers: CloudWatch + Billing + CloudTrail
- 4 Agents:
  - Metrics Specialist (leaf, uses CloudWatch faker)
  - Cost Specialist (leaf, uses Billing faker)
  - Audit Specialist (leaf, uses CloudTrail faker)
  - Orchestrator (parent, coordinates leaf agents)

**Scenario**: RDS performance issue with cost impact

**Expected Results**:
- ✅ Orchestrator calls 3 leaf agents successfully
- ✅ Each leaf agent uses faker tools correctly
- ✅ Hierarchical traces in Jaeger showing orchestrator → leaf flow
- ✅ Multiple `station.faker` spans from each leaf agent
- ✅ Orchestrator synthesizes findings from all specialists

## Running the Tests

### Prerequisites

1. **Jaeger Running**:
   ```bash
   docker compose -f docker-compose.otel.yml up -d
   ```

2. **Station Built**:
   ```bash
   make build
   ```

3. **OpenAI API Key** (or other AI provider configured):
   ```bash
   export OPENAI_API_KEY="sk-..."
   ```

### Execute Test Suite

```bash
cd /home/epuerta/projects/hack/station/test-environments
./run-faker-e2e-tests.sh
```

The script will:
1. Check Jaeger is running
2. Sync each test environment
3. Run agents with faker tools
4. Verify traces in Jaeger
5. Display summary with run IDs

### Manual Testing

**Test Individual Environments**:
```bash
# Sync environment
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export GENKIT_ENV="prod"
stn sync faker-test-1-single

# Run agent
stn agent run "CloudWatch Analyzer" "Analyze the logs"
```

## Verification Checklist

After running tests, verify:

- [ ] All 3 agents completed without errors
- [ ] No agents stuck in "running" status
- [ ] Faker sessions created (check: `stn faker sessions list`)
- [ ] Jaeger shows traces with `station.faker` spans
- [ ] Agent outputs include simulated AWS data
- [ ] Run inspection shows tool calls: `stn runs inspect <id> -v`

## Expected Jaeger Trace Structure

### Test 1 (Single Faker)
```
station: agent_execution_engine.execute
  └─ station: dotprompt.execute
      └─ station: mcp.server.start
          └─ station.faker: filesystem-faker.tool_call
              └─ station.faker: ai_enrichment
```

### Test 2 (Dual Faker)
```
station: agent_execution_engine.execute
  └─ station: dotprompt.execute
      ├─ station.faker: cloudwatch-faker.aws_cloudwatch_get_logs
      │   └─ station.faker: ai_enrichment
      └─ station.faker: billing-faker.aws_get_cost_and_usage
          └─ station.faker: ai_enrichment
```

### Test 3 (Multi-Agent Hierarchy)
```
station: agent_execution_engine.execute (Orchestrator)
  └─ station: dotprompt.execute
      ├─ station: agent_call (Metrics Specialist)
      │   └─ station.faker: cloudwatch-faker.aws_cloudwatch_get_logs
      ├─ station: agent_call (Cost Specialist)
      │   └─ station.faker: billing-faker.aws_get_cost_and_usage
      └─ station: agent_call (Audit Specialist)
          └─ station.faker: cloudtrail-faker.aws_cloudtrail_lookup_events
```

## Troubleshooting

### Faker Not Starting

**Symptom**: Agent hangs at "Initializing MCP server pool"

**Cause**: `stn` command not in PATH when MCP spawns subprocess

**Solution**: Verify `which stn` returns `/home/epuerta/.local/bin/stn`

### No station.faker Spans

**Symptom**: Jaeger shows station traces but no faker spans

**Causes**:
1. OTEL not configured (check `OTEL_EXPORTER_OTLP_ENDPOINT`)
2. Agent never called faker tools (check run with `stn runs inspect <id> -v`)
3. Faker not initialized (check logs for "GenKit initialized")

**Solution**: Run with OTEL env vars:
```bash
OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318" GENKIT_ENV="prod" stn agent run ...
```

### Agents Hanging

**Symptom**: Agent stuck in "running" status after 60+ seconds

**Causes**:
1. MCP server command path issue
2. Circular dependency (orchestrator calling itself)
3. Missing tools in agent config

**Debug**:
```bash
# Check running processes
ps aux | grep "stn faker"

# Kill stuck processes
pkill -9 -f "stn faker"

# Mark stuck runs as failed
sqlite3 ~/.config/station/station.db "UPDATE agent_runs SET status='failed' WHERE status='running';"
```

## Test Outputs

### Successful Run Example

```
═══════════════════════════════════════════════════════════
  TEST 1: Single Agent with ONE Faker MCP Tool
═══════════════════════════════════════════════════════════
🔄 Syncing environment: faker-test-1-single
✅ Synced faker-test-1-single

🤖 Running agent: CloudWatch Analyzer
📝 Task: Analyze the CloudWatch logs...

✅ Agent execution completed!
📋 Execution Results
Run ID: 295
Status: completed
Time: 3.2s

✅ Test 1 Complete - Run ID: 295
🔭 Checking Jaeger traces for: Test 1 - Single Faker
✅ Found 4 station.faker spans in Jaeger
📈 Total traces in last 5 minutes: 12
```

## AWS Simulation Scenarios

### CloudWatch Faker
- **High CPU**: 85-95% utilization on EC2
- **Lambda Throttling**: 10,000+ invocations/min with errors
- **RDS Connection Pool**: max_connections exceeded

### Billing Faker
- **Cost Spike**: 300% increase in Lambda costs
- **Oversized Instances**: db.r6g.8xlarge vs optimal db.r6g.2xlarge
- **Daily spend**: $4,500 vs $1,500 baseline

### CloudTrail Faker
- **Instance Modifications**: ModifyDBInstance events
- **IAM Users**: Realistic user names (john.doe@company.com)
- **Timestamps**: Recent events with proper format

## Next Steps

After successful testing:

1. **Package as Bundles**: Convert test environments to installable bundles
2. **CI/CD Integration**: Add to GitHub Actions workflow
3. **Documentation**: Update main docs with faker best practices
4. **Registry**: Publish faker bundles to Station registry

## References

- Faker Architecture: `/docs/features/FAKER_ARCHITECTURE.md`
- Testing Session: `/docs/features/FAKER_TESTING_SESSION_2025-11-09.md`
- OpenTelemetry Setup: `/docs/OTEL_SETUP.md`
