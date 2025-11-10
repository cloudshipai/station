# Station Faker PRD Scenarios - Multi-Agent Hierarchy Testing

## Overview

This test suite validates the Station Faker system with **multi-agent hierarchies** across three complexity levels of real-world DevOps scenarios. Each scenario uses multiple faker tools wrapping real MCP servers to simulate realistic cloud infrastructure operations.

## Test Scenarios

### 🟢 Scenario 1: EASY - Infrastructure Health Check

**Complexity**: 2 agents, 2 faker tools  
**Environment**: `scenario-easy-health-check`  
**Duration**: ~2 minutes

**Architecture**:
```
Health Check Orchestrator
├── Metrics Monitor (CloudWatch faker)
│   └── Tools: get_metric_data, get_active_alarms, analyze_metric
└── Service Monitor (ECS faker)
    └── Tools: list_directory, read_text_file
```

**Scenario**: Routine infrastructure health check for a production environment. Check CloudWatch metrics for normal CPU usage, verify no alarms are firing, and validate ECS services are healthy with all tasks running.

**Expected Behavior**:
- CloudWatch returns steady 45-55% CPU
- No active alarms (OK state)
- ECS shows 3 services running with all tasks healthy
- Orchestrator synthesizes "HEALTHY" status report

**Success Criteria**:
- ✅ Both specialist agents complete successfully
- ✅ Orchestrator correlates findings
- ✅ Final report shows overall HEALTHY status
- ✅ OTEL traces capture all agent calls

---

### 🟡 Scenario 2: MEDIUM - Cost Spike Investigation

**Complexity**: 4 agents, 3 faker tools  
**Environment**: `scenario-medium-cost-investigation`  
**Duration**: ~3 minutes

**Architecture**:
```
Cost Investigation Orchestrator
├── Billing Analyst (Billing faker)
│   └── Tools: list_directory, read_text_file
├── Resource Investigator (CloudWatch faker)
│   └── Tools: get_metric_data, analyze_metric
└── Audit Investigator (CloudTrail faker)
    └── Tools: list_directory, read_text_file
```

**Scenario**: AWS bill shows 300% cost increase. FinOps team needs to investigate what caused the spike, correlate with resource usage changes, identify who made the changes, and provide remediation steps.

**Expected Behavior**:
- Billing Analyst identifies EC2 costs jumped from $5k to $20k daily in us-west-2
- Resource Investigator finds instance count increased from 10 to 45 (4.5x)
- Resource Investigator notes CPU utilization only 20-30% (overprovisioned)
- Audit Investigator finds terraform-automation changed AutoScaling DesiredCapacity from 10 to 50
- Audit Investigator traces change to Jenkins pipeline 'prod-deploy-#1847'
- Orchestrator correlates all findings into comprehensive cost spike report

**Success Criteria**:
- ✅ All 3 specialist agents complete investigation
- ✅ Orchestrator identifies root cause (AutoScaling misconfiguration)
- ✅ Timeline shows cause and effect
- ✅ Remediation plan includes immediate, short-term, and long-term actions
- ✅ Cost recovery estimate provided

---

### 🔴 Scenario 3: HARD - Critical Incident Response

**Complexity**: 5 agents, 4 faker tools  
**Environment**: `scenario-hard-incident-response`  
**Duration**: ~4 minutes

**Architecture**:
```
Incident Commander
├── Metrics Analyst (CloudWatch faker)
│   └── Tools: get_metric_data, get_active_alarms, analyze_metric
├── Logs Analyst (Logs faker)
│   └── Tools: list_directory, read_text_file
├── Deployment Analyst (CloudTrail faker)
│   └── Tools: list_directory, read_text_file
└── Infrastructure Analyst (Terraform faker)
    └── Tools: list_directory, read_text_file
```

**Scenario**: Production outage. API returning 45% errors, RDS database connection pool exhausted, ECS tasks in CrashLoopBackoff. Incident Commander coordinates multi-team investigation to determine root cause, assess impact, and provide immediate mitigation plan.

**Expected Behavior**:
- Metrics Analyst reports RDS CPU 95%, max_connections 500/500, API 5xx errors 45%
- Logs Analyst finds "Too many connections", "Query exceeded 30s", OOMKilled events
- Deployment Analyst discovers v2.8.0 deployed at 14:25 UTC (5 min before incident)
- Deployment Analyst notes DB_POOL_SIZE changed from 50 to 500 (10x increase)
- Infrastructure Analyst calculates: 15 tasks × 500 connections = 7500 attempts to 500-max database
- Incident Commander identifies **root cause**: Configuration change created connection storm
- Incident Commander correlates deployment timing with incident start

**Success Criteria**:
- ✅ All 4 specialist agents complete analysis
- ✅ Incident Commander determines root cause (database connection storm from config change)
- ✅ Timeline shows deployment at 14:25, incident at 14:30
- ✅ Immediate mitigation: Rollback to v2.7.5, reduce DB_POOL_SIZE
- ✅ Short-term: Right-size connection pool per task
- ✅ Long-term: Implement connection pooling middleware, pre-deployment capacity testing
- ✅ Severity correctly assessed as P0

---

## Running the Tests

### Prerequisites

1. **Jaeger running** for OTEL traces:
   ```bash
   docker ps | grep jaeger
   # If not running:
   docker run -d --name jaeger \
     -p 16686:16686 \
     -p 4317:4317 \
     -p 4318:4318 \
     jaegertracing/all-in-one:latest
   ```

2. **OpenAI API key** (or alternative AI provider):
   ```bash
   export OPENAI_API_KEY="your-key-here"
   ```

3. **Station built** and in PATH:
   ```bash
   cd /home/epuerta/projects/hack/station
   make build
   ```

### Execute All Scenarios

```bash
cd /home/epuerta/projects/hack/station/test-environments
./run-prd-scenarios.sh
```

This will:
1. Sync each environment
2. Run the orchestrator agent for each scenario
3. Display results and OTEL trace links
4. Report pass/fail for all scenarios

### Execute Individual Scenarios

**Easy - Infrastructure Health Check:**
```bash
stn sync scenario-easy-health-check
stn agent run "Health Check Orchestrator" \
  "Perform a comprehensive infrastructure health check across all systems" \
  --env scenario-easy-health-check \
  --enable-telemetry --otel-endpoint http://localhost:4318 --tail
```

**Medium - Cost Spike Investigation:**
```bash
stn sync scenario-medium-cost-investigation
stn agent run "Cost Investigation Orchestrator" \
  "Investigate the AWS cost spike: identify what caused the 300% cost increase, who made the changes, and provide remediation steps" \
  --env scenario-medium-cost-investigation \
  --enable-telemetry --otel-endpoint http://localhost:4318 --tail
```

**Hard - Critical Incident Response:**
```bash
stn sync scenario-hard-incident-response
stn agent run "Incident Commander" \
  "Production incident: API is returning 45% errors, database connections exhausted, ECS tasks crashing. Investigate root cause, determine if related to recent deployment, and provide immediate mitigation plan" \
  --env scenario-hard-incident-response \
  --enable-telemetry --otel-endpoint http://localhost:4318 --tail
```

## Inspecting Results

### View Run Details
```bash
# Get run ID from output, then:
stn runs inspect <run-id> -v
```

### View Faker Sessions
```bash
# List all faker sessions
stn faker sessions list

# Replay specific session
stn faker sessions replay <session-id>
```

### View OTEL Traces
1. Open Jaeger UI: http://localhost:16686
2. Select service: `station`
3. Search for operations: `agent_execution_engine.execute`
4. Look for tags: `agent.name`, `run.id`
5. Inspect spans for faker tool calls and AI generation

## Expected Outcomes

### All Scenarios Should:
- ✅ Complete without errors (status: completed)
- ✅ Generate realistic simulated data via faker
- ✅ Demonstrate agent hierarchy coordination
- ✅ Capture complete OTEL traces
- ✅ Provide actionable insights and recommendations

### Validation Checklist

For each scenario, verify:

1. **Agent Completion**:
   - [ ] Orchestrator agent status = completed
   - [ ] All specialist agents called successfully
   - [ ] Final report generated

2. **Faker Functionality**:
   - [ ] Faker sessions created (check `stn faker sessions list`)
   - [ ] Realistic data generated (not empty/trivial responses)
   - [ ] AI instructions followed (data matches scenario)

3. **Multi-Agent Coordination**:
   - [ ] Orchestrator called all specialist agents
   - [ ] Findings from specialists synthesized
   - [ ] Correlations identified across data sources

4. **OTEL Observability**:
   - [ ] Traces visible in Jaeger
   - [ ] Agent execution spans captured
   - [ ] Tool call spans captured
   - [ ] Timing and duration data accurate

5. **Quality of Response**:
   - [ ] Root cause identified correctly
   - [ ] Timeline accurate
   - [ ] Recommendations actionable
   - [ ] Report professional and complete

## Troubleshooting

**Scenario fails to sync:**
```bash
# Check environment exists
stn env list | grep scenario

# Re-sync with verbose output
stn sync scenario-<name> --verbose
```

**Agent times out:**
- Increase faker timeout in template.json
- Check OpenAI API rate limits
- Verify Jaeger isn't causing performance issues

**Faker returns empty data:**
- Check `--ai-enabled` flag in template.json
- Verify OPENAI_API_KEY is set
- Check faker debug logs

**OTEL traces not appearing:**
```bash
# Verify Jaeger is running
curl http://localhost:16686

# Check Station OTEL config
stn agent run --enable-telemetry --otel-endpoint http://localhost:4318 ...
```

## PRD Goals Validation

These scenarios validate the PRD requirements for Station Faker:

✅ **Multi-Agent Hierarchies**: 3 orchestrators coordinating 9 specialist agents  
✅ **Multiple Faker Tools**: 9 faker MCP servers across 3 scenarios  
✅ **Real MCP Servers**: CloudWatch, ECS, filesystem, all wrapped by faker  
✅ **Complexity Progression**: Easy (2 agents) → Medium (4 agents) → Hard (5 agents)  
✅ **DevOps Focused**: Health monitoring, cost optimization, incident response  
✅ **Realistic Scenarios**: Based on actual production patterns  
✅ **OTEL Integration**: Full observability with Jaeger traces  
✅ **Actionable Outputs**: Each orchestrator provides concrete recommendations

## Architecture Highlights

### Easy Scenario
- **Agents**: 3 total (1 orchestrator + 2 specialists)
- **Faker Tools**: 2 (CloudWatch, ECS)
- **Agent Calls**: Orchestrator → 2 specialists
- **Tool Calls**: ~5-6 faker tool invocations
- **Complexity**: Linear hierarchy

### Medium Scenario
- **Agents**: 4 total (1 orchestrator + 3 specialists)
- **Faker Tools**: 3 (Billing, CloudWatch, CloudTrail)
- **Agent Calls**: Orchestrator → 3 specialists
- **Tool Calls**: ~8-10 faker tool invocations
- **Complexity**: Parallel specialist analysis with correlation

### Hard Scenario
- **Agents**: 5 total (1 orchestrator + 4 specialists)
- **Faker Tools**: 4 (CloudWatch, Logs, Deployments, Infrastructure)
- **Agent Calls**: Orchestrator → 4 specialists
- **Tool Calls**: ~12-15 faker tool invocations
- **Complexity**: Multi-layer investigation with cross-domain correlation

---

**Created**: 2025-11-09  
**Station Version**: v0.1.0  
**Status**: Ready for execution
