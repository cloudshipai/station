# Station MCP Tools Reference

Complete reference for Station's 41 MCP tools - the programmatic interface for managing agents, environments, and execution workflows through AI assistants like Claude, Cursor, and OpenCode.

## Overview

Station exposes its full functionality through MCP tools, making it **MCP-first** rather than CLI-first. When you use Station through Claude Desktop, Cursor, or OpenCode, you're using these tools.

**Why MCP Tools?**
- **Structured Responses**: JSON responses instead of CLI text parsing
- **Better Error Handling**: Programmatic error codes and messages
- **AI-Native**: Designed for AI assistant interaction
- **Full Featured**: Complete access to all Station capabilities

## Tool Categories

- [Agent Management](#agent-management) - Create, update, list agents
- [Agent Execution](#agent-execution) - Run agents and monitor execution
- [Evaluation & Testing](#evaluation--testing) - Test and benchmark agents
- [Reports & Analytics](#reports--analytics) - Team performance metrics
- [Environment Management](#environment-management) - Manage environments
- [MCP Server Management](#mcp-server-management) - Configure MCP servers
- [Tool Discovery](#tool-discovery) - Discover available tools
- [Scheduling](#scheduling) - Schedule agent runs
- [Bundles](#bundles) - Package and install agent bundles
- [Faker System](#faker-system) - Generate simulated environments

---

## Agent Management

### `opencode-station_create_agent`

Create a new agent with specified configuration.

**Parameters:**
```typescript
{
  name: string,                    // Agent name
  description: string,             // Agent description
  prompt: string,                  // System prompt (dotprompt format)
  environment_id: string,          // Environment ID
  enabled?: boolean,               // Enable agent (default: true)
  max_steps?: number,              // Max execution steps (default: 5)
  tool_names?: string[],           // List of tool names to assign
  input_schema?: string,           // JSON schema for custom input variables
  output_schema?: string,          // JSON schema for output format
  output_schema_preset?: string,   // Preset schema (e.g., 'finops-inventory')
  app?: string,                    // CloudShip app classification ('finops', 'security', 'deployments')
  app_type?: string                // CloudShip app_type ('inventory', 'investigations', 'opportunities', 'projections', 'events')
}
```

**Example:**
```typescript
opencode-station_create_agent({
  name: "AWS Cost Analyzer",
  description: "Analyzes AWS cost trends and identifies optimization opportunities",
  prompt: `---
metadata:
  name: "AWS Cost Analyzer"
  description: "Cost analysis and optimization"
model: gpt-4o-mini
max_steps: 5
tools:
  - "__get_cost_and_usage"
  - "__list_cost_allocation_tags"
---

{{role "system"}}
You are a FinOps analyst specializing in AWS cost optimization.

{{role "user"}}
{{userInput}}`,
  environment_id: "1",
  enabled: true
})
```

**Returns:**
```json
{
  "id": "42",
  "name": "AWS Cost Analyzer",
  "environment_id": "1",
  "created_at": "2025-11-19T10:30:00Z"
}
```

---

### `opencode-station_update_agent`

Update an existing agent's configuration.

**Parameters:**
```typescript
{
  agent_id: string,              // Required: Agent ID to update
  name?: string,                 // New name
  description?: string,          // New description
  prompt?: string,               // New system prompt
  enabled?: boolean,             // Enable/disable agent
  max_steps?: number,            // New max steps
  output_schema?: string,        // New output schema
  output_schema_preset?: string  // New schema preset
}
```

**Example:**
```typescript
opencode-station_update_agent({
  agent_id: "42",
  description: "Enhanced cost analysis with forecasting",
  max_steps: 8
})
```

---

### `opencode-station_update_agent_prompt`

Update only an agent's system prompt.

**Parameters:**
```typescript
{
  agent_id: string,    // Agent ID
  prompt: string       // New system prompt
}
```

**Example:**
```typescript
opencode-station_update_agent_prompt({
  agent_id: "42",
  prompt: "You are an expert FinOps analyst with forecasting capabilities..."
})
```

---

### `opencode-station_list_agents`

List all agents with pagination.

**Parameters:**
```typescript
{
  environment_id?: string,  // Filter by environment
  enabled_only?: boolean,   // Show only enabled agents (default: false)
  limit?: number,           // Max results (default: 50)
  offset?: number           // Pagination offset (default: 0)
}
```

**Example:**
```typescript
opencode-station_list_agents({
  environment_id: "3",
  enabled_only: true,
  limit: 10
})
```

**Returns:**
```json
{
  "agents": [
    {
      "id": "42",
      "name": "AWS Cost Analyzer",
      "description": "Cost analysis and optimization",
      "environment_id": "3",
      "enabled": true,
      "max_steps": 5,
      "created_at": "2025-11-19T10:30:00Z"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

---

### `opencode-station_get_agent_details`

Get detailed information about a specific agent.

**Parameters:**
```typescript
{
  agent_id: string  // Agent ID
}
```

**Returns:**
```json
{
  "id": "42",
  "name": "AWS Cost Analyzer",
  "description": "Cost analysis and optimization",
  "prompt": "---\nmetadata:\n  name: \"AWS Cost Analyzer\"...",
  "environment_id": "3",
  "enabled": true,
  "max_steps": 5,
  "tools": ["__get_cost_and_usage", "__list_cost_allocation_tags"],
  "created_at": "2025-11-19T10:30:00Z",
  "updated_at": "2025-11-19T11:15:00Z"
}
```

---

### `opencode-station_get_agent_schema`

Get input schema and available variables for an agent's dotprompt template.

**Parameters:**
```typescript
{
  agent_id: string  // Agent ID
}
```

**Returns:**
```json
{
  "agent_id": "42",
  "input_schema": {
    "type": "object",
    "properties": {
      "userInput": {
        "type": "string",
        "description": "User query or task description"
      },
      "time_range": {
        "type": "string",
        "description": "Time range for cost analysis"
      }
    }
  },
  "available_variables": ["userInput", "time_range"]
}
```

---

### `opencode-station_delete_agent`

Delete an agent.

**Parameters:**
```typescript
{
  agent_id: string  // Agent ID to delete
}
```

**Example:**
```typescript
opencode-station_delete_agent({ agent_id: "42" })
```

---

## Agent Execution

### `opencode-station_call_agent`

Execute an AI agent with advanced options and monitoring.

**Parameters:**
```typescript
{
  agent_id: string,         // Agent ID to execute
  task: string,             // Task or input for the agent
  variables?: object,       // Variables for dotprompt rendering (e.g., {"time_range": "30d"})
  context?: object,         // Additional context for the agent
  store_run?: boolean,      // Store execution in runs history (default: true)
  async?: boolean,          // Execute asynchronously and return run ID (default: false)
  timeout?: number          // Execution timeout in seconds (default: 300)
}
```

**Example:**
```typescript
opencode-station_call_agent({
  agent_id: "42",
  task: "Analyze AWS costs for the last 30 days and identify top 5 cost drivers",
  variables: {
    time_range: "30d",
    region: "us-east-1"
  },
  store_run: true,
  timeout: 600
})
```

**Returns (Synchronous):**
```json
{
  "run_id": "1234",
  "status": "success",
  "result": "Cost Analysis Results:\n\n1. EC2 Instances: $45,231 (35%)\n2. RDS Databases: $32,100 (25%)...",
  "execution_time_ms": 12450,
  "steps_taken": 3,
  "tool_calls": 5
}
```

**Returns (Async):**
```json
{
  "run_id": "1234",
  "status": "running",
  "message": "Agent execution started asynchronously"
}
```

---

### `opencode-station_list_runs`

List agent execution runs with pagination.

**Parameters:**
```typescript
{
  agent_id?: string,     // Filter by specific agent
  status?: string,       // Filter by status: 'success', 'error', 'running'
  limit?: number,        // Max results (default: 50)
  offset?: number        // Pagination offset (default: 0)
}
```

**Example:**
```typescript
opencode-station_list_runs({
  agent_id: "42",
  status: "success",
  limit: 10
})
```

**Returns:**
```json
{
  "runs": [
    {
      "id": "1234",
      "agent_id": "42",
      "agent_name": "AWS Cost Analyzer",
      "status": "success",
      "started_at": "2025-11-19T10:00:00Z",
      "completed_at": "2025-11-19T10:00:12Z",
      "execution_time_ms": 12450,
      "steps_taken": 3,
      "tool_calls": 5
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

---

### `opencode-station_list_runs_by_model`

List agent runs filtered by AI model name.

**Parameters:**
```typescript
{
  model_name: string,    // Model name (e.g., 'openai/gpt-4o-mini', 'openai/gpt-4o')
  limit?: number,        // Max results (default: 50)
  offset?: number        // Pagination offset (default: 0)
}
```

**Example:**
```typescript
opencode-station_list_runs_by_model({
  model_name: "openai/gpt-4o-mini",
  limit: 20
})
```

**Use Case:** Compare performance across different models.

---

### `opencode-station_inspect_run`

Get detailed information about a specific agent run.

**Parameters:**
```typescript
{
  run_id: string,        // Run ID to inspect
  verbose?: boolean      // Include detailed tool calls and steps (default: true)
}
```

**Example:**
```typescript
opencode-station_inspect_run({
  run_id: "1234",
  verbose: true
})
```

**Returns:**
```json
{
  "id": "1234",
  "agent_id": "42",
  "agent_name": "AWS Cost Analyzer",
  "status": "success",
  "started_at": "2025-11-19T10:00:00Z",
  "completed_at": "2025-11-19T10:00:12Z",
  "execution_time_ms": 12450,
  "steps_taken": 3,
  "tool_calls": [
    {
      "tool": "__get_cost_and_usage",
      "arguments": {"time_period": "LAST_30_DAYS"},
      "result": "{\"costs\": [...]}",
      "duration_ms": 3200
    },
    {
      "tool": "__list_cost_allocation_tags",
      "arguments": {},
      "result": "{\"tags\": [...]}",
      "duration_ms": 1100
    }
  ],
  "execution_steps": [
    {"step": 1, "action": "Called __get_cost_and_usage", "timestamp": "2025-11-19T10:00:01Z"},
    {"step": 2, "action": "Analyzed cost data", "timestamp": "2025-11-19T10:00:05Z"},
    {"step": 3, "action": "Generated recommendations", "timestamp": "2025-11-19T10:00:12Z"}
  ],
  "result": "Cost Analysis Results:\n\n1. EC2 Instances: $45,231 (35%)...",
  "token_usage": {
    "input_tokens": 1250,
    "output_tokens": 890,
    "total_tokens": 2140
  }
}
```

---

## Evaluation & Testing

### `opencode-station_generate_and_test_agent`

Generate test scenarios and execute comprehensive agent testing pipeline with full trace capture. Runs asynchronously.

**Parameters:**
```typescript
{
  agent_id: string,                // Agent ID to test
  scenario_count?: number,         // Number of test scenarios (default: 100)
  variation_strategy?: string,     // Strategy: 'comprehensive', 'edge_cases', 'common' (default: 'comprehensive')
  max_concurrent?: number,         // Max concurrent executions (default: 10)
  jaeger_url?: string              // Jaeger URL for traces (default: http://localhost:16686)
}
```

**Example:**
```typescript
opencode-station_generate_and_test_agent({
  agent_id: "42",
  scenario_count: 50,
  variation_strategy: "comprehensive",
  max_concurrent: 5
})
```

**Returns:**
```json
{
  "task_id": "test-42-20251119-103045",
  "status": "running",
  "message": "Test generation and execution started",
  "dataset_path": "/workspace/environments/default/datasets/agent-42-20251119-103045"
}
```

**Results Saved To:**
- `dataset.json` - Genkit-compatible execution traces
- `summary.json` - Test execution summary
- Jaeger traces for visualization

---

### `opencode-station_evaluate_benchmark`

Evaluate an agent run asynchronously using LLM-as-judge metrics.

**Parameters:**
```typescript
{
  run_id: string  // ID of completed agent run to evaluate
}
```

**Example:**
```typescript
opencode-station_evaluate_benchmark({
  run_id: "1234"
})
```

**Returns:**
```json
{
  "task_id": "eval-1234-20251119-103500",
  "status": "running",
  "message": "Evaluation started for run 1234"
}
```

---

### `opencode-station_get_benchmark_status`

Check the status of a benchmark evaluation task.

**Parameters:**
```typescript
{
  task_id: string  // Benchmark task ID
}
```

**Example:**
```typescript
opencode-station_get_benchmark_status({
  task_id: "eval-1234-20251119-103500"
})
```

**Returns:**
```json
{
  "task_id": "eval-1234-20251119-103500",
  "status": "completed",
  "run_id": "1234",
  "quality_score": 8.5,
  "metrics": {
    "accuracy": 0.92,
    "completeness": 0.88,
    "relevance": 0.85
  },
  "completed_at": "2025-11-19T10:36:00Z"
}
```

---

### `opencode-station_list_benchmark_results`

List benchmark evaluation results.

**Parameters:**
```typescript
{
  run_id?: string,    // Filter by specific run
  limit?: number      // Max results (default: 10)
}
```

**Example:**
```typescript
opencode-station_list_benchmark_results({
  limit: 20
})
```

---

### `opencode-station_evaluate_dataset`

Perform comprehensive LLM-as-judge evaluation on an entire dataset of agent runs.

**Parameters:**
```typescript
{
  dataset_path: string  // Absolute path to dataset directory with dataset.json
}
```

**Example:**
```typescript
opencode-station_evaluate_dataset({
  dataset_path: "/workspace/environments/default/datasets/agent-42-20251119-103045"
})
```

**Returns:**
```json
{
  "dataset_path": "/workspace/environments/default/datasets/agent-42-20251119-103045",
  "total_runs": 50,
  "evaluated": 50,
  "aggregate_scores": {
    "quality": 8.2,
    "accuracy": 0.89,
    "completeness": 0.85,
    "relevance": 0.87
  },
  "tool_effectiveness": {
    "__get_cost_and_usage": 0.92,
    "__list_cost_allocation_tags": 0.88
  },
  "production_ready": true,
  "evaluation_completed_at": "2025-11-19T11:00:00Z"
}
```

---

### `opencode-station_export_dataset`

Export agent runs and execution traces to Genkit-compatible JSON format.

**Parameters:**
```typescript
{
  filter_agent_id?: string,    // Filter by agent ID
  filter_model?: string,       // Filter by model name (e.g., 'openai/gpt-4o-mini')
  limit?: number,              // Max runs to export (default: 100)
  offset?: number,             // Pagination offset (default: 0)
  output_dir?: string          // Output directory (default: ./evals/)
}
```

**Example:**
```typescript
opencode-station_export_dataset({
  filter_agent_id: "42",
  limit: 50,
  output_dir: "/workspace/exports"
})
```

---

### `opencode-station_batch_execute_agents`

Execute multiple agents concurrently for testing and evaluation.

**Parameters:**
```typescript
{
  tasks: string,            // JSON array of tasks: [{"agent_id": 1, "task": "...", "variables": {...}}]
  iterations?: number,      // Times to execute each task (default: 1, max: 100)
  max_concurrent?: number,  // Max concurrent executions (default: 5, max: 20)
  store_runs?: boolean      // Store results in database (default: true)
}
```

**Example:**
```typescript
opencode-station_batch_execute_agents({
  tasks: JSON.stringify([
    {"agent_id": "42", "task": "Analyze costs", "variables": {"time_range": "30d"}},
    {"agent_id": "43", "task": "Security audit", "variables": {"severity": "high"}}
  ]),
  iterations: 5,
  max_concurrent: 10
})
```

---

## Reports & Analytics

### `opencode-station_create_report`

Create a new report to evaluate how well the agent team achieves its business purpose.

**Parameters:**
```typescript
{
  name: string,                  // Report name
  environment_id: string,        // Environment ID to evaluate
  description?: string,          // Report description
  team_criteria: string,         // JSON defining team's business goals and success criteria
  agent_criteria?: string,       // JSON defining individual agent contributions
  filter_model?: string          // Filter by model name for comparison
}
```

**Example:**
```typescript
opencode-station_create_report({
  name: "SRE Team Performance Q4",
  environment_id: "3",
  description: "Quarterly evaluation of SRE incident response team",
  team_criteria: JSON.stringify({
    goal: "Minimize incident MTTR and prevent recurring issues",
    criteria: {
      mttr_reduction: {
        weight: 0.4,
        description: "Reduce mean time to resolution by 30%",
        threshold: 0.7
      },
      root_cause_accuracy: {
        weight: 0.3,
        description: "Correctly identify root causes in 90% of incidents",
        threshold: 0.9
      },
      prevention_rate: {
        weight: 0.3,
        description: "Prevent 50% of similar incidents from recurring",
        threshold: 0.5
      }
    }
  }),
  agent_criteria: JSON.stringify({
    "Incident Coordinator": {
      delegation_quality: { weight: 0.5, threshold: 0.8 },
      coordination_speed: { weight: 0.5, threshold: 0.7 }
    }
  })
})
```

---

### `opencode-station_generate_report`

Generate a report by running benchmarks and LLM-as-judge evaluation on all agents.

**Parameters:**
```typescript
{
  report_id: string  // Report ID to generate
}
```

**Example:**
```typescript
opencode-station_generate_report({
  report_id: "5"
})
```

**Returns:**
```json
{
  "report_id": "5",
  "status": "running",
  "message": "Report generation started for SRE Team Performance Q4"
}
```

---

### `opencode-station_get_report`

Get detailed information about a specific report.

**Parameters:**
```typescript
{
  report_id: string  // Report ID
}
```

**Returns:**
```json
{
  "id": "5",
  "name": "SRE Team Performance Q4",
  "environment_id": "3",
  "status": "completed",
  "team_score": 7.5,
  "team_breakdown": {
    "mttr_reduction": 0.72,
    "root_cause_accuracy": 0.88,
    "prevention_rate": 0.58
  },
  "agent_scores": {
    "Incident Coordinator": 8.2,
    "Kubernetes Expert": 7.8,
    "Log Analyzer": 7.1
  },
  "generated_at": "2025-11-19T12:00:00Z"
}
```

---

### `opencode-station_list_reports`

List all reports.

**Parameters:**
```typescript
{
  environment_id?: string,  // Filter by environment
  limit?: number,           // Max results (default: 50)
  offset?: number           // Pagination offset (default: 0)
}
```

---

## Environment Management

### `opencode-station_list_environments`

List all available environments.

**Parameters:** None

**Returns:**
```json
{
  "environments": [
    {
      "id": "1",
      "name": "default",
      "description": "Default environment",
      "created_at": "2025-11-01T10:00:00Z"
    },
    {
      "id": "3",
      "name": "station-sre",
      "description": "SRE incident response team",
      "created_at": "2025-11-15T14:30:00Z"
    }
  ]
}
```

---

### `opencode-station_create_environment`

Create a new environment with file-based configuration.

**Parameters:**
```typescript
{
  name: string,             // Environment name
  description?: string      // Environment description
}
```

**Example:**
```typescript
opencode-station_create_environment({
  name: "security-team",
  description: "Security scanning and compliance agents"
})
```

---

### `opencode-station_delete_environment`

Delete an environment and all its associated data.

**Parameters:**
```typescript
{
  name: string,      // Environment name to delete
  confirm: boolean   // Must be true to proceed
}
```

**Example:**
```typescript
opencode-station_delete_environment({
  name: "old-environment",
  confirm: true
})
```

---

## MCP Server Management

### `opencode-station_add_mcp_server_to_environment`

Add an MCP server to an environment.

**Parameters:**
```typescript
{
  environment_name: string,    // Environment name
  server_name: string,         // MCP server name
  command: string,             // Command to execute
  args?: string[],             // Command arguments
  env?: object,                // Environment variables
  description?: string         // Server description
}
```

**Example:**
```typescript
opencode-station_add_mcp_server_to_environment({
  environment_name: "security-team",
  server_name: "filesystem",
  command: "npx",
  args: ["-y", "@modelcontextprotocol/server-filesystem@latest", "/home/user/projects"],
  description: "Filesystem access for security scanning"
})
```

---

### `opencode-station_update_mcp_server_in_environment`

Update an MCP server configuration.

**Parameters:**
```typescript
{
  environment_name: string,    // Environment name
  server_name: string,         // Server name to update
  command: string,             // New command
  args?: string[],             // New arguments
  env?: object,                // New environment variables
  description?: string         // New description
}
```

---

### `opencode-station_delete_mcp_server_from_environment`

Delete an MCP server from an environment.

**Parameters:**
```typescript
{
  environment_name: string,    // Environment name
  server_name: string          // Server name to delete
}
```

---

### `opencode-station_list_mcp_servers_for_environment`

List all MCP servers configured for an environment.

**Parameters:**
```typescript
{
  environment_name: string  // Environment name
}
```

**Returns:**
```json
{
  "environment": "security-team",
  "mcp_servers": [
    {
      "name": "filesystem",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "/home/user/projects"],
      "description": "Filesystem access for security scanning"
    },
    {
      "name": "ship",
      "command": "ship",
      "args": ["mcp", "security", "--stdio"],
      "description": "Security scanning tools"
    }
  ]
}
```

---

### `opencode-station_list_mcp_configs`

List all MCP configurations.

**Parameters:**
```typescript
{
  environment_id?: string  // Filter by environment ID
}
```

---

## Tool Discovery

### `opencode-station_list_tools`

List available MCP tools with pagination support.

**Parameters:**
```typescript
{
  environment_id?: string,  // Filter by environment
  search?: string,          // Search term to filter tools
  limit?: number,           // Max results (default: 50)
  offset?: number           // Pagination offset (default: 0)
}
```

**Example:**
```typescript
opencode-station_list_tools({
  environment_id: "3",
  search: "aws",
  limit: 20
})
```

**Returns:**
```json
{
  "tools": [
    {
      "name": "__get_cost_and_usage",
      "description": "Query AWS Cost Explorer for cost and usage data",
      "mcp_server": "aws",
      "input_schema": {
        "type": "object",
        "properties": {
          "time_period": {"type": "string"}
        }
      }
    }
  ],
  "total": 107,
  "limit": 20,
  "offset": 0
}
```

---

### `opencode-station_discover_tools`

Discover available MCP tools from configurations.

**Parameters:**
```typescript
{
  environment_id?: string,  // Environment ID to filter
  config_id?: string        // Specific MCP config ID
}
```

---

## Scheduling

### `opencode-station_set_schedule`

Configure an agent to run on a schedule with specified input variables.

**Parameters:**
```typescript
{
  agent_id: string,              // Agent ID to schedule
  cron_schedule: string,         // 6-field cron expression (includes seconds)
  schedule_variables?: string,   // JSON object with input variables
  enabled?: boolean              // Enable/disable schedule (default: true)
}
```

**Example:**
```typescript
opencode-station_set_schedule({
  agent_id: "42",
  cron_schedule: "0 0 2 * * *",  // Daily at 2 AM
  schedule_variables: JSON.stringify({
    time_range: "24h",
    alert_threshold: "10000"
  }),
  enabled: true
})
```

**Cron Format (6 fields):**
```
┌───────────── second (0-59)
│ ┌───────────── minute (0-59)
│ │ ┌───────────── hour (0-23)
│ │ │ ┌───────────── day of month (1-31)
│ │ │ │ ┌───────────── month (1-12)
│ │ │ │ │ ┌───────────── day of week (0-6, Sunday = 0)
│ │ │ │ │ │
0 0 2 * * *  # Daily at 2:00 AM
0 */15 * * * *  # Every 15 minutes
0 0 */6 * * *  # Every 6 hours
```

---

### `opencode-station_get_schedule`

Get an agent's current schedule configuration.

**Parameters:**
```typescript
{
  agent_id: string  // Agent ID
}
```

**Returns:**
```json
{
  "agent_id": "42",
  "cron_schedule": "0 0 2 * * *",
  "schedule_variables": {
    "time_range": "24h",
    "alert_threshold": "10000"
  },
  "enabled": true,
  "next_run": "2025-11-20T02:00:00Z",
  "last_run": "2025-11-19T02:00:00Z"
}
```

---

### `opencode-station_remove_schedule`

Remove/disable an agent's schedule configuration.

**Parameters:**
```typescript
{
  agent_id: string  // Agent ID
}
```

---

## Bundles

### `opencode-station_create_bundle_from_environment`

Create an API-compatible bundle (.tar.gz) from a Station environment.

**Parameters:**
```typescript
{
  environmentName: string,   // Environment name to bundle
  outputPath?: string        // Output path (default: <environment>.tar.gz)
}
```

**Example:**
```typescript
opencode-station_create_bundle_from_environment({
  environmentName: "station-sre",
  outputPath: "/workspace/bundles/station-sre.tar.gz"
})
```

**Returns:**
```json
{
  "bundle_path": "/workspace/bundles/station-sre.tar.gz",
  "environment": "station-sre",
  "agents_included": 9,
  "mcp_servers": ["filesystem", "aws", "grafana"],
  "size_bytes": 45678
}
```

---

## Faker System

### `opencode-station_faker_create_standalone`

Create a standalone faker with custom tools and AI-generated data.

**Parameters:**
```typescript
{
  environment_name: string,    // Environment where faker will be created
  faker_name: string,          // Faker name (e.g., 'prometheus-metrics')
  description: string,         // Faker description
  goal: string,                // Goal/instruction for AI data generation
  tools?: string,              // JSON array of tool definitions (optional, AI will suggest if not provided)
  persist?: boolean,           // Persist to template.json (default: true)
  auto_sync?: boolean,         // Auto-sync environment (default: true)
  debug?: boolean              // Enable debug logging (default: false)
}
```

**Example:**
```typescript
opencode-station_faker_create_standalone({
  environment_name: "dev-testing",
  faker_name: "datadog-metrics",
  description: "Simulated Datadog APM metrics for testing",
  goal: "Generate realistic Datadog APM metrics for a microservices application with 5 services, including latency, error rates, and throughput",
  persist: true,
  auto_sync: true
})
```

**Returns:**
```json
{
  "faker_name": "datadog-metrics",
  "environment": "dev-testing",
  "tools_generated": [
    {"name": "get_service_metrics", "description": "Get APM metrics for a service"},
    {"name": "list_services", "description": "List all services"}
  ],
  "status": "created",
  "synced": true
}
```

**Use Cases:**
- Test agents without real infrastructure
- Generate realistic training data
- Simulate production environments for development
- Create demo environments with fake data

---

## Multi-Agent Hierarchies

### `opencode-station_add_agent_as_tool`

Add another agent as a callable tool to create multi-agent hierarchies.

**Parameters:**
```typescript
{
  parent_agent_id: string,  // ID of parent agent that will call the child
  child_agent_id: string    // ID of child agent to add as tool
}
```

**Example:**
```typescript
opencode-station_add_agent_as_tool({
  parent_agent_id: "10",  // Incident Coordinator
  child_agent_id: "11"    // Kubernetes Expert
})
```

**Result:** Parent agent can now call child agent with `__agent_<child-name>` tool.

---

### `opencode-station_remove_agent_as_tool`

Remove a child agent from a parent agent's callable tools.

**Parameters:**
```typescript
{
  parent_agent_id: string,  // Parent agent ID
  child_agent_id: string    // Child agent ID to remove
}
```

---

### `opencode-station_add_tool`

Add a tool to an agent.

**Parameters:**
```typescript
{
  agent_id: string,   // Agent ID
  tool_name: string   // Tool name (e.g., '__read_text_file' or '__agent_specialist')
}
```

---

### `opencode-station_remove_tool`

Remove a tool from an agent.

**Parameters:**
```typescript
{
  agent_id: string,   // Agent ID
  tool_name: string   // Tool name to remove
}
```

---

## Common Workflows

### Creating a Complete Agent Team

```typescript
// 1. Create environment
opencode-station_create_environment({
  name: "security-team",
  description: "Security scanning and compliance"
})

// 2. Add MCP servers
opencode-station_add_mcp_server_to_environment({
  environment_name: "security-team",
  server_name: "filesystem",
  command: "npx",
  args: ["-y", "@modelcontextprotocol/server-filesystem@latest", "/workspace/code"]
})

// 3. Create specialist agents
const agent1 = opencode-station_create_agent({
  name: "Terraform Security Scanner",
  description: "Scans Terraform for security issues",
  environment_id: "4",
  prompt: "...",
  tool_names: ["__read_text_file", "__checkov_scan_directory"]
})

// 4. Create coordinator agent
const coordinator = opencode-station_create_agent({
  name: "Security Coordinator",
  description: "Coordinates security scanning specialists",
  environment_id: "4",
  prompt: "...",
  tool_names: []
})

// 5. Build hierarchy
opencode-station_add_agent_as_tool({
  parent_agent_id: coordinator.id,
  child_agent_id: agent1.id
})

// 6. Test agent
opencode-station_call_agent({
  agent_id: coordinator.id,
  task: "Perform comprehensive security scan of /workspace/code"
})

// 7. Schedule regular scans
opencode-station_set_schedule({
  agent_id: coordinator.id,
  cron_schedule: "0 0 2 * * *",  // Daily at 2 AM
  enabled: true
})
```

---

### Evaluating Agent Performance

```typescript
// 1. Generate test scenarios and execute
const testTask = opencode-station_generate_and_test_agent({
  agent_id: "42",
  scenario_count: 50,
  variation_strategy: "comprehensive"
})

// 2. Wait for completion, then evaluate dataset
opencode-station_evaluate_dataset({
  dataset_path: testTask.dataset_path
})

// 3. Create performance report
const report = opencode-station_create_report({
  name: "Agent 42 Performance Analysis",
  environment_id: "1",
  team_criteria: JSON.stringify({
    goal: "Accurate cost analysis with actionable recommendations",
    criteria: {
      accuracy: { weight: 0.5, threshold: 0.9 },
      actionability: { weight: 0.5, threshold: 0.8 }
    }
  })
})

// 4. Generate report
opencode-station_generate_report({
  report_id: report.id
})

// 5. Review results
opencode-station_get_report({
  report_id: report.id
})
```

---

## Best Practices

### 1. Always Use Structured Responses
Prefer MCP tools over CLI commands for programmatic access:
```typescript
// ✅ Good: Structured JSON response
opencode-station_list_agents({ environment_id: "3" })

// ❌ Avoid: CLI text parsing
bash("stn agent list --env default")
```

---

### 2. Handle Async Operations
For long-running operations, use async execution:
```typescript
// Start async execution
const result = opencode-station_call_agent({
  agent_id: "42",
  task: "Complex analysis task",
  async: true
})

// Check status later
opencode-station_inspect_run({
  run_id: result.run_id
})
```

---

### 3. Use Pagination for Large Datasets
Always paginate when listing resources:
```typescript
let offset = 0
const limit = 50

while (true) {
  const runs = opencode-station_list_runs({ limit, offset })
  
  // Process runs...
  
  if (runs.runs.length < limit) break
  offset += limit
}
```

---

### 4. Build Multi-Agent Hierarchies
Create coordinator agents that delegate to specialists:
```typescript
// Coordinator prompt
`You have access to specialist agents:
- __agent_kubernetes_expert - For K8s issues
- __agent_log_analyzer - For log analysis
- __agent_metrics_analyzer - For metrics analysis

Delegate tasks to appropriate specialists based on the incident type.`
```

---

### 5. Monitor and Evaluate
Continuously evaluate agent performance:
```typescript
// Regular evaluation
opencode-station_generate_and_test_agent({
  agent_id: "42",
  scenario_count: 100,
  variation_strategy: "comprehensive"
})

// Compare models
const gpt4runs = opencode-station_list_runs_by_model({
  model_name: "openai/gpt-4o"
})

const gpt4miniRuns = opencode-station_list_runs_by_model({
  model_name: "openai/gpt-4o-mini"
})
```

---

## Next Steps

- [Agent Development Guide](./agent-development.md) - Create agents that use these tools
- [Multi-Agent Teams](./multi-agent-teams.md) - Build coordinator hierarchies
- [Evaluation Guide](./evaluation.md) - Comprehensive testing strategies
- [MCP Tools](./mcp-tools.md) - Configure external MCP servers
