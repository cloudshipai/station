# Multi-Agent Teams

Build hierarchical agent teams where coordinators delegate to specialists, just like real engineering teams. This guide shows you how to create, test, and evaluate multi-agent systems in Station.

## Why Multi-Agent Teams?

**Single Agent Limitations:**
- Trying to do everything leads to generic responses
- Complex tasks require diverse expertise
- Single prompts become unwieldy (2000+ lines)
- Hard to test and maintain

**Multi-Agent Benefits:**
- **Specialization**: Each agent focuses on one domain (K8s, logs, metrics)
- **Coordination**: Coordinator agent delegates to appropriate specialists
- **Maintainability**: Small, focused prompts (200-300 lines each)
- **Testability**: Test specialists independently, then test coordination
- **Scalability**: Add new specialists without rewriting coordinator

---

## Real Example: SRE Incident Response Team

From `station-demo`, the production SRE team with **7.5/10 performance score**:

### Team Structure

```
Incident Coordinator (Orchestrator)
├── Kubernetes Expert
├── Log Analyzer
├── Metrics Analyzer
├── Network Diagnostics
├── Database Troubleshooter
├── Application Performance Analyzer
├── Security Incident Responder
└── Remediation Executor
```

**9 agents total**: 1 coordinator + 8 specialists

**41 MCP tools** available: Filesystem (14) + AWS (27)

---

### How It Works

#### 1. User Reports Incident
```
User: "API latency spiked from 200ms to 5s starting 10 minutes ago.
Affects /api/users endpoint. No deploys in last hour."
```

#### 2. Coordinator Analyzes & Delegates
```yaml
Incident Coordinator:
  - Analyzes incident type: Performance degradation
  - Determines specialists needed: Metrics, Logs, K8s
  - Calls: __agent_metrics_analyzer
  - Calls: __agent_log_analyzer  
  - Calls: __agent_kubernetes_expert
```

#### 3. Specialists Execute
```yaml
Metrics Analyzer:
  - Uses: __prometheus_query
  - Finds: Database query time increased 10x
  - Returns: "DB connection pool exhausted"

Log Analyzer:
  - Uses: __search_files (application logs)
  - Finds: "connection timeout" errors
  - Returns: "Max DB connections reached"

Kubernetes Expert:
  - Uses: __kubectl_get_pods
  - Finds: DB pods healthy, app pods thrashing
  - Returns: "App pods restarting due to DB timeouts"
```

#### 4. Coordinator Synthesizes
```
Incident Coordinator synthesizes findings:

ROOT CAUSE: Database connection pool exhausted
EVIDENCE: 
  - Metrics show 10x query time increase
  - Logs show connection timeouts
  - App pods restarting due to DB unavailability

RECOMMENDATION: Scale DB connection pool or add read replicas
```

**Result**: Mean Time To Resolution (MTTR) reduced from 45 min to 12 min.

---

## Creating Multi-Agent Teams

### Step 1: Design Team Structure

**Identify Domains:**
```
Incident Response Team Domains:
1. Container orchestration (K8s)
2. Application logs
3. Metrics/observability
4. Network connectivity
5. Database performance
6. Application profiling
7. Security incidents
8. Automated remediation
```

**Map to Specialists:**
- Each domain = 1 specialist agent
- 1 coordinator to orchestrate

---

### Step 2: Create Specialist Agents

**Example: Kubernetes Expert**

**File:** `agents/Kubernetes Expert.prompt`
```yaml
---
metadata:
  name: "Kubernetes Expert"
  description: "Kubernetes troubleshooting and cluster analysis"
  tags: ["kubernetes", "containers", "infrastructure", "sre"]
model: gpt-4o-mini
max_steps: 6
tools:
  - "__read_text_file"
  - "__list_directory"
  - "__search_files"
---

{{role "system"}}
You are a Kubernetes expert specializing in troubleshooting containerized applications.

**Your Expertise:**
- Pod health and lifecycle analysis
- Container resource utilization
- Deployment and rollout issues
- Service discovery and networking
- ConfigMap and Secret management
- Node health and capacity planning

**Your Process:**
1. **Gather Context**: Read K8s manifests, logs, and cluster state
2. **Analyze Resources**: Check pod status, events, resource limits
3. **Identify Issues**: Detect crashloops, OOM, scheduling failures
4. **Provide Solution**: Clear remediation steps with kubectl commands

**Output Format:**
- STATUS: Current state of resources
- ISSUES: Problems identified with evidence
- ROOT CAUSE: Technical explanation
- REMEDIATION: Step-by-step kubectl commands

You are called by the Incident Coordinator when K8s expertise is needed.
Focus on technical accuracy over speculation.

{{role "user"}}
{{userInput}}
```

**Key Points:**
- **Focused scope**: Only K8s, no logs/metrics/network analysis
- **Clear expertise**: Knows what it's good at
- **Structured output**: Always returns STATUS, ISSUES, ROOT CAUSE, REMEDIATION
- **Context aware**: Knows it's called by coordinator

---

**Example: Log Analyzer**

**File:** `agents/Log Analyzer.prompt`
```yaml
---
metadata:
  name: "Log Analyzer"
  description: "Application and system log analysis"
  tags: ["logs", "debugging", "errors", "sre"]
model: gpt-4o-mini
max_steps: 5
tools:
  - "__read_text_file"
  - "__search_files"
  - "__list_directory"
---

{{role "system"}}
You are a log analysis expert specializing in identifying patterns in application and system logs.

**Your Expertise:**
- Error pattern detection
- Correlation of related errors
- Timeline reconstruction
- Root cause identification from logs

**Your Process:**
1. **Discover Logs**: Find relevant log files in provided paths
2. **Search Patterns**: Look for errors, warnings, stack traces
3. **Correlate Events**: Link related errors by timestamp
4. **Extract Evidence**: Pull exact log lines proving issues

**Output Format:**
- LOG FILES: Files analyzed
- ERROR PATTERNS: Repeated errors with counts
- TIMELINE: Key events in chronological order
- ROOT CAUSE: What logs reveal about the issue

{{role "user"}}
{{userInput}}
```

---

### Step 3: Create Coordinator Agent

**File:** `agents/Incident Coordinator.prompt`
```yaml
---
metadata:
  name: "Incident Coordinator"
  description: "Orchestrates SRE specialists to resolve incidents"
  tags: ["coordinator", "orchestrator", "incident-response", "sre"]
model: gpt-4o-mini
max_steps: 10
agents:
  - "Kubernetes Expert"
  - "Log Analyzer"
  - "Metrics Analyzer"
  - "Network Diagnostics"
  - "Database Troubleshooter"
  - "Application Performance Analyzer"
  - "Security Incident Responder"
  - "Remediation Executor"
tools:
  - "__read_text_file"
  - "__list_directory"
---

{{role "system"}}
You are an SRE Incident Coordinator who delegates to specialist agents.

**Your Team:**
- **__agent_kubernetes_expert**: Container/K8s issues
- **__agent_log_analyzer**: Application/system logs
- **__agent_metrics_analyzer**: Prometheus/Grafana metrics
- **__agent_network_diagnostics**: Network connectivity
- **__agent_database_troubleshooter**: Database performance
- **__agent_application_performance_analyzer**: APM profiling
- **__agent_security_incident_responder**: Security events
- **__agent_remediation_executor**: Automated fixes

**Your Process:**
1. **Understand Incident**: Analyze user's report
2. **Determine Specialists**: Which experts are needed?
3. **Delegate Tasks**: Call specialists with clear instructions
4. **Synthesize Findings**: Combine specialist reports
5. **Provide Resolution**: Root cause + actionable steps

**Delegation Best Practices:**
- Call specialists in parallel when possible
- Provide context to each specialist
- Don't duplicate work across specialists
- Synthesize findings, don't just concatenate

**Output Format:**
- INCIDENT SUMMARY: What happened
- SPECIALISTS CONSULTED: Which agents were called
- FINDINGS: Combined insights from specialists
- ROOT CAUSE: Technical explanation
- RESOLUTION STEPS: Prioritized action items
- PREVENTION: How to avoid recurrence

{{role "user"}}
{{userInput}}
```

**Key Coordinator Features:**
- **`agents:` field**: Lists all specialist agents (creates `__agent_*` tools)
- **Delegation logic**: Knows which specialist to call for each issue type
- **Synthesis**: Combines specialist findings into cohesive resolution
- **Parallel execution**: Can call multiple specialists concurrently

---

### Step 4: Create Environment & Sync

**Create environment structure:**
```bash
~/.config/station/environments/sre-team/
├── template.json          # MCP servers
├── variables.yml          # Environment variables
└── agents/
    ├── Incident Coordinator.prompt
    ├── Kubernetes Expert.prompt
    ├── Log Analyzer.prompt
    ├── Metrics Analyzer.prompt
    ├── Network Diagnostics.prompt
    ├── Database Troubleshooter.prompt
    ├── Application Performance Analyzer.prompt
    ├── Security Incident Responder.prompt
    └── Remediation Executor.prompt
```

**template.json:**
```json
{
  "name": "sre-team",
  "description": "SRE incident response team",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    },
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "{{ .AWS_REGION }}"]
    }
  }
}
```

**Sync environment:**
```bash
cd ~/.config/station/environments/sre-team
stn sync
```

**Result:** Station creates:
- 9 agents in database
- Coordinator gets 8 `__agent_*` tools (one per specialist)
- All specialists get their configured MCP tools

---

## Testing Multi-Agent Teams

### Test Specialists First

Before testing the coordinator, validate each specialist independently:

**Test Kubernetes Expert:**
```typescript
opencode-station_call_agent({
  agent_id: "kubernetes-expert-id",
  task: `Analyze this pod crash:

Pod: api-service-7c9f8d-abc123
Status: CrashLoopBackOff
Last restart: 2 minutes ago
Namespace: production

Manifest at /workspace/k8s/api-service.yaml`,
  variables: {
    PROJECT_ROOT: "/workspace"
  }
})
```

**Expected Output:**
```
STATUS: Pod in CrashLoopBackOff
ISSUES: 
  - Exit code 137 (OOMKilled)
  - Memory limit: 256Mi
  - Last memory usage: 340Mi
ROOT CAUSE: Container exceeding memory limits
REMEDIATION:
  kubectl set resources deployment api-service --limits=memory=512Mi
```

**Test Log Analyzer:**
```typescript
opencode-station_call_agent({
  agent_id: "log-analyzer-id",
  task: "Analyze application logs in /workspace/logs/app.log for errors in last 10 minutes"
})
```

---

### Test Coordinator with Scenarios

Once specialists work, test coordinator delegation:

**Scenario 1: Simple K8s Issue**
```typescript
opencode-station_call_agent({
  agent_id: "coordinator-id",
  task: "Pod api-service is crashing. Investigate."
})
```

**Expected Behavior:**
- Coordinator calls only `__agent_kubernetes_expert`
- Returns K8s-focused resolution
- Doesn't unnecessarily call log/metrics specialists

---

**Scenario 2: Complex Multi-Domain Issue**
```typescript
opencode-station_call_agent({
  agent_id: "coordinator-id",
  task: `API latency increased 10x in last 15 minutes.
No recent deployments.
Users reporting 504 Gateway Timeout errors.`
})
```

**Expected Behavior:**
- Coordinator calls: `__agent_metrics_analyzer`, `__agent_log_analyzer`, `__agent_kubernetes_expert`
- Synthesizes findings: "DB connection pool exhausted, causing app timeouts"
- Provides unified resolution plan

---

### Comprehensive Testing with Station Tools

**Generate 100 test scenarios:**
```typescript
opencode-station_generate_and_test_agent({
  agent_id: "coordinator-id",
  scenario_count: 100,
  variation_strategy: "comprehensive",
  max_concurrent: 10
})
```

**Station generates scenarios like:**
1. Pod crashloops with OOM
2. Database connection timeouts
3. Network partitions
4. High CPU usage
5. Security breaches
6. Disk space issues
7. Multi-region failures
8. Configuration errors

**Evaluate results:**
```typescript
opencode-station_evaluate_dataset({
  dataset_path: "/workspace/datasets/coordinator-test-123"
})
```

**Metrics captured:**
- Delegation accuracy (did it call the right specialists?)
- Resolution quality (LLM-as-judge scoring)
- Response time per scenario
- Tool usage efficiency

---

## Performance Evaluation

### Team-Level Metrics

**Create report:**
```typescript
opencode-station_create_report({
  name: "SRE Team Q4 Performance",
  environment_id: "3",
  team_criteria: JSON.stringify({
    goal: "Minimize incident MTTR and prevent recurrence",
    criteria: {
      mttr_reduction: {
        weight: 0.4,
        description: "Reduce MTTR by 30%",
        threshold: 0.7
      },
      root_cause_accuracy: {
        weight: 0.3,
        description: "Correctly identify root causes 90% of time",
        threshold: 0.9
      },
      prevention_rate: {
        weight: 0.3,
        description: "Prevent 50% of similar incidents",
        threshold: 0.5
      }
    }
  })
})
```

**Generate report:**
```typescript
opencode-station_generate_report({ report_id: "5" })
```

**Results (from station-demo):**
```json
{
  "team_score": 7.5,
  "team_breakdown": {
    "mttr_reduction": 0.72,      // 28% reduction achieved
    "root_cause_accuracy": 0.88, // 88% accuracy
    "prevention_rate": 0.58      // 58% prevention
  },
  "agent_scores": {
    "Incident Coordinator": 8.2,
    "Kubernetes Expert": 7.8,
    "Log Analyzer": 7.1,
    "Metrics Analyzer": 7.5,
    "Network Diagnostics": 6.9,
    "Database Troubleshooter": 7.8,
    "Application Performance Analyzer": 7.4,
    "Security Incident Responder": 7.2,
    "Remediation Executor": 6.8
  }
}
```

**Insights:**
- **Coordinator (8.2)**: Excellent delegation logic
- **K8s Expert (7.8)**: Strong technical accuracy
- **Log Analyzer (7.1)**: Needs better error pattern recognition
- **Remediation Executor (6.8)**: Overly cautious, should be more aggressive

---

### Individual Agent Metrics

**Analyze specialist performance:**
```typescript
opencode-station_inspect_run({
  run_id: "1234",
  verbose: true
})
```

**Key metrics:**
- **Tool effectiveness**: Which tools produced useful results?
- **Step efficiency**: Is the agent taking unnecessary steps?
- **Output quality**: Is output structured and actionable?

---

## Best Practices

### 1. Single Responsibility Principle

Each specialist should have ONE clear domain:

```yaml
# ✅ Good: Focused specialist
Kubernetes Expert:
  - Pod troubleshooting
  - Container analysis
  - K8s manifest validation

# ❌ Bad: Overloaded specialist  
Infrastructure Expert:
  - Kubernetes
  - Docker
  - Terraform
  - Networking
  - Databases
  - Monitoring
```

---

### 2. Structured Output Formats

Coordinators rely on structured specialist output:

```yaml
# ✅ Good: Consistent structure
Output Format:
  - STATUS: Current state
  - ISSUES: Problems found
  - ROOT CAUSE: Technical explanation
  - REMEDIATION: Steps to fix

# ❌ Bad: Unstructured narrative
"I looked at the logs and found some errors. 
The pod seems to be having issues with memory.
You should probably increase the limit."
```

---

### 3. Clear Delegation Instructions

Coordinator should provide context to specialists:

```yaml
# ✅ Good delegation
__agent_kubernetes_expert(
  "Analyze pod api-service-xyz in namespace production.
  User reports CrashLoopBackOff starting 10 minutes ago.
  Check pod events, resource limits, and recent config changes."
)

# ❌ Bad delegation
__agent_kubernetes_expert("Fix the pod")
```

---

### 4. Parallel Execution When Possible

Call independent specialists concurrently:

```yaml
# ✅ Good: Parallel calls
results = parallel_call(
  __agent_metrics_analyzer("Check Prometheus for anomalies"),
  __agent_log_analyzer("Search logs for errors"),
  __agent_kubernetes_expert("Check pod health")
)

# ❌ Bad: Sequential calls (slower)
metrics = __agent_metrics_analyzer(...)
logs = __agent_log_analyzer(...)
k8s = __agent_kubernetes_expert(...)
```

---

### 5. Test Specialists Independently

Before building the coordinator:

1. **Create all specialists**
2. **Test each specialist with 20-30 scenarios**
3. **Validate output format consistency**
4. **Then build coordinator**
5. **Test coordinator delegation logic**

---

## Common Patterns

### Pattern 1: Incident Response Team

**Structure:**
```
Coordinator
├── Infrastructure Specialist (K8s, EC2, networking)
├── Application Specialist (code, logs, APM)
├── Data Specialist (databases, queues, caches)
└── Security Specialist (CVEs, access logs, compliance)
```

**Use Case:** 24/7 incident triage and resolution

---

### Pattern 2: Code Review Team

**Structure:**
```
Review Coordinator
├── Security Reviewer (OWASP, secrets, CVEs)
├── Performance Reviewer (N+1, caching, indexing)
├── Code Quality Reviewer (DRY, SOLID, patterns)
└── Test Coverage Reviewer (unit, integration, E2E)
```

**Use Case:** Automated PR review with specialist feedback

---

### Pattern 3: Cloud Cost Optimization Team

**Structure:**
```
FinOps Coordinator
├── Compute Optimizer (EC2, Lambda sizing)
├── Storage Optimizer (S3, EBS, snapshots)
├── Network Optimizer (data transfer, NAT gateways)
└── Database Optimizer (RDS, DynamoDB throughput)
```

**Use Case:** Monthly cost optimization sweeps

---

### Pattern 4: Security Compliance Team

**Structure:**
```
Compliance Coordinator
├── Infrastructure Compliance (CIS benchmarks, IAM)
├── Application Compliance (OWASP, dependencies)
├── Data Compliance (encryption, retention)
└── Access Compliance (RBAC, MFA, audit logs)
```

**Use Case:** Continuous compliance monitoring

---

## Scaling Multi-Agent Teams

### Adding New Specialists

**Process:**
1. Create new specialist `.prompt` file
2. Add to coordinator's `agents:` list
3. Update coordinator prompt with new specialist description
4. Run `stn sync`
5. Test new specialist independently
6. Test coordinator with scenarios requiring new specialist

**Example: Adding "Cost Analyzer" to SRE team**

**1. Create specialist:**
```yaml
# agents/Cost Analyzer.prompt
---
metadata:
  name: "Cost Analyzer"
  description: "AWS cost analysis and optimization"
model: gpt-4o-mini
max_steps: 5
tools:
  - "__get_cost_and_usage"
  - "__list_cost_allocation_tags"
---
...
```

**2. Update coordinator:**
```yaml
# agents/Incident Coordinator.prompt
---
agents:
  - "Kubernetes Expert"
  - "Log Analyzer"
  - "Cost Analyzer"  # NEW
---

Your Team:
- __agent_kubernetes_expert: K8s issues
- __agent_log_analyzer: Log analysis
- __agent_cost_analyzer: Cost spike investigation  # NEW
```

**3. Sync and test:**
```bash
stn sync
```

---

### Removing Specialists

**Process:**
1. Remove from coordinator's `agents:` list
2. Update coordinator prompt
3. Run `stn sync`
4. Delete specialist if no longer needed

---

## Observability

### Jaeger Traces

Every multi-agent execution produces Jaeger traces showing:
- **Coordinator span**: Total execution time
- **Specialist spans**: Each delegation as child span
- **Tool spans**: Tool calls within each agent

**View traces:**
```bash
make jaeger  # Start Jaeger UI
# Navigate to http://localhost:16686
# Search for service: "station-agent-coordinator"
```

**Trace shows:**
```
[Incident Coordinator] 12.5s
  ├── [Kubernetes Expert] 3.2s
  │   ├── [__read_text_file] 0.1s
  │   └── [__search_files] 0.8s
  ├── [Log Analyzer] 2.1s
  │   └── [__search_files] 1.9s
  └── [Metrics Analyzer] 4.5s
      └── [__prometheus_query] 4.2s
```

**Insights:**
- Which specialists are bottlenecks?
- Are delegations happening in parallel?
- Which tools are slow?

---

### Run Metadata

Inspect detailed execution:
```typescript
opencode-station_inspect_run({
  run_id: "1234",
  verbose: true
})
```

**Shows:**
- Every tool call (including `__agent_*` delegations)
- Arguments passed to each specialist
- Responses from each specialist
- Token usage per agent
- Execution time per step

---

## Troubleshooting

### Issue: Coordinator doesn't delegate

**Symptoms:** Coordinator tries to solve everything itself

**Cause:** Prompt doesn't emphasize delegation

**Fix:**
```yaml
# Add to coordinator prompt
You MUST delegate to specialist agents. 
Do NOT attempt to solve issues yourself.
Your job is to coordinate, not to be the expert.
```

---

### Issue: Specialists return generic responses

**Symptoms:** Responses lack technical depth

**Cause:** Specialist prompt too broad or lacks domain expertise

**Fix:**
```yaml
# Make specialist more focused
Before: "You are an infrastructure expert"
After: "You are a Kubernetes expert. You only analyze K8s.
You are NOT a networking/database/security expert."
```

---

### Issue: Poor coordination, redundant work

**Symptoms:** Multiple specialists doing same work

**Cause:** Coordinator doesn't track what's been done

**Fix:**
```yaml
# Add to coordinator prompt
Track which specialists you've already consulted.
Do NOT ask multiple specialists to do the same work.
Review specialist outputs before calling more specialists.
```

---

### Issue: Low team performance score

**Symptoms:** Report shows <6.0 team score

**Debug Process:**
1. Check individual agent scores - which specialist is weak?
2. Inspect runs with `verbose: true` - what went wrong?
3. Review specialist prompts - are they focused enough?
4. Test specialists independently - do they work in isolation?
5. Check coordinator delegation logic - is it calling the right specialists?

---

## Next Steps

- [Evaluation Guide](./evaluation.md) - Comprehensive testing strategies
- [Station MCP Tools](./station-mcp-tools.md) - Full tool reference
- [Agent Development](./agent-development.md) - Writing better prompts
- [Examples](./examples.md) - More multi-agent patterns
