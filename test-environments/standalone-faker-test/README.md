# Standalone Faker Test Environment

This test environment demonstrates Station's standalone faker mode, which generates realistic MCP tool schemas using AI without requiring a real MCP server connection.

## Overview

This environment contains **3 standalone fakers**:
- `aws-cloudwatch-faker` - AWS CloudWatch monitoring tools
- `datadog-faker` - Datadog monitoring and APM tools  
- `stripe-faker` - Stripe payment API tools

Each faker generates 5-10 tools based on its AI instruction. Tools are cached in the database, ensuring consistency across faker restarts.

## How It Works

1. **Tool Generation**: When a standalone faker starts for the first time, it:
   - Generates tools via AI using `genkit.GenerateData` with structured output
   - Caches generated tools in `faker_tool_cache` table by faker ID
   - Serves the tools as a standard MCP server

2. **Tool Caching**: On subsequent runs:
   - Checks if tools are cached for the faker ID
   - Loads cached tools from database if available
   - Skips AI generation for fast startup

3. **MCP Compatibility**: Station treats standalone fakers as regular MCP servers:
   - Agents can discover and call the AI-generated tools
   - Tool calls are tracked in faker sessions
   - All standard faker features work (sessions, metrics, etc.)

## Usage

### Sync the Environment

```bash
stn sync standalone-faker-test
```

This will:
- Create 3 standalone faker MCP servers
- Generate tools for each faker (first run only)
- Cache tools in the database

### List MCP Servers

```bash
stn mcp list --env standalone-faker-test
```

You should see:
- `aws-cloudwatch-faker`
- `datadog-faker`
- `stripe-faker`

### Discover Tools

```bash
# List all tools from all fakers
stn mcp discover --env standalone-faker-test

# Or use Station API
curl http://localhost:3000/api/v1/mcp/tools?environment=standalone-faker-test
```

### Create an Agent

Create an agent that uses the standalone fakers:

```bash
stn agent create \
  --name "Multi-Service Monitor" \
  --description "Monitor AWS CloudWatch, Datadog, and Stripe simultaneously" \
  --env standalone-faker-test \
  --tools __get_metrics \
  --tools __list_alarms \
  --tools __query_timeseries
```

### View Faker Sessions

```bash
# List all faker sessions
stn faker sessions list

# View detailed session info
stn faker sessions view <session-id>

# View aggregated metrics
stn faker metrics
```

## Tool Cache Management

### View Cached Tools

```sql
sqlite3 ~/.config/station/station.db
SELECT faker_id, tool_name FROM faker_tool_cache;
```

### Clear Cache for a Faker

To regenerate tools for a specific faker:

```sql
DELETE FROM faker_tool_cache WHERE faker_id = 'aws-cloudwatch-faker';
```

Then restart the faker to regenerate tools.

### Clear All Tool Caches

```sql
DELETE FROM faker_tool_cache;
```

## Testing Scenarios

### Scenario 1: Tool Generation
Test that AI generates realistic tools for each domain.

### Scenario 2: Tool Consistency
Verify tools remain consistent across faker restarts.

### Scenario 3: Multi-Faker Isolation
Ensure each faker has unique tools (no cross-contamination).

### Scenario 4: Agent Integration
Test agents can discover and use AI-generated tools.

## Architecture

```
┌─────────────────────────────────────────┐
│  template.json (3 standalone fakers)    │
└─────────────────────────────────────────┘
                  │
                  │ stn sync
                  ▼
┌─────────────────────────────────────────┐
│  Check Tool Cache                       │
│  - aws-cloudwatch-faker: No cache       │
│  - datadog-faker: No cache              │
│  - stripe-faker: No cache               │
└─────────────────────────────────────────┘
                  │
                  │ Cache miss
                  ▼
┌─────────────────────────────────────────┐
│  Generate Tools with AI                 │
│  - Use genkit.GenerateData              │
│  - Structured output (5-10 tools)       │
│  - Store in faker_tool_cache            │
└─────────────────────────────────────────┘
                  │
                  │ Tools ready
                  ▼
┌─────────────────────────────────────────┐
│  Serve as MCP Servers                   │
│  - aws-cloudwatch-faker (8 tools)       │
│  - datadog-faker (7 tools)              │
│  - stripe-faker (6 tools)               │
└─────────────────────────────────────────┘
                  │
                  │ Agents discover
                  ▼
┌─────────────────────────────────────────┐
│  Agent Calls Tools                      │
│  - Tools work like real MCP servers     │
│  - Faker enriches responses with AI     │
│  - Sessions tracked in database         │
└─────────────────────────────────────────┘
```

## Expected Output

When running `stn sync`, you should see:

```
[FAKER] 🚀 NewStandaloneFaker starting for ID: aws-cloudwatch-faker
[FAKER] Initializing GenKit for standalone mode with provider: openai
[FAKER] GenKit initialized successfully
[FAKER] Generating tools with AI for instruction: Generate realistic AWS CloudWatch tools...
[FAKER] Using GenerateData with model: openai/gpt-4o-mini
[FAKER] Generated 8 tools
[FAKER]   - get_metric_data: Retrieve time-series metric data from CloudWatch
[FAKER]   - list_alarms: List all CloudWatch alarms
[FAKER]   - get_alarm_history: Get historical alarm state changes
...
```

## Notes

- **First run** takes ~10-30s per faker (AI generation time)
- **Subsequent runs** are instant (tools loaded from cache)
- Tools are **deterministic** per faker ID (same instruction = similar tools)
- Each faker runs as an **independent MCP server** process
- **No real API credentials** needed - everything is AI-generated
