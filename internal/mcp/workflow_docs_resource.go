package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// workflowDSLDocumentation contains the comprehensive documentation for the Station Workflow DSL
const workflowDSLDocumentation = `# Station Workflow DSL Reference

Station uses a subset of the Serverless Workflow specification for defining multi-step automated workflows.

## Workflow Definition Structure

A workflow definition is a YAML or JSON document with these top-level fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique identifier for the workflow |
| name | string | Yes | Human-readable name |
| version | string | No | Semantic version (e.g., "1.0") |
| description | string | No | Brief description of what the workflow does |
| start | string | Yes | ID of the first state to execute |
| states | array | Yes | Array of state definitions |

### Example Workflow Definition

` + "```yaml" + `
id: incident-triage
name: "Incident Triage Workflow"
version: "1.0"
description: "Automated incident diagnosis with approval gate"
start: gather_info

states:
  - id: gather_info
    type: operation
    input:
      task: "Check Kubernetes pods in namespace {{ $.namespace }}"
    output:
      pod_status: "$.result"
    transition: analyze

  - id: analyze
    type: switch
    dataPath: "$.pod_status.severity"
    conditions:
      - if: "val == 'critical'"
        next: escalate
      - if: "val == 'low'"
        next: auto_resolve
    defaultNext: investigate

  - id: investigate
    type: operation
    input:
      task: "Deep investigation of {{ $.service }}"
    transition: request_approval

  - id: request_approval
    type: operation  # Will be human.approval in future
    input:
      message: "Please approve fix deployment"
    transition: apply_fix

  - id: apply_fix
    type: operation
    input:
      task: "Apply remediation"
    end: true

  - id: escalate
    type: operation
    input:
      task: "Page on-call engineer"
    end: true

  - id: auto_resolve
    type: operation
    input:
      task: "Log low-severity incident"
    end: true
` + "```" + `

---

## State Types

### 1. Operation State (type: "operation")

Executes an agent or action. This is the most common state type.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "operation" |
| input | object | No | Map input from workflow context to step input |
| output | object | No | Map step output back to workflow context |
| transition | string | No* | Next state to execute |
| next | string | No* | Alternative to 'transition' |
| end | boolean | No* | Mark as terminal state |
| timeout | string | No | Step timeout (e.g., "5m", "30s") |
| retry | object | No | Retry policy configuration |

*One of transition, next, or end: true is required.

#### Input/Output Mapping

Use JSONPath expressions to map data between workflow context and step inputs:

` + "```yaml" + `
- id: check_pods
  type: operation
  input:
    namespace: "$.input.namespace"      # From workflow input
    service: "$.context.service_name"   # From context set by previous step
    task: "Check pods in {{ $.input.namespace }}"  # Template string
  output:
    pod_count: "$.result.count"         # Store result.count in context.pod_count
    status: "$.result.status"
  transition: next_step
` + "```" + `

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

Conditional branching based on context data.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique state identifier |
| type | string | Yes | Must be "switch" |
| dataPath | string | Yes | JSONPath to the value to evaluate |
| conditions | array | Yes | Array of condition objects |
| defaultNext | string | No | Fallback state if no conditions match |

#### Condition Object

| Field | Type | Description |
|-------|------|-------------|
| if | string | Starlark expression (use 'val' for the dataPath value) |
| next | string | State to transition to if condition is true |

#### Supported Starlark Operators

- Comparison: ==, !=, <, >, <=, >=
- Logical: and, or, not
- String: in, startswith(), endswith(), contains()
- Type checks: type(val) == "string"

` + "```yaml" + `
- id: check_severity
  type: switch
  dataPath: "$.error_rate"
  conditions:
    - if: "val > 0.5"
      next: critical_path
    - if: "val > 0.1"
      next: warning_path
    - if: "val <= 0.1"
      next: normal_path
  defaultNext: unknown_path
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

### 5. Foreach State (type: "foreach")

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

` + "```json" + `
{
  "input": { ... },      // Original workflow input
  "context": { ... },    // Accumulated data from step outputs
  "current_step": "...", // Current step being executed
  "run_id": "..."        // Unique run identifier
}
` + "```" + `

### JSONPath Expressions

Use JSONPath to reference values:

| Expression | Description |
|------------|-------------|
| $.input.field | Access workflow input |
| $.context.field | Access accumulated context |
| $.field | Shorthand for $.context.field |
| $.result | Output from the previous step |

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

## Resources

- Read existing workflows: station://workflows
- Get workflow details: station://workflows/{workflow_id}
- List workflow runs: station://workflow-runs
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
