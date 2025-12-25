# DevOps Workflows - Design & Test Plan

This document defines the real-world DevOps workflows we'll create and test using Station's workflow engine and AI agents.

## Available Agents (by Category)

### Reliability/SRE Agents
| Agent | Purpose |
|-------|---------|
| `incident-root-cause-analyzer` | Analyzes incidents using CloudWatch, X-Ray, logs |
| `latency-spike-rca` | API latency spike root cause analysis |
| `error-burst-rca` | Error rate spike investigation |
| `slo-breach-rca` | SLO violation analysis |
| `cascade-failure-rca` | Cascading failure analysis |
| `capacity-exhaustion-rca` | Resource exhaustion investigation |

### Security Agents
| Agent | Purpose |
|-------|---------|
| `guardduty-threat-rca` | AWS GuardDuty threat analysis |
| `privilege-escalation-rca` | IAM privilege escalation investigation |
| `public-exposure-rca` | Public resource exposure analysis |
| `secrets-leak-rca` | Secrets/credential leak investigation |
| `vuln-exploitability-rca` | Vulnerability exploitability assessment |
| `iam-least-privilege-advisor` | IAM policy optimization |
| `patch-priority-planner` | Patch prioritization recommendations |

### FinOps Agents
| Agent | Purpose |
|-------|---------|
| `aws-cost-spike-analyzer` | Cost anomaly investigation |
| `aws-cost-inventory` | Resource cost breakdown |
| `rightsizing-advisor-lite` | EC2/RDS rightsizing recommendations |
| `orphaned-resource-cleaner` | Unused resource identification |
| `k8s-resource-optimizer` | Kubernetes resource optimization |
| `monthly-cost-forecaster` | Cost projection analysis |

### Deployment Agents
| Agent | Purpose |
|-------|---------|
| `dora-metrics-improver` | DORA metrics analysis |
| `rollback-safety-enhancer` | Rollback readiness assessment |
| `build-cache-optimizer` | CI/CD build optimization |

---

## Workflow 1: Production Incident Response

**Trigger**: Alert webhook or manual  
**Purpose**: Automated incident triage with parallel diagnosis and approval-gated remediation

```yaml
id: incident-response
name: Production Incident Response
description: Automated incident triage with parallel diagnosis and human-in-the-loop remediation

Flow:
  1. [inject] Initialize incident context from trigger
  2. [parallel] Run diagnostic agents concurrently:
     - incident-root-cause-analyzer
     - latency-spike-rca  
     - slo-breach-rca
  3. [agent] Synthesize findings into remediation plan
  4. [switch] Route based on severity:
     - critical → human.approval → auto-remediate
     - warning → log and notify
     - info → log only
  5. [agent] Execute approved remediation
  6. [agent] Generate post-incident report
```

### Step Types Covered
- `inject` - Initialize workflow state
- `parallel` - Concurrent agent execution
- `agent` - Single agent task
- `switch` - Conditional routing
- `human.approval` - Human-in-the-loop gate
- `operation` - Generic task execution

---

## Workflow 2: Canary Deployment Validation

**Trigger**: CI/CD webhook on deployment start  
**Purpose**: Validate canary deployment with progressive rollout gates

```yaml
id: canary-validation
name: Canary Deployment Validation
description: Progressive deployment validation with automated rollback

Flow:
  1. [inject] Capture deployment metadata (version, service, namespace)
  2. [timer] Wait 2 minutes for canary to stabilize
  3. [parallel] Initial health checks:
     - Check pod status (k8s-investigator)
     - Check error rates (latency-spike-rca)
  4. [switch] Evaluate initial health:
     - unhealthy → rollback branch
     - healthy → continue
  5. [timer] Wait 5 minutes for traffic analysis
  6. [agent] Deep canary analysis (dora-metrics-improver)
  7. [switch] Evaluate canary metrics:
     - pass → proceed to full rollout
     - fail → human.approval for override or rollback
  8. [agent] Execute rollout decision
  9. [inject] Record deployment outcome
```

### Step Types Covered
- `timer` - Delay for stabilization windows
- `parallel` - Concurrent health checks
- `switch` - Progressive gate evaluation
- `human.approval` - Override gate for edge cases
- `try_catch` - Rollback on failure

---

## Workflow 3: Daily Cost Anomaly Scan

**Trigger**: Cron (9 AM daily)  
**Purpose**: Scheduled cost monitoring with conditional alerting

```yaml
id: daily-cost-scan
name: Daily Cost Anomaly Scan
description: Scheduled cost monitoring with threshold-based alerting

Flow:
  1. [cron] Trigger at 9 AM CST daily
  2. [foreach] Scan each AWS account:
     - aws-cost-inventory (get current spend)
     - aws-cost-spike-analyzer (detect anomalies)
  3. [agent] Aggregate findings across accounts
  4. [switch] Route based on anomaly severity:
     - high → alert + human.approval for investigation
     - medium → create ticket
     - low → log only
  5. [agent] Generate daily cost report (monthly-cost-forecaster)
  6. [inject] Store report for dashboard
```

### Step Types Covered
- `cron` - Scheduled execution
- `foreach` - Iterate over multiple accounts
- `switch` - Threshold-based routing
- `agent` - Analysis and reporting

---

## Workflow 4: Security Vulnerability Remediation

**Trigger**: Security scanner webhook  
**Purpose**: Automated vulnerability assessment with approval-gated patching

```yaml
id: security-remediation
name: Security Vulnerability Remediation
description: Vulnerability assessment with prioritized, approval-gated patching

Flow:
  1. [inject] Ingest vulnerability scan results
  2. [foreach] For each vulnerability:
     - [agent] vuln-exploitability-rca (assess exploitability)
     - [switch] Prioritize:
       - critical+exploitable → immediate queue
       - high → scheduled queue
       - medium/low → backlog
  3. [parallel] For critical vulnerabilities:
     - [agent] patch-priority-planner (create patch plan)
     - [agent] iam-least-privilege-advisor (check blast radius)
  4. [human.approval] Approve patch execution
  5. [try_catch] Apply patches:
     - try: Execute patch
     - catch: Rollback and alert
  6. [agent] Verify remediation success
  7. [inject] Update vulnerability status
```

### Step Types Covered
- `foreach` - Process vulnerability list
- `parallel` - Concurrent assessment
- `switch` - Priority-based routing
- `human.approval` - Patch approval gate
- `try_catch` - Safe patching with rollback

---

## Workflow 5: Infrastructure Drift Detection

**Trigger**: Cron (hourly) or on-demand  
**Purpose**: Detect and alert on infrastructure drift from declared state

```yaml
id: drift-detection
name: Infrastructure Drift Detection
description: Hourly scan for infrastructure configuration drift

Flow:
  1. [cron] Trigger hourly (or manual)
  2. [parallel] Scan infrastructure layers:
     - [agent] k8s-resource-optimizer (K8s drift)
     - [agent] aws-cost-inventory (AWS resource drift)
     - [agent] iam-least-privilege-advisor (IAM drift)
  3. [agent] Correlate findings (infrastructure-drift-detector)
  4. [switch] Route based on drift severity:
     - critical (security) → immediate alert + approval
     - warning (cost) → create ticket
     - info → log only
  5. [human.approval] (if critical) Approve drift correction
  6. [agent] Execute approved corrections
  7. [inject] Record drift metrics for trending
```

### Step Types Covered
- `cron` - Scheduled execution
- `parallel` - Multi-layer scanning
- `switch` - Severity routing
- `human.approval` - Correction approval

---

## Test Execution Plan

### Phase 1: Create Workflows
```bash
# Create each workflow via API
curl -X POST http://localhost:8585/api/v1/workflows -d @incident-response.json
curl -X POST http://localhost:8585/api/v1/workflows -d @canary-validation.json
curl -X POST http://localhost:8585/api/v1/workflows -d @daily-cost-scan.json
curl -X POST http://localhost:8585/api/v1/workflows -d @security-remediation.json
curl -X POST http://localhost:8585/api/v1/workflows -d @drift-detection.json
```

### Phase 2: Test Each Step Type
| Step Type | Workflow | Expected Behavior |
|-----------|----------|-------------------|
| `inject` | All | State initialization works |
| `agent` | All | Agent execution returns structured output |
| `parallel` | incident-response | All branches complete, results merged |
| `foreach` | daily-cost-scan | Iterates correctly, handles empty lists |
| `switch` | All | Correct routing based on conditions |
| `timer` | canary-validation | Delays execution for specified duration |
| `cron` | daily-cost-scan | Triggers on schedule |
| `human.approval` | incident-response | Pauses until approved via API |
| `try_catch` | security-remediation | Catches errors, executes rollback |

### Phase 3: E2E Scenarios
1. **Happy Path**: Run each workflow with normal inputs
2. **Error Handling**: Inject agent failures, verify try_catch
3. **Recovery**: Kill server mid-workflow, verify recovery on restart
4. **Approval Flow**: Test approve/reject paths for human.approval
5. **Cron Reliability**: Verify cron triggers across server restarts

---

## Success Criteria

- [ ] All 5 workflows created successfully
- [ ] Each step type executes correctly
- [ ] Parallel branches complete and merge results
- [ ] Switch routing works with real agent output
- [ ] Human approval pauses/resumes correctly
- [ ] Timer delays work and survive restarts
- [ ] Cron schedules trigger reliably
- [ ] Try/catch handles agent failures gracefully
- [ ] Workflow recovery works after server restart

---

## Current Blocker (2025-12-25)

### 🔴 NATS Consumer Not Receiving New Messages After Startup

**Status**: Root cause identified, fix pending

**Problem**: Workflow runs with `inject` steps are created but remain stuck at "pending" status. The NATS push consumer receives messages at startup (recovery works), but stops receiving NEW messages published after startup.

**Impact**: Cannot test DevOps workflows end-to-end until this is fixed.

**Root Cause**: JetStream push consumer with `DeliverAll()` policy stops receiving after processing the initial batch of pending messages.

**Proposed Fix**: Convert to pull-based consumer that continuously fetches messages.

**See**: `docs/features/workflow-engine-v1.md` Section 12 for full debug log and proposed fixes.

**Next Steps**:
1. Implement pull-based consumer fix in `internal/workflows/runtime/nats_engine.go`
2. Rebuild and verify workflow runs complete
3. Continue with DevOps workflow testing
