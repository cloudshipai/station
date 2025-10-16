# MCP Tools

Give your agents secure access to APIs, databases, filesystems, and services using the Model Context Protocol (MCP). This guide explains how to add tools to your environment, configure MCP servers, and create custom integrations.

## What is MCP?

**Model Context Protocol (MCP)** is an open standard created by Anthropic for connecting AI agents to external systems. Think of it as a universal adapter that lets agents safely interact with your infrastructure.

### Why MCP Instead of Direct API Integrations?

| Approach | Security | Maintenance | Ecosystem |
|----------|----------|-------------|-----------|
| **Direct Integration** | Agent has full API access | You write custom wrappers | Limited to what you build |
| **MCP Protocol** | Fine-grained permissions | Standard interface | 100+ community servers |

**Key Benefits:**
- **Security First**: Grant only specific operations (read vs write, specific endpoints)
- **Standardized**: Same MCP server works with Claude Desktop, Cursor, and Station
- **Ecosystem**: Community-maintained servers for AWS, GCP, databases, monitoring tools
- **Credential Safety**: MCP servers handle auth, agents never see credentials

## MCP Architecture

```
┌─────────────────────────────────────────┐
│  Station Agent                           │
│  "Analyze AWS costs for EC2"            │
└────────────────┬────────────────────────┘
                 │
                 │ 1. Agent calls __get_cost_and_usage
                 ▼
┌─────────────────────────────────────────┐
│  Station MCP Client                      │
│  Validates permissions, routes request   │
└────────────────┬────────────────────────┘
                 │
                 │ 2. MCP protocol request
                 ▼
┌─────────────────────────────────────────┐
│  AWS MCP Server                          │
│  Uses your AWS credentials               │
│  Calls Cost Explorer API                 │
└────────────────┬────────────────────────┘
                 │
                 │ 3. Returns cost data
                 ▼
┌─────────────────────────────────────────┐
│  Agent receives structured results       │
│  Analyzes and provides recommendations   │
└─────────────────────────────────────────┘
```

**Key Points:**
- Agents only see tool results, never credentials
- MCP servers run as separate processes with their own permissions
- Station validates each tool call against agent's allowed tools list
- Credentials are loaded from environment variables or cloud provider IAM

## Configuring MCP Servers

MCP servers are configured in your environment's `template.json` file.

### Basic Structure

```json
{
  "name": "my-environment",
  "description": "My agents with filesystem and AWS access",
  "mcpServers": {
    "server-name": {
      "command": "command-to-run",
      "args": ["arg1", "arg2"],
      "env": {
        "VAR_NAME": "value"
      }
    }
  }
}
```

### Example: Filesystem Server

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "/home/user/projects"
      ]
    }
  }
}
```

**What This Does:**
- Runs the official MCP filesystem server
- Grants access only to `/home/user/projects` directory
- Provides tools: `__read_text_file`, `__write_to_file`, `__list_directory`, etc.

### Example: AWS Server

```json
{
  "mcpServers": {
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "us-east-1"],
      "env": {
        "AWS_PROFILE": "production"
      }
    }
  }
}
```

**Credentials:**
MCP servers use standard AWS credential resolution:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. AWS profiles (`~/.aws/credentials`)
3. IAM roles (when running on EC2/ECS/Lambda)

### Using Template Variables

Make MCP configurations environment-specific with template variables:

**template.json:**
```json
{
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

**variables.yml:**
```yaml
PROJECT_ROOT: "/home/user/projects/api-service"
AWS_REGION: "us-west-2"
```

This pattern enables:
- Same configuration across dev/staging/prod with different values
- GitOps workflows (commit configs, deploy with environment-specific variables)
- No hardcoded paths or regions

## Available MCP Servers

### Official MCP Servers (Anthropic)

#### Filesystem
**Purpose**: Local file operations
**Install**: `npx -y @modelcontextprotocol/server-filesystem@latest`
**Config**:
```json
{
  "filesystem": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "/allowed/path"]
  }
}
```

**Tools Provided:**
- `__read_text_file` - Read file contents
- `__write_to_file` - Write/create files
- `__list_directory` - List directory contents
- `__directory_tree` - Recursive directory structure
- `__move_file` - Move/rename files
- `__search_files` - Search files by pattern
- `__get_file_info` - File metadata (size, modified time)

**Use Cases:** Code analysis, log parsing, configuration management

---

#### PostgreSQL
**Purpose**: PostgreSQL database queries
**Install**: `npx -y @modelcontextprotocol/server-postgres@latest`
**Config**:
```json
{
  "postgres": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-postgres@latest"],
    "env": {
      "POSTGRES_CONNECTION_STRING": "postgresql://user:pass@localhost/db"
    }
  }
}
```

**Tools Provided:**
- `__query` - Execute SELECT queries
- `__list_tables` - List database tables
- `__describe_table` - Get table schema

**Security Note:** Only provides read access. No INSERT, UPDATE, DELETE operations.

---

#### GitHub
**Purpose**: GitHub API access
**Install**: `npx -y @modelcontextprotocol/server-github@latest`
**Config**:
```json
{
  "github": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-github@latest"],
    "env": {
      "GITHUB_TOKEN": "ghp_your_token_here"
    }
  }
}
```

**Tools Provided:**
- `__list_repositories` - List user/org repositories
- `__get_file_contents` - Read repository files
- `__create_issue` - Create GitHub issues
- `__list_pull_requests` - List PRs
- `__search_code` - Search code across repositories

**Use Cases:** Automated issue triage, PR analysis, code search

---

### Community MCP Servers

#### AWS (mcp-server-aws)
**Purpose**: Comprehensive AWS operations
**Install**: `npm install -g mcp-server-aws`
**Config**:
```json
{
  "aws": {
    "command": "mcp-server-aws",
    "args": ["--region", "us-east-1", "--profile", "production"]
  }
}
```

**Tools Provided (100+):**
- **Cost Management**: `__get_cost_and_usage`, `__get_reservation_coverage`, `__get_savings_plans_coverage`
- **EC2**: `__describe_instances`, `__list_security_groups`, `__describe_volumes`
- **S3**: `__list_buckets`, `__get_bucket_policy`, `__list_objects`
- **RDS**: `__describe_db_instances`, `__describe_db_snapshots`
- **Lambda**: `__list_functions`, `__get_function_configuration`
- **IAM**: `__list_users`, `__get_role_policy`

**Repository**: https://github.com/your-org/mcp-server-aws

---

#### Ship Security Tools (ship mcp security)
**Purpose**: 300+ security scanning tools
**Install**: `npm install -g ship-cli`
**Config**:
```json
{
  "ship": {
    "command": "ship",
    "args": ["mcp", "security", "--stdio"]
  }
}
```

**Tools Provided (307):**
- **Infrastructure**: `__checkov_scan_directory`, `__tflint_directory`, `__terrascan_scan`
- **Containers**: `__trivy_scan_filesystem`, `__hadolint_dockerfile`, `__dockle_scan`
- **Secrets**: `__gitleaks_dir`, `__trufflehog_scan`
- **Code Security**: `__semgrep_scan`, `__bandit_scan`, `__gosec_scan`
- **Cloud Security**: `__aws_scout_suite`, `__kube_bench`, `__kubescape_scan`

**Repository**: https://github.com/shipengine/ship-cli

---

#### Grafana (mcp-server-grafana)
**Purpose**: Query Prometheus and Grafana dashboards
**Install**: `npm install -g mcp-server-grafana`
**Config**:
```json
{
  "grafana": {
    "command": "mcp-server-grafana",
    "env": {
      "GRAFANA_URL": "https://grafana.company.com",
      "GRAFANA_API_KEY": "your-api-key"
    }
  }
}
```

**Tools Provided:**
- `__prometheus_query` - Query Prometheus metrics
- `__get_dashboard` - Fetch Grafana dashboard config
- `__list_dashboards` - List available dashboards

**Use Cases:** Performance analysis, alerting agents, SLI/SLO monitoring

---

#### MySQL
**Purpose**: MySQL database queries
**Install**: `npm install -g mcp-server-mysql`
**Config**:
```json
{
  "mysql": {
    "command": "mcp-server-mysql",
    "env": {
      "MYSQL_HOST": "localhost",
      "MYSQL_USER": "readonly",
      "MYSQL_PASSWORD": "{{ .MYSQL_PASSWORD }}",
      "MYSQL_DATABASE": "production"
    }
  }
}
```

**Security Best Practice:** Use read-only database user for analysis agents.

---

#### Datadog
**Purpose**: Query Datadog metrics and logs
**Install**: `npm install -g mcp-server-datadog`
**Config**:
```json
{
  "datadog": {
    "command": "mcp-server-datadog",
    "env": {
      "DD_API_KEY": "{{ .DATADOG_API_KEY }}",
      "DD_APP_KEY": "{{ .DATADOG_APP_KEY }}"
    }
  }
}
```

**Tools Provided:**
- `__query_metrics` - Query time-series metrics
- `__search_logs` - Search application logs
- `__get_monitors` - List Datadog monitors

---

#### Stripe
**Purpose**: Stripe payment and subscription data
**Install**: `npm install -g mcp-server-stripe`
**Config**:
```json
{
  "stripe": {
    "command": "mcp-server-stripe",
    "env": {
      "STRIPE_API_KEY": "{{ .STRIPE_SECRET_KEY }}"
    }
  }
}
```

**Tools Provided:**
- `__list_customers` - List Stripe customers
- `__list_subscriptions` - Get subscription data
- `__list_invoices` - Query invoices
- `__get_payment_intent` - Payment details

**Use Cases:** Revenue analysis, churn prediction, subscription management

## Tool Discovery

After configuring MCP servers, discover available tools:

```bash
# List all tools
stn mcp tools

# Search for specific tools
stn mcp tools | grep aws
stn mcp tools | grep filesystem
stn mcp tools | grep checkov

# Get detailed tool information
stn mcp tools --verbose
```

**Example Output:**
```
Available Tools (321 total):

Filesystem Tools (14):
  __read_text_file          Read contents of a text file
  __write_to_file           Write content to a file
  __list_directory          List directory contents
  __directory_tree          Get recursive directory structure
  __search_files            Search for files matching pattern

AWS Tools (107):
  __get_cost_and_usage      Query AWS Cost Explorer
  __describe_ec2_instances  List EC2 instances
  __list_s3_buckets         List S3 buckets

Ship Security Tools (307):
  __checkov_scan_directory  Scan IaC for security issues
  __trivy_scan_filesystem   Container vulnerability scan
  __gitleaks_dir            Detect secrets in code
```

## Fine-Grained Access Control

Station gives you precise control over agent capabilities.

### Read-Only Analysis Agent

```yaml
---
metadata:
  name: "AWS Cost Analyzer"
tools:
  - "__get_cost_and_usage"          # ✅ Read AWS costs
  - "__list_cost_allocation_tags"   # ✅ Read cost tags
  - "__get_savings_plans_coverage"  # ✅ Read savings plans
  # ❌ No modification tools
---
```

**Security:** Agent can analyze but never modify infrastructure.

### Limited Write Agent

```yaml
---
metadata:
  name: "Security Report Generator"
tools:
  - "__read_text_file"       # ✅ Read scan results
  - "__write_to_file"        # ✅ Write reports
  # ❌ No filesystem deletion or AWS access
---
```

**Security:** Agent can generate reports but can't delete files or access cloud APIs.

### Multi-Cloud Operations Agent

```yaml
---
metadata:
  name: "Multi-Cloud Cost Optimizer"
tools:
  # AWS tools
  - "__get_cost_and_usage"
  - "__describe_ec2_instances"
  # GCP tools
  - "__query_bigquery"
  - "__list_gcp_instances"
  # Azure tools
  - "__query_azure_cost_management"
---
```

**Use Case:** SaaS companies running on multiple clouds needing unified cost visibility.

## Read vs Write Permissions

Design agents with least-privilege principle:

| Operation Type | Read Tools | Write Tools |
|----------------|------------|-------------|
| **Filesystem** | `__read_text_file`, `__list_directory`, `__search_files` | `__write_to_file`, `__delete_file`, `__move_file` |
| **AWS** | `__describe_*`, `__list_*`, `__get_*` | `__create_*`, `__delete_*`, `__modify_*` |
| **Databases** | `__query` (SELECT only) | `__execute` (INSERT/UPDATE/DELETE) |
| **GitHub** | `__get_file_contents`, `__list_*` | `__create_issue`, `__create_pr`, `__merge_pr` |

**Best Practice:** Start with read-only tools, add write permissions only when necessary.

## Common Tool Patterns

### Pattern 1: File Analysis

```yaml
tools:
  - "__read_text_file"      # Read file contents
  - "__search_files"        # Find files by name/pattern
  - "__get_file_info"       # Get file metadata
```

**Use Cases:** Log analysis, configuration auditing, code review

---

### Pattern 2: Infrastructure Security Scanning

```yaml
tools:
  - "__read_text_file"            # Read IaC files
  - "__list_directory"            # Discover project structure
  - "__checkov_scan_directory"    # Terraform security
  - "__trivy_scan_filesystem"     # Container security
  - "__gitleaks_dir"              # Secret detection
```

**Use Cases:** CI/CD security gates, pre-deployment validation

---

### Pattern 3: Cost Optimization

```yaml
tools:
  - "__get_cost_and_usage"           # AWS costs
  - "__get_reservation_coverage"     # RI coverage
  - "__describe_ec2_instances"       # Instance inventory
  - "__list_s3_buckets"              # Storage audit
```

**Use Cases:** FinOps analysis, budget monitoring, cost anomaly detection

---

### Pattern 4: Deployment Validation

```yaml
tools:
  - "__kubernetes_get_pods"       # Pod status
  - "__prometheus_query"          # Metrics validation
  - "__http_get"                  # Health check endpoints
  - "__query_logs"                # Error log search
```

**Use Cases:** Post-deployment validation, canary analysis

## Creating Custom MCP Servers

Build custom MCP servers to expose proprietary APIs or internal tools.

### Why Create Custom Servers?

- **Internal Tools**: Expose company-specific APIs to agents
- **Legacy Systems**: Wrap SOAP or XML-RPC services with modern MCP interface
- **Custom Business Logic**: Implement domain-specific operations
- **Data Aggregation**: Combine multiple APIs into unified tools

### MCP Server Structure

```
my-mcp-server/
├── package.json
├── src/
│   ├── index.ts          # Main server entry point
│   ├── tools.ts          # Tool definitions
│   └── handlers.ts       # Tool implementation
└── README.md
```

### Example: Custom Slack MCP Server

**package.json:**
```json
{
  "name": "mcp-server-slack",
  "version": "1.0.0",
  "bin": {
    "mcp-server-slack": "./dist/index.js"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.0.0",
    "@slack/web-api": "^6.0.0"
  }
}
```

**src/index.ts:**
```typescript
import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { WebClient } from '@slack/web-api';

const slack = new WebClient(process.env.SLACK_BOT_TOKEN);

const server = new Server(
  {
    name: 'mcp-server-slack',
    version: '1.0.0',
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// Register tools
server.setRequestHandler('tools/list', async () => ({
  tools: [
    {
      name: 'send_message',
      description: 'Send a message to a Slack channel',
      inputSchema: {
        type: 'object',
        properties: {
          channel: { type: 'string', description: 'Channel ID or name' },
          message: { type: 'string', description: 'Message text' },
        },
        required: ['channel', 'message'],
      },
    },
    {
      name: 'list_channels',
      description: 'List all Slack channels',
      inputSchema: { type: 'object', properties: {} },
    },
  ],
}));

// Implement tool handlers
server.setRequestHandler('tools/call', async (request) => {
  const { name, arguments: args } = request.params;

  if (name === 'send_message') {
    const result = await slack.chat.postMessage({
      channel: args.channel,
      text: args.message,
    });
    return { content: [{ type: 'text', text: JSON.stringify(result) }] };
  }

  if (name === 'list_channels') {
    const result = await slack.conversations.list();
    return { content: [{ type: 'text', text: JSON.stringify(result.channels) }] };
  }

  throw new Error(`Unknown tool: ${name}`);
});

// Start server
const transport = new StdioServerTransport();
await server.connect(transport);
```

**Usage in Station:**
```json
{
  "mcpServers": {
    "slack": {
      "command": "node",
      "args": ["/path/to/mcp-server-slack/dist/index.js"],
      "env": {
        "SLACK_BOT_TOKEN": "{{ .SLACK_BOT_TOKEN }}"
      }
    }
  }
}
```

### MCP Server Best Practices

1. **Input Validation**: Validate all tool arguments before execution
2. **Error Handling**: Return structured errors, never crash the server
3. **Authentication**: Use environment variables for credentials
4. **Rate Limiting**: Respect API rate limits, implement backoff
5. **Logging**: Log tool calls for audit and debugging
6. **Documentation**: Provide clear tool descriptions and examples

### Publishing Custom Servers

Share your MCP server with the community:

1. **Publish to npm**: `npm publish` (for Node.js servers)
2. **GitHub Release**: Tag releases for version management
3. **Documentation**: Include usage examples and configuration guide
4. **MCP Registry**: Submit to https://mcp.anthropic.com/servers

## Troubleshooting

### "Tool not found" Error

**Problem**: Agent tries to use a tool that doesn't exist.

**Debug Steps:**
```bash
# 1. Verify MCP server is configured
cat ~/.config/station/environments/my-env/template.json

# 2. Check MCP servers are connected
stn mcp list --env my-env

# 3. List available tools
stn mcp tools | grep tool_name

# 4. Verify tool name matches exactly (case-sensitive)
```

**Common Cause**: Tool name typo or MCP server not started.

---

### MCP Server Connection Failed

**Problem**: Station can't connect to MCP server.

**Debug Steps:**
```bash
# 1. Test MCP server command manually
npx -y @modelcontextprotocol/server-filesystem@latest /tmp

# 2. Check environment variables are set
echo $AWS_PROFILE
echo $OPENAI_API_KEY

# 3. Review Station logs
stn up  # Watch for MCP connection errors

# 4. Verify executable is in PATH
which mcp-server-aws
```

**Common Causes:**
- MCP server not installed
- Missing environment variables
- Invalid command or arguments in template.json

---

### Tool Permission Denied

**Problem**: MCP server returns permission denied.

**Debug Steps:**
```bash
# 1. Check filesystem permissions
ls -la /path/to/file

# 2. Verify AWS/cloud credentials
aws sts get-caller-identity  # AWS
gcloud auth list             # GCP

# 3. Test tool manually
npx -y @modelcontextprotocol/server-filesystem@latest /restricted/path
```

**Common Causes:**
- Filesystem: Agent accessing path outside allowed directory
- AWS: IAM permissions insufficient for operation
- Database: User lacks required privileges

---

### Slow Tool Response

**Problem**: Tools take too long to respond.

**Solutions:**
- **Filesystem**: Use `__search_files` with specific patterns instead of scanning entire directories
- **Cloud APIs**: Limit query scope (specific regions, time ranges, filters)
- **Databases**: Add indexes, optimize queries, use read replicas
- **Network**: Check MCP server location (prefer same region/network)

## Security Best Practices

### 1. Principle of Least Privilege

Grant agents only the tools they need:

```yaml
# ❌ Over-privileged agent
tools:
  - "__read_text_file"
  - "__write_to_file"
  - "__delete_file"
  - "__execute_command"
  - "__aws_create_instance"
  - "__aws_delete_instance"

# ✅ Focused agent
tools:
  - "__read_text_file"
  - "__search_files"
```

### 2. Environment Isolation

Use separate environments for different privilege levels:

```
environments/
├── production-readonly/    # Read-only production access
├── staging/                # Full staging access
└── development/            # Full development access
```

### 3. Credential Management

**Never hardcode credentials:**

```json
{
  "mcpServers": {
    "aws": {
      "command": "mcp-server-aws",
      "env": {
        "AWS_ACCESS_KEY_ID": "{{ .AWS_ACCESS_KEY_ID }}",
        "AWS_SECRET_ACCESS_KEY": "{{ .AWS_SECRET_ACCESS_KEY }}"
      }
    }
  }
}
```

**Load from environment or secrets manager:**
```bash
export AWS_ACCESS_KEY_ID=$(vault kv get -field=access_key secret/aws)
export AWS_SECRET_ACCESS_KEY=$(vault kv get -field=secret_key secret/aws)
stn up
```

### 4. Audit Logging

Enable execution logging for compliance:

```bash
# All tool calls are logged in runs
stn runs list

# Detailed tool call audit
stn runs inspect <run-id> -v

# Export for compliance
stn runs list --format json > audit-log.json
```

### 5. Network Isolation

Run MCP servers in isolated networks:
- Production agents: VPC with production database access only
- Development agents: Separate VPC, no production access
- Use security groups to enforce network policies

## Next Steps

- [Agent Development](./agent-development.md) - Create agents using MCP tools
- [Template Variables](./templates.md) - Secure configuration management
- [Examples](./examples.md) - Real-world agents with MCP tools
- [MCP Server Registry](https://mcp.anthropic.com) - Discover community servers
