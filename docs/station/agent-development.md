# Agent Development

Create powerful infrastructure management agents using Station's declarative dotprompt format. This guide covers everything from basic agent creation to advanced debugging and best practices.

## What Are Station Agents?

Station agents are AI-powered automation tools that:
- Execute multi-step infrastructure tasks autonomously
- Access your tools and APIs through MCP servers
- Understand context and make intelligent decisions
- Run on your infrastructure with your credentials

**Key Difference from ChatGPT/Claude**: Agents have direct access to your tools (AWS, databases, filesystems) and can execute complex workflows autonomously rather than requiring you to copy-paste commands.

## Dotprompt Format Reference

Dotprompt is a declarative format for defining agents, inspired by Firebase GenKit. Every agent is a `.prompt` file with YAML frontmatter and a template body.

### Complete Dotprompt Example

```yaml
---
metadata:
  name: "AWS Cost Spike Analyzer"
  description: "Detects unusual cost increases and identifies root causes"
  tags: ["finops", "aws", "cost-analysis"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__get_cost_and_usage"
  - "__list_cost_allocation_tags"
  - "__get_savings_plans_coverage"
  - "__get_reservation_coverage"
---

{{role "system"}}
You are a FinOps analyst specializing in AWS cost anomaly detection.

Your analysis process:
1. Query AWS Cost Explorer for the past 30 days
2. Identify cost spikes exceeding 20% baseline
3. Break down costs by service (EC2, RDS, S3, Lambda)
4. Analyze tag-based cost allocation to identify responsible teams
5. Check Reserved Instance and Savings Plans coverage gaps
6. Provide actionable recommendations with estimated monthly savings

Focus on:
- Unusual EC2 instance type changes or scaling events
- Untagged resources that can't be attributed to teams
- Underutilized Reserved Instances
- Savings Plans coverage opportunities

{{role "user"}}
{{userInput}}
```

### Frontmatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `metadata.name` | string | Yes | Agent display name (shown in UI) |
| `metadata.description` | string | Yes | What the agent does (1-2 sentences) |
| `metadata.tags` | array | No | Categories for organization and search |
| `model` | string | Yes | AI model: `gpt-4o`, `gpt-4o-mini`, `claude-3-5-sonnet-latest` |
| `max_steps` | integer | Yes | Maximum tool calls (5-15 typical, 3-5 for simple tasks) |
| `tools` | array | Yes | MCP tool names agent can use (use `stn mcp tools` to discover) |

### Template Body

The template body uses GenKit's templating syntax:

```yaml
{{role "system"}}
System instructions for the agent's behavior and personality.

{{role "user"}}
{{userInput}}  # User's task/query gets injected here
```

**Template Variables:**
- `{{userInput}}`: The task/query provided when running the agent
- Custom variables: Can be defined in environment `variables.yml` and referenced with `{{ .VARIABLE_NAME }}`

### Model Selection Guide

| Model | Use Case | Speed | Cost | Context |
|-------|----------|-------|------|---------|
| `gpt-4o-mini` | Simple analysis, file operations, quick tasks | Fast | Low | 128k |
| `gpt-4o` | Complex reasoning, multi-step workflows | Medium | Medium | 128k |
| `claude-3-5-sonnet-latest` | Code analysis, security audits, detailed reports | Medium | Medium | 200k |
| `claude-3-5-haiku-latest` | Fast operations, simple queries | Very Fast | Low | 200k |

### Tool Selection

Agents can only use tools explicitly listed in the `tools` array. Discover available tools:

```bash
# List all tools in current environment
stn mcp tools

# Search for specific tool categories
stn mcp tools | grep filesystem
stn mcp tools | grep aws
```

Common tool patterns:
- **Filesystem**: `__read_text_file`, `__write_to_file`, `__list_directory`, `__search_files`
- **AWS**: `__get_cost_and_usage`, `__describe_ec2_instances`, `__list_s3_buckets`
- **Databases**: `__postgresql_query`, `__mysql_query`, `__query_bigquery`
- **Security**: `__checkov_scan_directory`, `__trivy_scan_filesystem`, `__gitleaks_dir`

## Development Workflows

### Method 1: Via Claude/Cursor (Recommended)

Use Station's MCP tools directly from Claude Desktop or Cursor to create and manage agents.

**Step 1: Connect Station to Claude/Cursor**
```bash
# Add to Claude Desktop config (~/.config/claude/claude_desktop_config.json)
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"]
    }
  }
}
```

**Step 2: Create Agent via Natural Language**
In Claude Code or Cursor, simply ask:
> "Create a Station agent named cost-analyzer that analyzes AWS costs using the Cost Explorer API tools"

Claude will use MCP tools like:
- `create_agent` - Creates the agent in Station
- `list_tools` - Discovers available MCP tools
- `add_tool` - Assigns tools to the agent
- `call_agent` - Tests agent execution
- `list_runs` - Views execution history

**Step 3: Iterate and Test**
> "Run the cost-analyzer agent with task: Analyze yesterday's EC2 costs"

Claude can:
- Execute agents and show results
- Debug failures by inspecting runs
- Modify agent prompts based on results
- Add or remove tools as needed

**Advantages:**
- Natural language interface - no manual file editing
- AI-assisted tool discovery and assignment
- Immediate testing and iteration
- Full access to all Station MCP management tools

### Method 2: File-Based Development (Recommended for Advanced Users)

**Step 1: Create Environment Directory**
```bash
mkdir -p ~/.config/station/environments/my-agents/agents
cd ~/.config/station/environments/my-agents
```

**Step 2: Configure MCP Servers** (`template.json`)
```json
{
  "name": "my-agents",
  "description": "My custom agents",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    },
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "us-east-1"]
    }
  }
}
```

**Step 3: Set Variables** (`variables.yml`)
```yaml
PROJECT_ROOT: "/home/user/projects"
AWS_REGION: "us-east-1"
```

**Step 4: Create Agent File** (`agents/cost-analyzer.prompt`)
```yaml
---
metadata:
  name: "Cost Analyzer"
  description: "Analyzes AWS costs"
  tags: ["finops", "aws"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__get_cost_and_usage"
---

{{role "system"}}
You are a cost analyst. Analyze AWS spending and identify savings opportunities.

{{role "user"}}
{{userInput}}
```

**Step 5: Sync Environment**
```bash
stn sync my-agents
```

**Step 6: Test Agent**
```bash
stn agent run cost-analyzer "Analyze yesterday's AWS costs"
```

**Advantages:**
- Version control friendly (commit with your code)
- GitOps-ready deployment
- Easy to duplicate and modify
- Scriptable and automatable

### Method 3: Web UI

Use the Station web interface for MCP server management, syncing, and Docker builds.

**Step 1: Start Station Server**
```bash
stn up
# Opens http://localhost:8585
```

**Step 2: Manage MCP Servers**
- Navigate to **MCP Servers** → **Add Server**
- Configure server command, args, and environment variables
- Test connection and view available tools

**Step 3: Sync Environments**
- Go to **Environments** → Select environment
- Click **Sync** to load file-based agents and templates
- Resolve any template variable prompts

**Step 4: Build Docker Images**
- Navigate to **Bundles** → Select bundle
- Click **Build Docker Image**
- Configure environment variables and deployment settings

**Note:** The Web UI is for *infrastructure management*, not agent creation. Use Method 1 (Claude/Cursor) or Method 2 (file-based) to create agents.

## Testing and Debugging

### Run Agent with Live Output

```bash
# Run agent and stream output
stn agent run agent-name "Task description" --tail

# Run in specific environment
stn agent run agent-name "Task" --env production
```

### View Execution History

```bash
# List recent runs
stn runs list

# Detailed run inspection
stn runs inspect <run-id> -v
```

**Run details include:**
- Tool calls with parameters
- Agent reasoning at each step
- Token usage and cost
- Execution duration
- Error messages

### Debug Common Issues

**Issue: "Tool not found"**
```bash
# Verify tool is available
stn mcp tools | grep tool_name

# Check MCP server is connected
stn mcp list --env my-agents
```

**Issue: "Agent timeout" or "Max steps exceeded"**
- Increase `max_steps` in dotprompt frontmatter
- Simplify the task or break into smaller agents
- Use faster model (gpt-4o-mini vs gpt-4o)

**Issue: "Permission denied" or "Credentials not found"**
- Check MCP server configuration in `template.json`
- Verify template variables in `variables.yml`
- Ensure AWS/GCP credentials are available to MCP server process

**Issue: "Agent gives generic responses"**
- Make system prompt more specific with examples
- Add step-by-step instructions in system prompt
- Provide context about expected output format

### Test-Driven Agent Development

1. **Start with Simple Task**: Test basic functionality first
   ```bash
   stn agent run file-analyzer "List files in /tmp"
   ```

2. **Add Complexity Gradually**: Increase task difficulty
   ```bash
   stn agent run file-analyzer "Find all Python files modified in last 7 days"
   ```

3. **Test Edge Cases**: Verify error handling
   ```bash
   stn agent run file-analyzer "Analyze /nonexistent/path"
   ```

4. **Measure Performance**: Check execution time and costs
   ```bash
   stn runs inspect <run-id> -v | grep -E '(duration|tokens)'
   ```

## Best Practices

### Writing Effective System Prompts

**Do:**
- ✅ Provide step-by-step process instructions
- ✅ Include specific examples of expected output
- ✅ Define success criteria and edge case handling
- ✅ Explain the agent's role and expertise domain
- ✅ Specify output format (JSON, tables, bullet points)

**Don't:**
- ❌ Write vague instructions like "be helpful"
- ❌ Assume agent knows your business context
- ❌ Skip error handling instructions
- ❌ Create overly generic agents

**Example - Vague Prompt:**
```yaml
{{role "system"}}
You are a helpful AWS cost assistant.
```

**Example - Effective Prompt:**
```yaml
{{role "system"}}
You are a FinOps analyst for a SaaS company running on AWS.

Your analysis process:
1. Query AWS Cost Explorer for the specified time period
2. Calculate baseline average from previous 30 days
3. Identify any day with >20% cost increase
4. For spikes, break down by service and tag
5. Provide recommendations with estimated savings

Output format:
- Cost spike summary (amount, percentage, time period)
- Root cause analysis (which services/teams)
- Actionable recommendations (specific changes)
- Estimated monthly savings

If no spikes detected, confirm costs are within normal range.
```

### Tool Selection Strategy

**Principle: Minimal Viable Toolset**

Only include tools the agent actually needs:

```yaml
# ❌ Too many tools (agent gets confused)
tools:
  - "__read_text_file"
  - "__write_to_file"
  - "__list_directory"
  - "__search_files"
  - "__get_file_info"
  - "__directory_tree"
  - "__move_file"
  - "__delete_file"

# ✅ Focused toolset for specific task
tools:
  - "__read_text_file"   # Read file contents
  - "__search_files"     # Find files matching pattern
  - "__list_directory"   # List directory contents
```

**Tool Naming Convention:**
- Tools from MCP servers are prefixed with `__`
- Use `stn mcp tools` to get exact names
- Tool names must match exactly (case-sensitive)

### Model and Max Steps Guidelines

**Simple File Operations** (3-5 steps):
```yaml
model: gpt-4o-mini
max_steps: 5
# Example: Read files, search directories, basic analysis
```

**Complex Analysis** (8-12 steps):
```yaml
model: gpt-4o
max_steps: 10
# Example: Multi-step AWS cost analysis, security scanning
```

**Multi-Tool Workflows** (15-20 steps):
```yaml
model: gpt-4o
max_steps: 15
# Example: Deployment validation with health checks, metric queries, log analysis
```

### Environment Organization

**Pattern 1: By Purpose**
```
~/.config/station/environments/
├── local-dev/          # Development agents
├── cicd/               # CI/CD pipeline agents
└── production/         # Production monitoring agents
```

**Pattern 2: By Team**
```
~/.config/station/environments/
├── platform-team/      # Infrastructure agents
├── security-team/      # Security scanning agents
└── finops-team/        # Cost optimization agents
```

**Pattern 3: By Project**
```
~/.config/station/environments/
├── api-service/        # API project agents
├── data-pipeline/      # Data pipeline agents
└── mobile-backend/     # Mobile backend agents
```

### Security Best Practices

1. **Least Privilege Tools**: Only grant necessary tools to each agent
2. **Environment Isolation**: Use separate environments for dev/staging/prod
3. **Credential Management**: Use template variables for secrets, never hardcode
4. **Audit Logging**: Enable `stn runs list` logging for compliance
5. **Read-Only Agents**: Prefer read-only tools for analysis agents

Example - Read-Only Security Scanner:
```yaml
tools:
  - "__read_text_file"        # ✅ Read only
  - "__search_files"          # ✅ Read only
  - "__checkov_scan_directory" # ✅ Read only
  # ❌ No write_to_file or delete_file
```

### Performance Optimization

**Reduce Latency:**
- Use `gpt-4o-mini` for simple tasks (3-5x faster than gpt-4o)
- Limit `max_steps` to minimum required
- Use specific tool queries instead of broad searches

**Reduce Costs:**
- Use smaller models where possible
- Cache frequently accessed data in environment variables
- Batch similar tasks instead of running agents repeatedly

**Improve Reliability:**
- Test agents with realistic data before production
- Include error handling instructions in system prompt
- Set appropriate timeouts and retry logic in MCP servers

## Advanced Patterns

### Multi-Agent Workflows

Break complex tasks into specialized agents:

```yaml
# Agent 1: Discovery
---
metadata:
  name: "Terraform File Discoverer"
tools: ["__search_files", "__directory_tree"]
---
Find all Terraform files and return their paths.

# Agent 2: Analysis
---
metadata:
  name: "Terraform Security Analyzer"
tools: ["__read_text_file", "__checkov_scan_directory"]
---
Analyze Terraform files for security issues.

# Agent 3: Reporting
---
metadata:
  name: "Security Report Generator"
tools: ["__write_to_file"]
---
Generate security report from analysis results.
```

Run sequentially:
```bash
FILES=$(stn agent run terraform-file-discoverer "Find all .tf files")
ISSUES=$(stn agent run terraform-security-analyzer "Analyze $FILES")
stn agent run security-report-generator "Create report from: $ISSUES"
```

### Dynamic Tool Selection

Use environment-specific tool configurations:

```json
// dev-environment/template.json
{
  "mcpServers": {
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "us-east-1", "--profile", "dev"]
    }
  }
}

// prod-environment/template.json
{
  "mcpServers": {
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "us-west-2", "--profile", "prod"]
    }
  }
}
```

Same agent works in both environments with different credentials.

### Scheduled Agent Execution

Run agents automatically with cron:

```bash
# Daily cost analysis at 8 AM
0 8 * * * stn agent run aws-cost-analyzer "Daily cost report" >> /var/log/station-costs.log

# Weekly security scan
0 0 * * 0 stn agent run security-scanner "Weekly infrastructure scan" >> /var/log/station-security.log
```

### CI/CD Integration

```yaml
# .github/workflows/security-scan.yml
name: Security Scan
on: [pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Station
        run: curl -fsSL https://install.station.dev | bash
      - name: Run Security Scan
        run: |
          stn agent run infrastructure-security-scanner \
            "Scan for security issues and fail if critical findings" \
            --format json > results.json
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

## Debugging Checklist

When an agent doesn't work as expected:

1. **Verify Environment Setup**
   ```bash
   stn env list
   stn mcp list --env my-environment
   stn mcp tools
   ```

2. **Check Agent Configuration**
   ```bash
   stn agent list --env my-environment
   cat ~/.config/station/environments/my-environment/agents/my-agent.prompt
   ```

3. **Test with Simple Task**
   ```bash
   stn agent run my-agent "Simple test task" --tail
   ```

4. **Inspect Execution Details**
   ```bash
   stn runs list | head -1  # Get latest run ID
   stn runs inspect <run-id> -v
   ```

5. **Check MCP Server Logs**
   ```bash
   stn up  # View logs in terminal
   # Look for MCP connection errors, tool call failures
   ```

6. **Validate Template Variables**
   ```bash
   cat ~/.config/station/environments/my-environment/variables.yml
   # Ensure all variables referenced in template.json are defined
   ```

## Troubleshooting Common Patterns

### Pattern: Agent Returns Generic Responses

**Problem**: Agent gives vague answers instead of using tools.

**Solution**: Make system prompt more directive with explicit steps.

**Before:**
```yaml
{{role "system"}}
You are a cost analyst. Help with AWS costs.
```

**After:**
```yaml
{{role "system"}}
You MUST follow this process:
1. Call __get_cost_and_usage for the past 7 days
2. Calculate the average daily cost
3. Identify any days >20% above average
4. For each spike, call __get_cost_and_usage with service breakdown
5. Return findings in this format: [Date, Cost, Spike %, Root Cause]
```

### Pattern: Agent Timeout on Large Scans

**Problem**: Agent times out scanning large directories.

**Solution**: Break into discovery + analysis steps or increase max_steps.

```yaml
# Instead of: "Scan entire /home directory"
# Use: "Scan /home/user/projects excluding node_modules"

{{role "system"}}
When scanning directories:
1. Use __search_files with specific patterns (*.tf, *.py)
2. Exclude large directories (node_modules, .git, venv)
3. Process files in batches of 10
4. If you hit step limit, report partial results
```

### Pattern: Inconsistent Results Across Runs

**Problem**: Agent gives different answers for same task.

**Solution**: Use lower temperature or add deterministic instructions.

```yaml
---
model: gpt-4o-mini
max_steps: 8
temperature: 0.3  # Lower = more consistent
---

{{role "system"}}
Always return results in this exact JSON format:
{
  "findings": [...],
  "severity": "high|medium|low",
  "recommendations": [...]
}
```

## Next Steps

- [MCP Tools Reference](./mcp-tools.md) - Available tools and MCP servers
- [Template Variables](./templates.md) - Secure configuration management
- [Examples](./examples.md) - Real-world agent examples
- [Bundles](./bundles.md) - Package and share agents
