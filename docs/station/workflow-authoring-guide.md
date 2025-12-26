# Workflow Authoring Guide

This guide teaches you how to write Station workflows from scratch. Whether you're automating incident response, deployment pipelines, or scheduled health checks, this document covers everything you need to know.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Workflow Structure](#workflow-structure)
3. [State Types](#state-types)
4. [Data Flow & Context](#data-flow--context)
5. [Expressions & Starlark](#expressions--starlark)
6. [Complete Examples](#complete-examples)
7. [Best Practices](#best-practices)
8. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Your First Workflow

Create a file `~/.config/station/environments/default/workflows/hello-world.workflow.yaml`:

```yaml
id: hello-world
name: "Hello World Workflow"
version: "1.0.0"
description: "A simple workflow that demonstrates basic concepts"

start: greet

states:
  - id: greet
    type: inject
    data:
      message: "Hello from Station!"
      timestamp: "2025-01-01T00:00:00Z"
    output:
      greeting: "$.data"
    transition: done

  - id: done
    type: inject
    data:
      status: "completed"
    output:
      final_status: "$.data"
    end: true
```

Run it:
```bash
stn sync  # Load the workflow
stn workflow run hello-world --wait
```

---

## Workflow Structure

### Anatomy of a Workflow Definition

```
┌─────────────────────────────────────────────────────────────────────┐
│                        WORKFLOW DEFINITION                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ HEADER                                                       │   │
│  │  id: unique-workflow-id                                      │   │
│  │  name: "Human Readable Name"                                 │   │
│  │  version: "1.0.0"                                            │   │
│  │  description: "What this workflow does"                      │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ INPUT/OUTPUT SCHEMAS (optional)                              │   │
│  │  inputSchema:                                                │   │
│  │    type: object                                              │   │
│  │    properties:                                               │   │
│  │      service_name: { type: string }                          │   │
│  │    required: [service_name]                                  │   │
│  │                                                              │   │
│  │  outputSchema:                                               │   │
│  │    type: object                                              │   │
│  │    properties:                                               │   │
│  │      result: { type: object }                                │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ ENTRY POINT                                                  │   │
│  │  start: first-state-id                                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ STATES (the actual steps)                                    │   │
│  │  states:                                                     │   │
│  │    - id: state-1                                             │   │
│  │      type: inject|agent|switch|parallel|foreach|transform   │   │
│  │      ...                                                     │   │
│  │      transition: state-2                                     │   │
│  │                                                              │   │
│  │    - id: state-2                                             │   │
│  │      ...                                                     │   │
│  │      end: true                                               │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Top-Level Fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique identifier (used in CLI: `stn workflow run <id>`) |
| `name` | Yes | Human-readable name shown in UI |
| `version` | No | Semantic version (e.g., "1.0.0") |
| `description` | No | What the workflow does |
| `inputSchema` | No | JSON Schema for workflow input validation |
| `outputSchema` | No | JSON Schema for workflow output validation |
| `start` | Yes | ID of the first state to execute |
| `states` | Yes | Array of state definitions |

### State Transitions

Every state (except terminal states) must specify where to go next:

```yaml
# Option 1: transition to another state
transition: next-state-id

# Option 2: end the workflow
end: true

# For switch states, conditions define transitions
conditions:
  - if: "expression"
    next: some-state
defaultNext: fallback-state
```

---

## State Types

### Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         STATE TYPES                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐  Execute an AI agent                              │
│  │   AGENT     │  Calls Station agent by name                      │
│  └─────────────┘  agent: "my-agent"                                │
│                                                                     │
│  ┌─────────────┐  Inject static data                               │
│  │   INJECT    │  Sets values in workflow context                  │
│  └─────────────┘  data: { key: "value" }                           │
│                                                                     │
│  ┌─────────────┐  Conditional branching                            │
│  │   SWITCH    │  Routes based on Starlark expressions             │
│  └─────────────┘  conditions: [{ if: "expr", next: "state" }]      │
│                                                                     │
│  ┌─────────────┐  Transform data                                   │
│  │  TRANSFORM  │  Starlark expression to reshape data              │
│  └─────────────┘  expression: "{ 'key': value }"                   │
│                                                                     │
│  ┌─────────────┐  Parallel execution                               │
│  │  PARALLEL   │  Run multiple branches concurrently               │
│  └─────────────┘  branches: [{ id: "b1", states: [...] }]          │
│                                                                     │
│  ┌─────────────┐  Loop over array                                  │
│  │   FOREACH   │  Iterate with optional concurrency                │
│  └─────────────┘  itemsPath: "$.items", maxConcurrency: 3          │
│                                                                     │
│  ┌─────────────┐  Human approval gate                              │
│  │  APPROVAL   │  Blocks until human approves/rejects              │
│  └─────────────┘  type: human_approval                             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 1. Agent State (`type: agent`)

Executes a Station agent and captures its output.

```yaml
- id: investigate-logs
  name: "Investigate Logs"
  type: agent
  agent: log-analyzer           # Agent name (must exist in environment)
  input:
    service_name: "$.input.service_name"    # JSONPath from workflow input
    time_range_minutes: 60                   # Static value
  output:
    log_analysis: "$.result"    # Store agent output in context
  timeout: 2m                   # Agent execution timeout
  transition: next-step
```

**Key Fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `agent` | Yes | Agent name (resolved globally or with `@env` suffix) |
| `input` | No | Map of inputs to pass to the agent (supports JSONPath) |
| `output` | No | Map of context keys to store results |
| `timeout` | No | Execution timeout (e.g., "30s", "2m", "1h") |

### 2. Inject State (`type: inject`)

Injects static or computed data into the workflow context.

```yaml
- id: set-config
  name: "Set Configuration"
  type: inject
  data:
    thresholds:
      error_rate: 0.05
      latency_p99: 500
    regions:
      - us-east-1
      - us-west-2
    timestamp: "$.input.incident_time"   # Can use JSONPath
  output:
    config: "$.data"            # Store under context.config
  transition: use-config
```

### 3. Switch State (`type: switch`)

Conditional branching based on Starlark expressions.

```yaml
- id: route-by-severity
  name: "Route by Severity"
  type: switch
  dataPath: "$.analysis_result"           # Context path to evaluate
  conditions:
    - if: "hasattr(analysis_result, 'severity') and analysis_result.severity == 'critical'"
      next: escalate
    - if: "analysis_result.severity == 'high'"
      next: alert-oncall
    - if: "analysis_result.severity == 'medium'"
      next: create-ticket
  defaultNext: log-and-close              # Fallback if no condition matches
```

**Important:** Use `hasattr()` to check if a field exists before accessing it. This prevents errors when agent outputs are incomplete.

### 4. Transform State (`type: transform`)

Reshape data using Starlark expressions.

```yaml
- id: build-report
  name: "Build Final Report"
  type: transform
  expression: |
    # Multi-line Starlark expression
    # All context variables are available directly
    
    report = {
      "service_name": getattr(input, "service_name", "unknown"),
      "root_cause": getattr(analysis_result, "root_cause", {}),
      "recommended_actions": getattr(analysis_result, "actions", []),
      "severity": getattr(analysis_result, "severity", "unknown"),
      "generated_at": "2025-01-01T00:00:00Z"
    }
    
    # The last expression is the output
    report
  output:
    final_report: "$.result"
  end: true
```

**Transform Expression Rules:**
- Use standard Starlark syntax (Python-like)
- All workflow context variables are available as globals
- Use `getattr(obj, "field", default)` for safe field access
- Use `hasattr(obj, "field")` to check field existence
- The last expression becomes the output
- Comments with `#` are supported

### 5. Parallel State (`type: parallel`)

Execute multiple branches concurrently.

```yaml
- id: gather-diagnostics
  name: "Gather Diagnostics in Parallel"
  type: parallel
  branches:
    - id: check-pods
      start: get-pod-status
      states:
        - id: get-pod-status
          type: agent
          agent: k8s-investigator
          input:
            namespace: "$.input.namespace"
          output:
            pod_data: "$.result"
          end: true

    - id: check-logs
      start: get-logs
      states:
        - id: get-logs
          type: agent
          agent: log-analyzer
          input:
            service: "$.input.service_name"
          output:
            log_data: "$.result"
          end: true

    - id: check-metrics
      start: get-metrics
      states:
        - id: get-metrics
          type: agent
          agent: metrics-collector
          input:
            service: "$.input.service_name"
          output:
            metrics_data: "$.result"
          end: true
  output:
    investigation_results: "$.branches"   # All branch outputs merged
  transition: synthesize
```

**Parallel Execution Diagram:**

```
                    ┌───────────────────┐
                    │  gather-diagnostics│
                    │    (parallel)      │
                    └─────────┬─────────┘
                              │
           ┌──────────────────┼──────────────────┐
           │                  │                  │
           ▼                  ▼                  ▼
   ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
   │  check-pods   │  │  check-logs   │  │ check-metrics │
   │ (k8s-invest.) │  │(log-analyzer) │  │(metrics-coll.)│
   └───────────────┘  └───────────────┘  └───────────────┘
           │                  │                  │
           └──────────────────┼──────────────────┘
                              │
                        (wait for all)
                              │
                              ▼
                    ┌───────────────────┐
                    │    synthesize     │
                    └───────────────────┘
```

### 6. Foreach State (`type: foreach`)

Iterate over an array, optionally with concurrency.

```yaml
- id: check-all-services
  name: "Check All Services"
  type: foreach
  itemsPath: "$.input.services"           # JSONPath to array
  itemName: "service"                      # Variable name for current item
  maxConcurrency: 3                        # Run up to 3 in parallel
  iterator:
    start: health-check
    states:
      - id: health-check
        type: agent
        agent: health-checker
        input:
          service_name: "$.service.name"   # Access current item
          namespace: "$.service.namespace"
        output:
          health_result: "$.result"
        end: true
  output:
    all_health_results: "$.iterations"
  transition: summarize
```

**Foreach Execution Diagram (maxConcurrency: 2):**

```
    Input: services = [svc-a, svc-b, svc-c, svc-d]

    ┌─────────────────────────────────────────────┐
    │              check-all-services              │
    │                 (foreach)                    │
    └─────────────────────────────────────────────┘
                         │
          ┌──────────────┴──────────────┐
          │                             │
          ▼                             ▼
    ┌───────────┐                 ┌───────────┐
    │  svc-a    │                 │  svc-b    │    Batch 1 (concurrent)
    └───────────┘                 └───────────┘
          │                             │
          ▼                             ▼
        done                          done
          │                             │
          └──────────────┬──────────────┘
                         │
          ┌──────────────┴──────────────┐
          │                             │
          ▼                             ▼
    ┌───────────┐                 ┌───────────┐
    │  svc-c    │                 │  svc-d    │    Batch 2 (concurrent)
    └───────────┘                 └───────────┘
          │                             │
          ▼                             ▼
        done                          done
          │                             │
          └──────────────┬──────────────┘
                         │
                         ▼
                   ┌───────────┐
                   │ summarize │
                   └───────────┘
```

### 7. Human Approval State (`type: human_approval`)

Block workflow execution until a human approves or rejects.

```yaml
- id: request-deploy-approval
  name: "Request Deployment Approval"
  type: human_approval
  approval_title: "Production Deployment Approval"
  message: |
    **Deployment Request**
    
    Service: {{input.service_name}}
    Environment: {{input.environment}}
    Version: {{build_result.version}}
    
    Changes:
    {{build_result.changelog}}
    
    Do you approve this deployment?
  approvers:
    - platform-leads
    - oncall-sre
  timeout: 30m                            # Auto-reject after 30 minutes
  transition: deploy                      # After approval
```

**Approval Flow:**

```
    ┌─────────────────────────────────────┐
    │       request-deploy-approval        │
    │         (human_approval)             │
    └─────────────────┬───────────────────┘
                      │
                      ▼
              ┌───────────────┐
              │   WAITING     │◄──────────────┐
              │  (blocked)    │               │
              └───────┬───────┘               │
                      │                       │
         ┌────────────┼────────────┐          │
         │            │            │          │
         ▼            ▼            ▼          │
    ┌─────────┐ ┌──────────┐ ┌─────────┐      │
    │ APPROVE │ │  REJECT  │ │ TIMEOUT │      │
    └────┬────┘ └────┬─────┘ └────┬────┘      │
         │           │            │           │
         ▼           ▼            ▼           │
    ┌─────────┐ ┌──────────┐ ┌─────────┐      │
    │ deploy  │ │  failed  │ │  failed │      │
    │ (next)  │ │  (end)   │ │  (end)  │      │
    └─────────┘ └──────────┘ └─────────┘      │
```

---

## Data Flow & Context

### Understanding Workflow Context

The workflow context is a JSON object that accumulates data as the workflow executes.

```
┌─────────────────────────────────────────────────────────────────────┐
│                       WORKFLOW CONTEXT                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  {                                                                  │
│    "input": {                    // Original workflow input         │
│      "service_name": "api",                                         │
│      "namespace": "production"                                      │
│    },                                                               │
│                                                                     │
│    "workflow": {                 // Workflow metadata               │
│      "id": "incident-rca",                                          │
│      "version": "1.0.0"                                             │
│    },                                                               │
│                                                                     │
│    "config": { ... },            // From inject step                │
│    "pod_data": { ... },          // From agent step output          │
│    "log_analysis": { ... },      // From another agent step         │
│                                                                     │
│    "steps": {                    // Step outputs (auto-stored)      │
│      "check-pods": { "output": {...} },                             │
│      "analyze-logs": { "output": {...} }                            │
│    }                                                                │
│  }                                                                  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### JSONPath References

Use JSONPath to reference values in the context:

| Expression | Description |
|------------|-------------|
| `$.input.field` | Access workflow input |
| `$.field` | Access top-level context field |
| `$.steps.step_id.output` | Access specific step output |
| `$.branches.branch_id.field` | Access parallel branch output |
| `$.service.name` | In foreach: access current item field |

### Input Mapping

Map context values to step inputs:

```yaml
- id: analyze
  type: agent
  agent: analyzer
  input:
    # Static value
    max_results: 10
    
    # From workflow input
    target_service: "$.input.service_name"
    
    # From previous step output
    pod_list: "$.pod_data.pods"
    
    # From inject step
    threshold: "$.config.thresholds.error_rate"
```

### Output Mapping

Store step results in context:

```yaml
- id: investigate
  type: agent
  agent: investigator
  output:
    # Store entire result
    investigation: "$.result"
    
    # Store specific field (if agent outputs JSON)
    findings: "$.result.findings"
    severity: "$.result.severity"
```

---

## Expressions & Starlark

### Switch Conditions

Switch conditions use Starlark expressions. The `dataPath` value is available as a variable.

```yaml
- id: route
  type: switch
  dataPath: "$.analysis_result"           # Available as 'analysis_result'
  conditions:
    # Simple comparison
    - if: "analysis_result.severity == 'critical'"
      next: escalate
    
    # Numeric comparison
    - if: "analysis_result.error_rate > 0.5"
      next: high-alert
    
    # Safe field access (RECOMMENDED)
    - if: "hasattr(analysis_result, 'rollback_needed') and analysis_result.rollback_needed"
      next: rollback
    
    # Boolean field
    - if: "analysis_result.is_healthy"
      next: all-clear
    
    # String contains
    - if: "'timeout' in analysis_result.error_type"
      next: timeout-handler
      
  defaultNext: unknown-state
```

### Transform Expressions

Transform expressions are full Starlark programs that reshape data.

```yaml
- id: build-output
  type: transform
  expression: |
    # Access context variables directly
    # 'input', 'pod_data', 'log_analysis' etc. are all available
    
    # Safe field access with getattr
    service = getattr(input, "service_name", "unknown")
    root_cause = getattr(analysis_result, "root_cause", {})
    
    # Conditional logic
    if hasattr(analysis_result, "rollback_recommended"):
        needs_rollback = analysis_result.rollback_recommended
    else:
        needs_rollback = False
    
    # Build output object
    {
        "service_name": service,
        "root_cause": root_cause,
        "needs_rollback": needs_rollback,
        "timestamp": "2025-01-01T00:00:00Z",
        "recommendations": getattr(analysis_result, "recommendations", [])
    }
  output:
    final_report: "$.result"
```

### Built-in Functions

| Function | Description | Example |
|----------|-------------|---------|
| `hasattr(obj, "field")` | Check if field exists | `hasattr(result, "error")` |
| `getattr(obj, "field", default)` | Get field with default | `getattr(result, "count", 0)` |
| `len(collection)` | Get length | `len(pods) > 0` |
| `str(value)` | Convert to string | `str(error_code)` |
| `int(value)` | Convert to integer | `int(count_str)` |
| `type(value)` | Get type name | `type(val) == "string"` |

### Common Patterns

**1. Safe Field Access (ALWAYS USE THIS):**
```yaml
if: "hasattr(result, 'error') and result.error != None"
```

**2. Check Rollback Recommended:**
```yaml
if: "hasattr(rca_result, 'rollback_recommended') and rca_result.rollback_recommended == True"
```

**3. Error Rate Threshold:**
```yaml
if: "hasattr(metrics, 'error_rate') and metrics.error_rate > 0.05"
```

**4. Array Not Empty:**
```yaml
if: "hasattr(scan_result, 'vulnerabilities') and len(scan_result.vulnerabilities) > 0"
```

---

## Complete Examples

### Example 1: Incident Root Cause Analysis

A parallel investigation workflow that gathers data from multiple sources and synthesizes findings.

```yaml
id: incident-rca
name: "Incident Root Cause Analysis"
version: "1.0.0"
description: "Parallel investigation across K8s, logs, and metrics"

inputSchema:
  type: object
  properties:
    service_name:
      type: string
      description: "Service experiencing the incident"
    namespace:
      type: string
      default: "default"
  required:
    - service_name

outputSchema:
  type: object
  properties:
    root_cause:
      type: object
    recommended_actions:
      type: array
    rollback_recommended:
      type: boolean

start: parallel-investigation

states:
  # Step 1: Parallel data gathering
  - id: parallel-investigation
    name: "Parallel Data Collection"
    type: parallel
    branches:
      - id: k8s-branch
        start: check-k8s
        states:
          - id: check-k8s
            type: agent
            agent: k8s-investigator
            input:
              service_name: "$.input.service_name"
              namespace: "$.input.namespace"
            output:
              k8s_data: "$.result"
            timeout: 2m
            end: true

      - id: logs-branch
        start: check-logs
        states:
          - id: check-logs
            type: agent
            agent: log-analyzer
            input:
              service_name: "$.input.service_name"
              time_range_minutes: 60
            output:
              log_data: "$.result"
            timeout: 2m
            end: true

      - id: metrics-branch
        start: check-metrics
        states:
          - id: check-metrics
            type: agent
            agent: metrics-collector
            input:
              service_name: "$.input.service_name"
            output:
              metrics_data: "$.result"
            timeout: 2m
            end: true

    output:
      investigation_results: "$.branches"
    transition: synthesize-findings

  # Step 2: Synthesize all findings
  - id: synthesize-findings
    name: "Synthesize Root Cause"
    type: agent
    agent: rca-synthesizer
    input:
      service_name: "$.input.service_name"
      k8s_data: "$.investigation_results.k8s_branch.k8s_data"
      log_data: "$.investigation_results.logs_branch.log_data"
      metrics_data: "$.investigation_results.metrics_branch.metrics_data"
    output:
      rca_result: "$.result"
    timeout: 2m
    transition: check-rollback-needed

  # Step 3: Conditional routing based on analysis
  - id: check-rollback-needed
    name: "Evaluate Rollback Need"
    type: switch
    dataPath: "$.rca_result"
    conditions:
      - if: "hasattr(rca_result, 'rollback_recommended') and rca_result.rollback_recommended == True"
        next: request-rollback-approval
    defaultNext: generate-output

  # Step 4: Human approval for rollback
  - id: request-rollback-approval
    name: "Request Rollback Approval"
    type: human_approval
    approval_title: "Rollback Approval Required"
    message: |
      Root Cause Analysis Complete for {{input.service_name}}
      
      **Root Cause**: {{rca_result.root_cause.category}}
      **Confidence**: {{rca_result.root_cause.confidence}}
      
      {{rca_result.root_cause.description}}
      
      Do you approve the rollback?
    approvers:
      - oncall-sre
    timeout: 30m
    transition: generate-output

  # Step 5: Generate final report
  - id: generate-output
    name: "Generate Final Report"
    type: transform
    expression: |
      report = {
        "service_name": getattr(rca_result, "service_name", input.service_name),
        "root_cause": getattr(rca_result, "root_cause", {}),
        "recommended_actions": getattr(rca_result, "recommended_actions", []),
        "rollback_recommended": getattr(rca_result, "rollback_recommended", False),
        "timeline": getattr(rca_result, "timeline", []),
        "analysis_completed_at": getattr(rca_result, "analysis_completed_at", "")
      }
      report
    output:
      final_report: "$.result"
    end: true
```

**Execution Flow Diagram:**

```
                        ┌──────────────────┐
                        │      START       │
                        │ (workflow input) │
                        └────────┬─────────┘
                                 │
                                 ▼
              ┌──────────────────────────────────────┐
              │       parallel-investigation          │
              └──────────────────┬───────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
         ▼                       ▼                       ▼
   ┌───────────┐          ┌───────────┐          ┌───────────┐
   │check-k8s  │          │check-logs │          │check-metr.│
   │(agent)    │          │(agent)    │          │(agent)    │
   └─────┬─────┘          └─────┬─────┘          └─────┬─────┘
         │                      │                      │
         └──────────────────────┼──────────────────────┘
                                │
                          (wait for all)
                                │
                                ▼
                   ┌─────────────────────┐
                   │ synthesize-findings │
                   │    (rca-synth.)     │
                   └──────────┬──────────┘
                              │
                              ▼
                   ┌─────────────────────┐
                   │check-rollback-needed│
                   │      (switch)       │
                   └──────────┬──────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
     rollback_recommended              default (no rollback)
              │                               │
              ▼                               │
   ┌─────────────────────┐                    │
   │request-rollback-appr│                    │
   │  (human_approval)   │                    │
   └──────────┬──────────┘                    │
              │                               │
              └───────────────┬───────────────┘
                              │
                              ▼
                   ┌─────────────────────┐
                   │   generate-output   │
                   │    (transform)      │
                   └──────────┬──────────┘
                              │
                              ▼
                        ┌──────────┐
                        │   END    │
                        └──────────┘
```

### Example 2: Multi-Service Health Check (Foreach)

```yaml
id: multi-service-health
name: "Multi-Service Health Check"
version: "1.0.0"
description: "Check health of multiple services with concurrent execution"

inputSchema:
  type: object
  properties:
    services:
      type: array
      items:
        type: object
        properties:
          name: { type: string }
          namespace: { type: string }
  required:
    - services

start: check-all-services

states:
  - id: check-all-services
    name: "Check All Services"
    type: foreach
    itemsPath: "$.input.services"
    itemName: "service"
    maxConcurrency: 5
    iterator:
      start: check-one
      states:
        - id: check-one
          type: agent
          agent: health-checker
          input:
            service_name: "$.service.name"
            namespace: "$.service.namespace"
          output:
            health: "$.result"
          timeout: 1m
          end: true
    output:
      all_results: "$.iterations"
    transition: summarize

  - id: summarize
    name: "Summarize Health"
    type: transform
    expression: |
      healthy = 0
      unhealthy = 0
      results = []
      
      for item in all_results:
          if hasattr(item, "health") and item.health.status == "healthy":
              healthy = healthy + 1
          else:
              unhealthy = unhealthy + 1
          results.append({
              "service": item.service.name,
              "status": getattr(item.health, "status", "unknown")
          })
      
      {
          "total_services": len(all_results),
          "healthy_count": healthy,
          "unhealthy_count": unhealthy,
          "details": results
      }
    output:
      summary: "$.result"
    end: true

### Example 3: Slack-Based Human Approval

Analyzes a security threat and requires human confirmation before taking a blocking action.

```yaml
id: slack-approval-escalation
name: "Security Alert: Blocking Action"
version: "1.0.0"
description: "IP analysis with human-in-the-loop blocking"

start: analyze-ip

states:
  - id: analyze-ip
    type: agent
    agent: security-analyzer
    input:
      ip: "$.input.ip"
    output:
      analysis: "$.result"
    transition: check-risk

  - id: check-risk
    type: switch
    dataPath: "$.analysis"
    conditions:
      - if: "risk_score > 0.8"
        next: request-approval
    defaultNext: log-incident

  - id: request-approval
    type: human_approval
    message: |
      A high-risk IP ({{input.ip}}) was detected with risk score {{analysis.risk_score}}. 
      Reason: {{analysis.reason}}
      
      Should we block this IP in the production firewall?
    approvers:
      - security-admin
    output:
      approval_decision: "$.result"
    transition: handle-approval

  - id: handle-approval
    type: switch
    dataPath: "$.approval_decision"
    conditions:
      - if: "approved == True"
        next: block-ip
    defaultNext: log-incident

  - id: block-ip
    type: agent
    agent: firewall-admin
    input:
      ip: "$.input.ip"
      action: "block"
    output:
      block_result: "$.result"
    transition: notify-team

  - id: log-incident
    type: agent
    agent: security-logger
    input:
      ip: "$.input.ip"
      analysis: "$.analysis"
      blocked: false
    output:
      log_result: "$.result"
    end: true

  - id: notify-team
    type: agent
    agent: notifier
    input:
      message: "IP {{input.ip}} has been blocked successfully."
    end: true
```

### Example 4: CloudWatch to JIRA Escalation

Ingests a CloudWatch alarm, gathers log context, and manages JIRA tickets.

```yaml
id: cloudwatch-jira-escalation
name: "Infrastructure Alert: JIRA Escalation"
version: "1.0.0"
description: "CloudWatch alarm analysis and ticket deduplication"

start: analyze-logs

states:
  - id: analyze-logs
    type: agent
    agent: aws-log-analyzer
    input:
      service_name: "$.input.metric_data.service"
      region: "$.input.metric_data.region"
      time_range: 15
    output:
      log_analysis: "$.result"
    transition: check-duplicate

  - id: check-duplicate
    type: agent
    agent: jira-analyst
    input:
      query: "service = {{input.metric_data.service}} AND status != Closed"
    output:
      duplicate_found: "$.result.has_duplicate"
      existing_ticket_id: "$.result.ticket_id"
    transition: decide-escalation

  - id: decide-escalation
    type: switch
    dataPath: "$"
    conditions:
      - if: "duplicate_found == False"
        next: create-ticket
    defaultNext: comment-on-ticket

  - id: create-ticket
    type: agent
    agent: jira-admin
    input:
      summary: "Alarm: {{input.alarm_name}}"
      description: "{{input.alarm_description}}\n\nAnalysis: {{log_analysis.summary}}"
    output:
      ticket: "$.result"
    transition: notify-team

  - id: comment-on-ticket
    type: agent
    agent: jira-admin
    input:
      ticket_id: "$.existing_ticket_id"
      comment: "Alarm triggered again. New analysis: {{log_analysis.summary}}"
    transition: notify-team

  - id: notify-team
    type: agent
    agent: notifier
    input:
      channel: "#alerts"
      text: "Processed alarm {{input.alarm_name}}. Ticket: {{ticket.key if hasattr(state, 'ticket') else existing_ticket_id}}"
    end: true
```

---


## Best Practices

### 1. Always Use Safe Field Access

```yaml
# BAD - will fail if field missing
if: "result.rollback_recommended == True"

# GOOD - checks existence first
if: "hasattr(result, 'rollback_recommended') and result.rollback_recommended == True"
```

### 2. Set Timeouts on Agent Steps

```yaml
- id: long-running-analysis
  type: agent
  agent: deep-analyzer
  timeout: 5m                    # Prevent hanging workflows
```

### 3. Use Descriptive State IDs

```yaml
# BAD
- id: step1
- id: step2

# GOOD  
- id: gather-pod-metrics
- id: analyze-error-patterns
```

### 4. Add Input/Output Schemas

```yaml
inputSchema:
  type: object
  properties:
    service_name:
      type: string
      description: "Name of the service to analyze"
  required:
    - service_name
```

### 5. Use Inject for Configuration

```yaml
- id: set-thresholds
  type: inject
  data:
    thresholds:
      error_rate: 0.05
      latency_p99: 500
  output:
    config: "$.data"
  transition: analyze
```

### 6. Handle Default Cases in Switch

```yaml
- id: route
  type: switch
  dataPath: "$.severity"
  conditions:
    - if: "severity == 'critical'"
      next: escalate
  defaultNext: normal-path       # ALWAYS have a fallback
```

---

## Troubleshooting

### Common Errors

**1. "attrdict has no .field field or method"**

The agent didn't output the expected field. Use `hasattr()`:
```yaml
if: "hasattr(result, 'field') and result.field == 'value'"
```

**2. "undefined: variable_name"**

The variable isn't in context. Check:
- Previous step's `output` mapping
- JSONPath is correct
- Step actually executed

**3. "transform expression failed"**

Check Starlark syntax:
- Use `=` for assignment, `==` for comparison
- Strings need quotes: `"value"` not `value`
- Dict syntax: `{"key": value}` not `{key: value}`

**4. Workflow stuck in "pending"**

- Check NATS is running: `stn status`
- Check workflow consumer started (look for "Workflow consumer started" in logs)
- Restart Station: `stn serve`

### Debugging Tips

1. **Check run context:**
   ```bash
   sqlite3 ~/.config/station/station.db \
     "SELECT context FROM workflow_runs WHERE run_id = 'YOUR_RUN_ID'" \
     | python3 -m json.tool
   ```

2. **View step outputs:**
   ```bash
   stn workflow runs YOUR_RUN_ID --steps
   ```

3. **Clear cached workflow (after edits):**
   ```bash
   sqlite3 ~/.config/station/station.db \
     "DELETE FROM workflows WHERE json_extract(definition, '$.id') = 'your-workflow-id'"
   stn sync
   ```

4. **Enable debug logging:**
   ```bash
   STN_LOG_LEVEL=debug stn serve
   ```

---

## Next Steps

- Read the [MCP Workflow DSL Resource](station://docs/workflow-dsl) for API details
- Check [Starlark Expressions Guide](./starlark-expressions.md) for advanced expressions
- See [Workflow Engine Architecture](../features/workflow-engine-v1.md) for internals

