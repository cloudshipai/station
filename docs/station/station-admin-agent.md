# Station Admin Agent

The Station Admin Agent is a meta-agent that manages Station itself using the Station Management API. It demonstrates the power of OpenAPI MCP servers by enabling AI-driven infrastructure management.

## Table of Contents
- [Overview](#overview)
- [Quick Start](#quick-start)
- [Capabilities](#capabilities)
- [Installation](#installation)
- [Usage Examples](#usage-examples)
- [Advanced Patterns](#advanced-patterns)
- [Best Practices](#best-practices)

---

## Overview

### What is the Station Admin Agent?

The Station Admin Agent is a self-managing agent that can:
- **List and inspect** environments, agents, MCP servers, and tools
- **Create new agents** from natural language requirements
- **Execute agents** and monitor their execution
- **Manage environments** and their configurations
- **Provide comprehensive overviews** of your Station deployment

It's powered by the Station Management API exposed as an OpenAPI MCP server, demonstrating how any REST API can become agent-accessible tools.

### Why Use It?

- ✅ **Automate Station management** - No manual YAML editing or CLI commands
- ✅ **Natural language interface** - "Create a FinOps agent that analyzes AWS costs"
- ✅ **Self-documenting** - Agent knows all available tools and their schemas
- ✅ **Production-ready** - Can manage multi-environment deployments programmatically

---

## Quick Start

### 1. Install Station API MCP Server

The Station API is available in the MCP directory. Install via UI or manually:

**Via UI** (http://localhost:8585):
1. Navigate to **MCP Directory**
2. Find **"Station Management API"**
3. Click **Install** for your environment
4. Sync environment

**Via Manual Installation**:
```bash
# Copy template files
cp mcp-servers/station-api.json ~/.config/station/environments/default/
cp mcp-servers/station-api.openapi.json ~/.config/station/environments/default/

# Sync to discover tools
stn sync default
```

**Verify**:
```bash
stn sync default 2>&1 | grep station-api
```

Expected output:
```
MCP SUCCESS: Discovered 11 tools from server 'station-api'
  🔧 Tool 1: __createAgent
  🔧 Tool 2: __deleteAgent
  🔧 Tool 3: __executeAgent
  🔧 Tool 4: __getAgent
  🔧 Tool 5: __getRun
  🔧 Tool 6: __listAgents
  🔧 Tool 7: __listEnvironments
  🔧 Tool 8: __listMCPServers
  🔧 Tool 9: __listRuns
  🔧 Tool 10: __listTools
  🔧 Tool 11: __updateAgent
```

### 2. Create Station Admin Agent

**`~/.config/station/environments/default/agents/station-admin.prompt`**:
```yaml
---
metadata:
  name: "Station Admin"
  description: "Manages Station environments, agents, and MCP servers"
  tags: ["admin", "management", "station"]
model: gpt-4o-mini
max_steps: 10
tools:
  - "__listEnvironments"
  - "__listAgents"
  - "__listMCPServers"
  - "__listTools"
  - "__listRuns"
  - "__getAgent"
  - "__getRun"
  - "__createAgent"
  - "__updateAgent"
  - "__deleteAgent"
  - "__executeAgent"
---

{{role "system"}}
You are a Station administrator that helps manage environments, agents, and MCP servers.

**Your Capabilities:**
1. **Environment Management**: List and inspect all Station environments
2. **Agent Management**: Create, list, inspect, update, and delete agents
3. **MCP Server Management**: List and inspect MCP servers and their tools
4. **Execution Monitoring**: Monitor agent runs and their results
5. **Comprehensive Reporting**: Provide detailed overviews of Station deployments

**Your Workflow:**
1. **ALWAYS use the Station API tools** to retrieve current, accurate information
2. **NEVER make assumptions** or provide generic responses without calling tools first
3. **Analyze the data** you receive before responding
4. **Provide actionable insights** based on real system state

**Critical**: You MUST actually call the tools. Do NOT hallucinate data or make up information.

**Example Workflow for "Show me all environments":**
1. Call `__listEnvironments` to get environment data
2. Call `__listAgents` to see which agents exist in each environment
3. Call `__listMCPServers` to see available MCP servers
4. Synthesize the information into a comprehensive overview

{{role "user"}}
{{userInput}}
```

### 3. Sync and Test

```bash
# Sync to create the agent
stn sync default

# Run the agent
stn agent run station-admin "Provide a comprehensive overview of the Station deployment"
```

---

## Capabilities

### 1. Environment Inspection

**List all environments:**
```bash
stn agent run station-admin "List all environments and their configurations"
```

**Sample Response:**
```
Here's a comprehensive overview of your Station environments:

### Environments

1. **default**
   - ID: 1
   - Description: Default environment for development and testing
   - Created: 2025-10-17T10:09:25Z
   - Agents: 7 agents deployed
   - MCP Servers: 4 servers connected (52 tools available)

2. **production**
   - ID: 2
   - Description: Production environment for live workloads
   - Created: 2025-10-18T14:22:10Z
   - Agents: 12 agents deployed
   - MCP Servers: 8 servers connected (124 tools available)
```

### 2. Agent Discovery

**List agents in an environment:**
```bash
stn agent run station-admin "What agents are running in the default environment?"
```

**Sample Response:**
```
The default environment has 7 agents:

1. **AWS Cost Spike Analyzer** (ID: 7)
   - Model: gpt-4o-mini
   - Max Steps: 10
   - Tools: 5 (AWS Cost Explorer)
   - Purpose: Detects unusual cost increases

2. **K8s Namespace Drift RCA** (ID: 8)
   - Model: gpt-4o-mini
   - Max Steps: 12
   - Tools: 3 (OpenCost, Kubernetes)
   - Purpose: Analyzes cost drift in K8s namespaces

[... additional agents ...]
```

### 3. Agent Creation

**Create new agent from natural language:**
```bash
stn agent run station-admin "Create a FinOps agent that analyzes AWS costs using the Cost Explorer tools. It should detect anomalies and provide recommendations."
```

**Sample Response:**
```
I've created the FinOps Cost Analyzer agent with the following configuration:

**Agent Created:**
- Name: FinOps Cost Analyzer
- ID: 15
- Model: gpt-4o-mini
- Max Steps: 10
- Tools:
  - __get_cost_and_usage
  - __get_cost_anomalies
  - __get_cost_forecast
  - __get_dimension_values

The agent is ready to use. You can execute it with:
stn agent run FinOps-Cost-Analyzer "Analyze our AWS costs for the last 30 days"
```

### 4. Execution Monitoring

**Monitor recent agent runs:**
```bash
stn agent run station-admin "Show me the last 5 agent runs and their statuses"
```

**Sample Response:**
```
Here are the last 5 agent executions:

1. **Run #45** - AWS Cost Spike Analyzer
   - Status: ✅ Completed
   - Duration: 12.3s
   - Steps Taken: 5
   - Result: Identified $450 spike in EC2 costs (m5.large instances)

2. **Run #44** - K8s Namespace Drift RCA
   - Status: ✅ Completed
   - Duration: 8.7s
   - Steps Taken: 4
   - Result: Found 15% cost increase in production namespace

[... additional runs ...]
```

### 5. MCP Server Management

**List available MCP servers and tools:**
```bash
stn agent run station-admin "What MCP servers and tools are available in the default environment?"
```

**Sample Response:**
```
The default environment has 4 MCP servers with 52 total tools:

### MCP Servers

1. **station-api** (11 tools)
   - __listEnvironments
   - __listAgents
   - __createAgent
   - [... 8 more tools]

2. **aws-cost-explorer** (8 tools)
   - __get_cost_and_usage
   - __get_cost_anomalies
   - [... 6 more tools]

3. **stripe** (25 tools)
   - __list_customers
   - __create_invoice
   - [... 23 more tools]

4. **ship-semgrep** (8 tools)
   - __semgrep_security_audit_scan
   - __semgrep_scan_secrets
   - [... 6 more tools]
```

---

## Usage Examples

### Example 1: Infrastructure Overview

**Query:**
```bash
stn agent run station-admin "Give me a comprehensive overview of our Station deployment including environments, agents, and available tools"
```

**Response:**
```
# Station Deployment Overview

## Summary
- **Total Environments**: 2 (default, production)
- **Total Agents**: 19 agents across all environments
- **Total MCP Servers**: 12 servers
- **Total Tools**: 176 tools available

## Environments

### 1. default (Development)
- **7 agents** focused on FinOps and cost analysis
- **4 MCP servers**: station-api, aws-cost-explorer, stripe, ship-semgrep
- **52 tools** available
- **Recent Activity**: 12 runs in the last 24 hours

### 2. production (Live Workloads)
- **12 agents** covering FinOps, security, and deployments
- **8 MCP servers**: Expanded toolset for production operations
- **124 tools** available
- **Recent Activity**: 45 runs in the last 24 hours

## Top Active Agents
1. AWS Cost Spike Analyzer - 8 runs today
2. K8s Namespace Drift RCA - 6 runs today
3. Multi-Cloud Anomaly Explainer - 4 runs today

## Recommendations
- Consider consolidating similar agents in default environment
- Production MCP servers are well-utilized
- All environments healthy and operational
```

### Example 2: Agent Creation Workflow

**Query:**
```bash
stn agent run station-admin "I need an agent that monitors Stripe subscription cancellations and sends me a daily summary. Create it for the production environment."
```

**Response:**
```
I've created the Stripe Subscription Monitor agent in the production environment.

**Agent Configuration:**
- **Name**: Stripe Subscription Monitor
- **ID**: 23
- **Environment**: production (ID: 2)
- **Model**: gpt-4o-mini
- **Max Steps**: 8
- **Tools Assigned**:
  - __list_subscriptions (Stripe MCP)
  - __list_customers (Stripe MCP)
  - __search_stripe_resources (Stripe MCP)

**Agent Purpose:**
Monitors Stripe for subscription cancellations and provides daily summaries including:
- Number of cancellations
- Revenue impact
- Cancellation reasons (if available)
- Customer segments affected

**Suggested Schedule:**
Run daily at 9 AM with:
stn agent run stripe-subscription-monitor "Generate cancellation summary for yesterday"

**Next Steps:**
1. Test the agent manually to verify it works
2. Set up a cron job or scheduled task for daily execution
3. Consider adding webhook integration for real-time alerts
```

### Example 3: Troubleshooting

**Query:**
```bash
stn agent run station-admin "The FinOps Cost Analyzer agent is failing. Help me debug it."
```

**Response:**
```
I've inspected the FinOps Cost Analyzer agent. Here's what I found:

**Agent Details:**
- ID: 15
- Status: Active
- Last 3 runs: All failed

**Recent Run Analysis (Run #47):**
- Status: ❌ Failed
- Duration: 2.1s
- Error: "Tool __get_cost_and_usage_comparisons not found"

**Root Cause:**
The agent is configured to use `__get_cost_and_usage_comparisons` but this tool is from the `aws-cost-explorer-extended` MCP server, which is not currently installed in the default environment.

**Available Cost Explorer Tools in Default Environment:**
- __get_cost_and_usage
- __get_cost_anomalies
- __get_cost_forecast
- __get_dimension_values
- __get_tag_values
- __get_cost_comparison_drivers
- __get_today_date

**Solutions:**

**Option 1: Update Agent to Use Available Tools**
Modify the agent to use __get_cost_and_usage and __get_cost_comparison_drivers instead.

**Option 2: Install Extended MCP Server**
Install the aws-cost-explorer-extended MCP server in the default environment to provide the missing tool.

**Recommendation**: Option 1 is quickest. The available tools can provide similar functionality.

Would you like me to update the agent configuration for you?
```

---

## Advanced Patterns

### Pattern 1: Automated Agent Lifecycle Management

Create a "meta-admin" agent that manages other agents:

```yaml
---
metadata:
  name: "Agent Lifecycle Manager"
  description: "Automatically creates, updates, and removes agents based on requirements"
model: gpt-4o
max_steps: 15
tools:
  - "__listAgents"
  - "__createAgent"
  - "__updateAgent"
  - "__deleteAgent"
  - "__listTools"
---

{{role "system"}}
You are an intelligent agent lifecycle manager.

**Your Responsibilities:**
1. **Detect redundant agents** - Find agents with overlapping capabilities
2. **Suggest consolidation** - Recommend merging similar agents
3. **Create missing agents** - Identify gaps in coverage and create agents to fill them
4. **Deprecate unused agents** - Remove agents with no recent runs

**Workflow:**
1. List all agents and their tool assignments
2. Analyze for redundancy and gaps
3. Provide actionable recommendations
4. Execute approved changes

{{role "user"}}
{{userInput}}
```

**Usage:**
```bash
stn agent run agent-lifecycle-manager "Analyze our agent deployment and suggest optimizations"
```

### Pattern 2: Environment Promotion Pipeline

Automate promoting agents from dev → staging → production:

```yaml
---
metadata:
  name: "Environment Promoter"
  description: "Promotes agents through environment pipeline"
model: gpt-4o-mini
max_steps: 12
tools:
  - "__listEnvironments"
  - "__listAgents"
  - "__getAgent"
  - "__createAgent"
---

{{role "system"}}
You manage agent promotion through dev → staging → production environments.

**Promotion Checklist:**
1. Verify agent exists in source environment
2. Check agent has successful runs
3. Create identical agent in target environment
4. Verify tool availability in target environment
5. Confirm successful creation

{{role "user"}}
{{userInput}}
```

**Usage:**
```bash
stn agent run environment-promoter "Promote the AWS Cost Spike Analyzer from dev to production"
```

### Pattern 3: Self-Healing Infrastructure

Create an agent that monitors and fixes common issues:

```yaml
---
metadata:
  name: "Infrastructure Self-Healer"
  description: "Monitors and automatically fixes common Station issues"
model: gpt-4o
max_steps: 20
tools:
  - "__listAgents"
  - "__listMCPServers"
  - "__listRuns"
  - "__updateAgent"
---

{{role "system"}}
You are a self-healing infrastructure agent that monitors Station health and fixes issues.

**Your Monitoring:**
1. **Detect failing agents** - Agents with consecutive failed runs
2. **MCP server connectivity** - Servers with 0 discovered tools
3. **Stale runs** - Long-running executions that may be hung
4. **Tool mismatches** - Agents using non-existent tools

**Your Actions:**
1. Attempt automatic fixes (update agent configs, restart MCP servers)
2. Report issues that require human intervention
3. Provide detailed diagnostics and remediation steps

{{role "user"}}
{{userInput}}
```

**Usage:**
```bash
stn agent run infrastructure-self-healer "Check for issues and fix what you can"
```

---

## Best Practices

### 1. Always Use Real Data

**❌ Bad:**
```
"I assume you have 3 environments: dev, staging, and production..."
```

**✅ Good:**
```
# Agent calls __listEnvironments first
"You have 2 environments: default and production. Here are their details..."
```

### 2. Provide Actionable Insights

**❌ Bad:**
```
"There are some agents in your deployment."
```

**✅ Good:**
```
"You have 7 agents in default environment:
- 3 FinOps agents (may be redundant)
- 2 Security agents (good coverage)
- 2 Deployment agents

Recommendation: Consider consolidating the 3 FinOps agents into a single
multi-purpose cost analyzer to reduce complexity."
```

### 3. Handle Errors Gracefully

**❌ Bad:**
```
"Error occurred. Cannot proceed."
```

**✅ Good:**
```
"I encountered an error retrieving agents: 'Environment not found (ID: 99)'.

This suggests:
1. The environment may have been deleted
2. You may have specified an incorrect environment ID

Available environments:
- default (ID: 1)
- production (ID: 2)

Please specify which environment you'd like to inspect."
```

### 4. Use Explicit Tool Calls

**❌ Bad (agent prompt):**
```yaml
tools:
  - "__listAgents"
# Agent may not know when to use this
```

**✅ Good (agent prompt):**
```yaml
tools:
  - "__listAgents"
---
{{role "system"}}
When asked about agents, ALWAYS call __listAgents first to get current data.
NEVER make assumptions about agent names or configurations.
```

### 5. Provide Context for Created Agents

When creating agents via Station Admin, include:
- Clear purpose and use case
- Recommended execution schedule
- Expected outputs
- Next steps for the user

---

## Next Steps

- [Learn about OpenAPI MCP Servers →](./openapi-mcp-servers.md)
- [Explore Template Variables →](./templates.md)
- [Agent Development Guide →](./agent-development.md)
- [Browse More Examples →](./examples.md)
