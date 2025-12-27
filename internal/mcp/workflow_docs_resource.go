package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// workflowDSLDocumentation contains the comprehensive documentation for the Station Workflow DSL
const workflowDSLDocumentation = `# Station Workflow DSL Reference

Station uses a subset of the Serverless Workflow specification for defining multi-step automated workflows.

> **TIP**: For a complete authoring guide with examples and diagrams, see the Workflow Authoring Guide at docs/station/workflow-authoring-guide.md

---

## ⚠️ CRITICAL: Input Variable Context Paths

**Input variables are FLATTENED to the root context.** This is the #1 source of workflow errors.

| Correct ✅ | Wrong ❌ |
|-----------|---------|
| ` + "`$.service_name`" + ` | ` + "`$.input.service_name`" + ` |
| ` + "`$.services`" + ` | ` + "`$.input.services`" + ` |
| ` + "`$.environment`" + ` | ` + "`$.workflow.input.environment`" + ` |

**Example:** If workflow is started with input ` + "`" + `{"services": ["api", "web"], "environment": "prod"}` + "`" + `:

` + "```yaml" + `
# CORRECT - input flattened to root
itemsPath: "$.services"           # ✅ Works
dataPath: "$.environment"         # ✅ Works

# WRONG - $.input does not exist at root level
itemsPath: "$.input.services"     # ❌ "items not found at itemsPath"
dataPath: "$.input.environment"   # ❌ Will fail
` + "```" + `

**Why?** The workflow engine builds context like this:
` + "```json" + `
{
  "services": ["api", "web"],     // ← Input FLATTENED here (USE THIS)
  "environment": "prod",          // ← Input FLATTENED here (USE THIS)
  "workflow": { "input": {...} }, // ← Also stored here (internal use)
  "steps": { ... }                // ← Step outputs stored here
}
` + "```" + `

---

## Prefer YAML for Workflow Definitions

**YAML is strongly recommended over JSON** for workflow definitions:
- More readable for complex workflows with embedded prompts
- Supports multi-line strings naturally (for transform expressions)
- Comments allowed (` + "`#`" + ` prefix)

**File naming:** ` + "`<workflow-id>.workflow.yaml`" + ` (preferred) or ` + "`.workflow.json`" + `
**Location:** ` + "`~/.config/station/environments/<env>/workflows/`" + `

---

## Workflow Definition Structure

A workflow definition is a YAML or JSON document with these top-level fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique identifier for the workflow |
| name | string | Yes | Human-readable name |
| version | string | No | Semantic version (e.g., "1.0") |
| description | string | No | Brief description of what the workflow does |
| inputSchema | object | No | JSON Schema for validating workflow input |
| outputSchema | object | No | JSON Schema for validating workflow output |
| start | string | Yes | ID of the first state to execute |
| states | array | Yes | Array of state definitions |

### Example Workflow Definition

` + "```yaml" + `
id: incident-rca
name: "Incident Root Cause Analysis"
version: "1.0"
description: "Parallel investigation and synthesis with approval gate"

inputSchema:
  type: object
  properties:
    service_name:
      type: string
  required:
    - service_name

start: parallel-investigation

states:
  # Parallel data gathering
  - id: parallel-investigation
    type: parallel
    branches:
      - id: check-k8s
        start: k8s-step
        states:
          - id: k8s-step
            type: agent
            agent: k8s-investigator
            input:
              service: "$.service_name"    # Input flattened to root!
            output:
              k8s_data: "$.result"
            end: true
      - id: check-logs
        start: logs-step
        states:
          - id: logs-step
            type: agent
            agent: log-analyzer
            input:
              service: "$.service_name"    # Input flattened to root!
            output:
              log_data: "$.result"
            end: true
    output:
      investigation: "$.branches"
    transition: synthesize

  # Synthesize findings
  - id: synthesize
    type: agent
    agent: rca-synthesizer
    input:
      k8s_data: "$.investigation.check_k8s.k8s_data"
      log_data: "$.investigation.check_logs.log_data"
    output:
      rca_result: "$.result"
    transition: check-rollback

  # Conditional routing (USE hasattr for safe access!)
  - id: check-rollback
    type: switch
    dataPath: "$.rca_result"
    conditions:
      - if: "hasattr(rca_result, 'rollback_recommended') and rca_result.rollback_recommended == True"
        next: request-approval
    defaultNext: generate-report

  # Human approval gate
  - id: request-approval
    type: human_approval
    message: "Approve rollback for {{input.service_name}}?"
    timeout: 30m
    transition: generate-report

  # Transform to build final output
  - id: generate-report
    type: transform
    expression: |
      {
        "service": getattr(input, "service_name", "unknown"),
        "root_cause": getattr(rca_result, "root_cause", {}),
        "actions": getattr(rca_result, "recommended_actions", []),
        "rollback_needed": getattr(rca_result, "rollback_recommended", False)
      }
    output:
      final_report: "$.result"
    end: true
` + "```" + `

---

## State Types

### 1. Agent State (type: "agent")

Executes a Station agent by name. This is the primary state type for AI-powered steps.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "agent" |
| agent | string | Yes | Agent name (resolved from environment) |
| input | object | No | Map workflow context to agent input |
| output | object | No | Map agent output to workflow context |
| timeout | string | No | Agent execution timeout (e.g., "2m", "5m") |
| transition | string | No* | Next state to execute |
| end | boolean | No* | Mark as terminal state |

*One of transition or end: true is required.

` + "```yaml" + `
- id: analyze-logs
  type: agent
  agent: log-analyzer                 # Agent name (must exist)
  input:
    service_name: "$.service_name"    # Input is at root level!
    time_range: 60                    # Static value
  output:
    log_analysis: "$.result"          # Store full result
    severity: "$.result.severity"     # Store specific field
  timeout: 2m
  transition: check-severity
` + "```" + `

### 1b. Operation State (type: "operation") - Legacy

Operation state is an older syntax that wraps agent execution. Prefer "agent" type for new workflows.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "operation" |
| input | object | No | Map input from workflow context to step input |
| output | object | No | Map step output back to workflow context |
| transition | string | No* | Next state to execute |
| timeout | string | No | Step timeout (e.g., "5m", "30s") |
| retry | object | No | Retry policy configuration |

#### Retry Policy

` + "```yaml" + `
retry:
  max_attempts: 3
  backoff: "exponential"   # or "fixed"
  retry_on:
    - "timeout"
    - "agent_error"
` + "```" + `

---

### 2. Switch State (type: "switch")

Conditional branching based on context data using Starlark expressions.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "switch" |
| dataPath | string | Yes | JSONPath to the value to evaluate |
| conditions | array | Yes | Array of condition objects |
| defaultNext | string | No | Fallback state if no conditions match |

#### Condition Object

> **⚠️ CRITICAL: Use ` + "`if`" + ` and ` + "`next`" + ` - NOT ` + "`condition`" + `/` + "`transition`" + `!**
> 
> Using wrong field names will cause: ` + "`condition evaluation failed: parse error: got end of file`" + `

| Field | Type | Description |
|-------|------|-------------|
| if | string | Starlark expression - the dataPath variable is available by name |
| next | string | State to transition to if condition is true |

` + "```yaml" + `
# ✅ CORRECT
conditions:
  - if: "input.logs != None"
    next: analyze-logs

# ❌ WRONG (will fail silently - fields not recognized)
conditions:
  - condition: "$.logs != null"    # WRONG field name!
    transition: analyze-logs       # WRONG field name!
` + "```" + `

#### Starlark Built-in Functions

**IMPORTANT**: Always use hasattr() to check if a field exists before accessing it. Agent outputs may be incomplete.

| Function | Description | Example |
|----------|-------------|---------|
| hasattr(obj, "field") | Check if field exists | hasattr(result, "error") |
| getattr(obj, "field", default) | Get field with default value | getattr(result, "count", 0) |
| len(collection) | Get length of array/dict | len(pods) > 0 |
| type(value) | Get type name | type(val) == "string" |

#### Supported Starlark Operators

- Comparison: ==, !=, <, >, <=, >=
- Logical: and, or, not
- String: in, startswith(), endswith()

` + "```yaml" + `
# Example: Safe routing with hasattr
- id: check_rollback
  type: switch
  dataPath: "$.rca_result"
  conditions:
    # ALWAYS use hasattr() for safe field access
    - if: "hasattr(rca_result, 'rollback_recommended') and rca_result.rollback_recommended == True"
      next: request_approval
    - if: "hasattr(rca_result, 'severity') and rca_result.severity == 'critical'"
      next: escalate
  defaultNext: normal_flow

# Example: Numeric comparison
- id: check_error_rate
  type: switch
  dataPath: "$.metrics.error_rate"
  conditions:
    - if: "error_rate > 0.5"
      next: critical_path
    - if: "error_rate > 0.1"
      next: warning_path
  defaultNext: normal_path
` + "```" + `

---

### 3. Inject State (type: "inject")

Insert static data into the workflow context.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "inject" |
| data | object | Yes | Static data to inject |
| resultPath | string | No | JSONPath where to store the data |
| transition | string | No* | Next state |

` + "```yaml" + `
- id: set_defaults
  type: inject
  data:
    thresholds:
      error_rate: 0.05
      latency_p99: 500
    regions:
      - us-east-1
      - us-west-2
  resultPath: "$.config"
  transition: check_services
` + "```" + `

---

### 4. Parallel State (type: "parallel")

Execute multiple branches concurrently.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "parallel" |
| branches | array | Yes | Array of branch definitions |
| join | object | No | How to join branches (default: wait for all) |
| transition | string | No* | Next state after all branches complete |

#### Branch Definition

| Field | Type | Description |
|-------|------|-------------|
| name | string | Branch identifier |
| states | array | Array of states to execute in this branch |

` + "```yaml" + `
- id: gather_diagnostics
  type: parallel
  branches:
    - name: pods
      states:
        - id: check_pods
          type: operation
          input:
            task: "List unhealthy pods"
          end: true
    - name: logs
      states:
        - id: fetch_logs
          type: operation
          input:
            task: "Get recent error logs"
          end: true
    - name: metrics
      states:
        - id: query_metrics
          type: operation
          input:
            task: "Query Prometheus for anomalies"
          end: true
  join:
    mode: "all"   # Wait for all branches (default)
  transition: analyze_results
` + "```" + `

---

### 5. Transform State (type: "transform")

Reshape data using Starlark expressions. All workflow context variables are available.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "transform" |
| expression | string | Yes | Starlark expression (multi-line supported) |
| output | object | No | Map result to context variables |
| transition | string | No* | Next state |

#### Writing Transform Expressions

- All workflow context variables are available as globals (input, previous step outputs, etc.)
- Use getattr(obj, "field", default) for safe field access
- Use hasattr(obj, "field") to check existence
- Comments with # are supported
- The last expression becomes the output

` + "```yaml" + `
- id: build_report
  type: transform
  expression: |
    # Build final report from analysis results
    # Safe access with getattr for potentially missing fields
    
    report = {
      "service_name": getattr(input, "service_name", "unknown"),
      "root_cause": getattr(rca_result, "root_cause", {}),
      "recommended_actions": getattr(rca_result, "recommended_actions", []),
      "rollback_recommended": getattr(rca_result, "rollback_recommended", False),
      "timeline": getattr(rca_result, "timeline", [])
    }
    
    # Last expression is the output
    report
  output:
    final_report: "$.result"
  end: true
` + "```" + `

#### Transform vs Inject

- **inject**: Insert static or JSONPath-referenced data
- **transform**: Compute new data using Starlark logic

---

### 6. Foreach State (type: "foreach")

Iterate over an array, executing a sub-workflow for each item.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "foreach" |
| itemsPath | string | Yes | JSONPath to the array to iterate |
| itemName | string | No | Variable name for current item (default: "item") |
| maxConcurrency | int | No | Max parallel iterations (default: 1) |
| iterator | object | Yes | Sub-workflow to run for each item |
| transition | string | No* | Next state after iteration completes |

#### Iterator Definition

| Field | Type | Description |
|-------|------|-------------|
| start | string | First state in the sub-workflow |
| states | array | States for the iteration sub-workflow |

` + "```yaml" + `
- id: check_each_service
  type: foreach
  itemsPath: "$.services"
  itemName: "service"
  maxConcurrency: 5
  iterator:
    start: health_check
    states:
      - id: health_check
        type: operation
        input:
          task: "Check health of {{ $.service.name }}"
        end: true
  transition: summarize
` + "```" + `

---

## Context and Data Flow

### Workflow Context Structure

**Remember: Input is FLATTENED to root level!**

` + "```json" + `
{
  "service_name": "api-gateway",     // ← Input flattened here (USE $.service_name)
  "environment": "production",       // ← Input flattened here (USE $.environment)
  "workflow": {
    "input": { ... }                 // Internal copy (don't use in paths)
  },
  "steps": {
    "step_id": { "result": ... }     // Step outputs stored here
  },
  "run_id": "..."                    // Unique run identifier
}
` + "```" + `

### JSONPath Expressions

Use JSONPath to reference values:

| Expression | Description |
|------------|-------------|
| $.field | **Workflow input** (flattened to root) - USE THIS |
| $.steps.step_id.field | Output from a specific step |
| $.result | Output from the previous step |
| $.item | Current item in foreach loops |

### Template Strings

Use {{ }} for inline templates in string values:

` + "```yaml" + `
input:
  task: "Check pods in {{ $.namespace }} for service {{ $.service }}"
` + "```" + `

---

## Validation Warnings

The workflow validator provides helpful warnings for common issues:

| Warning Code | Meaning |
|--------------|---------|
| MISSING_INPUT_MAPPING | Step has no input mapping - data won't flow in |
| MISSING_EXPORT_MAPPING | Step has no output mapping - results won't be saved |
| MISSING_RETRY_POLICY | No retry policy - step won't retry on failure |
| MISSING_TIMEOUT | No timeout - step could hang indefinitely |
| UNREACHABLE_STATE | State has no incoming transitions |
| DEAD_END_STATE | Non-terminal state with no outgoing transitions |

---

## Best Practices

1. **Always define input/output mappings** - Ensures data flows correctly between steps
2. **Use descriptive state IDs** - Makes debugging easier (e.g., "check_pods" not "step1")
3. **Add retry policies** - Agents can have transient failures
4. **Set timeouts** - Prevent hung workflows
5. **Use parallel states** - Speed up independent operations
6. **Test with validate endpoint** - Check definition before creating workflow

---

## API Reference

### Create Workflow

` + "```" + `
Tool: workflow_create
Parameters:
  - workflow_id: Unique identifier (e.g., "incident-triage")
  - name: Human-readable name
  - description: Optional description
  - definition: JSON string of the workflow definition
` + "```" + `

### Start Workflow Run

` + "```" + `
Tool: workflow_start_run
Parameters:
  - workflow_id: Which workflow to run
  - input: Optional JSON input data for the workflow
  - version: Optional specific version (default: latest)
` + "```" + `

### Validate Workflow

` + "```" + `
Tool: workflow_validate
Parameters:
  - definition: JSON string to validate
` + "```" + `

---

## Common Patterns

### Safe Field Access in Switch Conditions

Agent outputs may not include all expected fields. Always use hasattr():

` + "```yaml" + `
# BAD - will fail if field missing
- if: "result.rollback_recommended == True"

# GOOD - checks existence first  
- if: "hasattr(result, 'rollback_recommended') and result.rollback_recommended == True"
` + "```" + `

### Building Reports with Transform

Use transform to assemble final output from multiple sources:

` + "```yaml" + `
- id: final-report
  type: transform
  expression: |
    {
      "service": getattr(input, "service_name", "unknown"),
      "findings": getattr(analysis, "findings", []),
      "severity": getattr(analysis, "severity", "unknown"),
      "timestamp": "2025-01-01T00:00:00Z"
    }
  output:
    report: "$.result"
  end: true
` + "```" + `

---

## Resources

- **Workflow Authoring Guide**: docs/station/workflow-authoring-guide.md (comprehensive guide with examples)
- **Starlark Expressions**: docs/developers/starlark-expressions.md
- **List workflows**: station://workflows
- **Get workflow details**: station://workflows/{workflow_id}
- **List workflow runs**: station://workflow-runs

## API Tools

| Tool | Description |
|------|-------------|
| create_workflow | Create a new workflow definition |
| update_workflow | Update existing workflow (creates new version) |
| validate_workflow | Validate definition without saving |
| start_workflow_run | Start a new workflow run |
| get_workflow_run | Get run status and details |

---

## File-Based vs MCP-Created Workflows

### File-Based Workflows (Recommended for GitOps)

Workflows defined in files are **automatically synced** when you run ` + "`stn sync`" + `:

` + "```" + `
~/.config/station/environments/<env>/workflows/
├── incident-triage.workflow.yaml     # ✅ Synced on 'stn sync'
├── deploy-pipeline.workflow.yaml     # ✅ Synced on 'stn sync'
└── security-scan.workflow.yaml       # ✅ Synced on 'stn sync'
` + "```" + `

**Benefits:**
- Version controlled in Git
- Reviewable via PRs
- Portable between environments
- Single source of truth

### MCP-Created Workflows

Workflows created via ` + "`create_workflow`" + ` tool are stored in the **database only**.

**Important:** MCP-created workflows are NOT automatically exported to files. For GitOps workflows, 
create your workflow as a ` + "`.workflow.yaml`" + ` file and run ` + "`stn sync`" + `.

**To export a DB workflow to file:** Use the workflow definition from ` + "`get_workflow`" + ` and save 
it manually to ` + "`~/.config/station/environments/<env>/workflows/<id>.workflow.yaml`" + `.
`

// handleWorkflowDSLResource returns the comprehensive Workflow DSL documentation
func (s *Server) handleWorkflowDSLResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     workflowDSLDocumentation,
		},
	}, nil
}

// handleWorkflowsListResource returns the list of workflow definitions
func (s *Server) handleWorkflowsListResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	workflows, err := s.workflowService.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}

	// Build a summary optimized for LLM context
	var summary string
	summary = "# Available Workflows\n\n"

	if len(workflows) == 0 {
		summary += "No workflows defined yet. Use the `workflow_create` tool to create one.\n\n"
		summary += "See the Workflow DSL reference at `station://docs/workflow-dsl` for schema documentation.\n"
	} else {
		summary += "| Workflow ID | Name | Version | Status |\n"
		summary += "|-------------|------|---------|--------|\n"
		for _, w := range workflows {
			summary += "| " + w.WorkflowID + " | " + w.Name + " | v" +
				string(rune('0'+w.Version)) + " | " + w.Status + " |\n"
		}
		summary += "\n\nUse `workflow_get` with a workflow_id for full details.\n"
		summary += "See `station://docs/workflow-dsl` for DSL reference.\n"
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     summary,
		},
	}, nil
}
