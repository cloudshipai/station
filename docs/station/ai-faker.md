# AI Faker: Mock Data Generation for MCP Servers

## Overview

The Station AI Faker is an MCP server proxy that enriches responses from real MCP servers with AI-generated mock data. This enables realistic testing and development without requiring real credentials, populated databases, or production environments.

**Use Cases:**
- **Development & Testing**: Test agents with realistic data without production credentials
- **Demos & Presentations**: Generate compelling demo scenarios with realistic mock data
- **Empty Environments**: Work with AWS accounts, databases, or systems that have no real data
- **Cost Scenarios**: Simulate high-cost, low-cost, or specific business scenarios on demand
- **Security Testing**: Test security agents without exposing real vulnerabilities or sensitive data

## How It Works

The faker operates as an **MCP proxy**:

```
Agent → Faker Proxy → Real MCP Server
         ↓
    AI Enrichment (GenKit + OpenAI/Gemini)
         ↓
    Mock Data Response
```

1. **Proxy Mode**: Faker acts as an MCP server that forwards tool calls to a real target MCP server
2. **AI Enrichment**: Responses from the real server are enhanced with AI-generated mock data
3. **Instruction-Based**: Use natural language instructions to guide what kind of data to generate
4. **Transparent**: Agent sees faker as a normal MCP server, no code changes needed

## Quick Start

### Basic Usage

```bash
# Start faker as MCP proxy for filesystem server
stn faker \
  --command npx \
  --args "-y,@modelcontextprotocol/server-filesystem,/tmp" \
  --ai-instruction "Generate realistic filesystem data with varied file types, sizes, and timestamps" \
  --debug
```

### Environment Configuration

The recommended way to use faker is through Station environments:

**1. Create environment configuration** (`~/.config/station/environments/faker-test/template.json`):

```json
{
  "name": "faker-test",
  "description": "Development environment with AI-enriched mock data",
  "mcpServers": {
    "filesystem-faker": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-filesystem,/tmp",
        "--ai-instruction", "Generate realistic filesystem data with varied file types, sizes, and timestamps",
        "--debug"
      ]
    }
  }
}
```

**2. Sync environment**:

```bash
stn sync faker-test
```

**3. Create and run agents** that use the faker proxy transparently:

```bash
stn agent run <agent-id> "List the files in the directory"
```

The agent receives AI-generated mock filesystem data instead of real `/tmp` contents.

## Real-World Examples

### Example 1: AWS CloudWatch Mock Data

**Scenario**: Empty AWS account, need realistic CloudWatch monitoring data showing high costs and critical alerts.

**Configuration**:

```json
{
  "mcpServers": {
    "aws-cloudwatch-faker": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "uvx",
        "--args", "awslabs.cloudwatch-mcp-server@latest",
        "--ai-instruction", "Generate realistic AWS CloudWatch monitoring data showing high resource usage, elevated costs, and critical alerts. Include metrics for EC2, RDS, Lambda, and S3 with timestamps, high values for CPU/memory/network, and alarm states indicating production issues.",
        "--debug"
      ],
      "env": {
        "AWS_ACCESS_KEY_ID": "{{ .AWS_ACCESS_KEY_ID }}",
        "AWS_SECRET_ACCESS_KEY": "{{ .AWS_SECRET_ACCESS_KEY }}",
        "AWS_REGION": "{{ .AWS_REGION }}"
      }
    }
  }
}
```

**Generated Output**:

```
Active CloudWatch Alarms:
1. High CPU Utilization Alert: Instance i-0123456789abcdef0 at 95% CPU
2. Memory Usage Alert: Instance i-0123456789abcdef0 using 90% memory
3. Critical Database Performance: DB mydb-instance at 1800 Read IOPS (threshold exceeded)
4. Elevated Cost: RDS instance cost increased 25% this month
5. Lambda Concurrent Executions: Function myFunction at 100 concurrent executions
6. Function Failure Alert: Lambda myFunction failed 15 times in last 5 minutes
7. S3 Storage Growth: Bucket mybucket grew 150GB this month
8. High S3 Request Rates: Bucket mybucket at 5000 req/sec (potential throttling)
9. Network Throughput Alert: Instance i-0123456789abcdef0 exceeded limits at 200 Mbps inbound
10. Critical Alert: EC2 instance i-0123456789abcdef0 high resource usage
```

### Example 2: Filesystem Mock Data

**Configuration**:

```json
{
  "mcpServers": {
    "filesystem-faker": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-filesystem,/tmp",
        "--ai-instruction", "Generate realistic filesystem data with varied file types, sizes, and timestamps"
      ]
    }
  }
}
```

**Generated Output**:

Instead of real `/tmp` contents, agent receives:

```
Files:
- report.pdf
- image.png
- notes.txt
- presentation.pptx
- script.py
- data.csv
- music.mp3
- readme.md
- index.html
- style.css
- video.mp4
- config.json

Directories:
- projects/
- archives/
- photos/
- logs/
```

## Command Reference

### Faker CLI

```bash
stn faker [flags]
```

**Flags:**

| Flag | Description | Required | Example |
|------|-------------|----------|---------|
| `--command` | Command to start target MCP server | Yes | `npx`, `uvx`, `docker` |
| `--args` | Arguments for target MCP server (comma-separated) | Yes | `-y,@modelcontextprotocol/server-filesystem,/tmp` |
| `--ai-instruction` | Natural language instruction for AI enrichment | No | `"Generate high-cost AWS monitoring data"` |
| `--debug` | Enable debug logging | No | - |

**Environment Variables:**

The faker uses Station's configured AI provider:

```bash
# Configure AI provider in Station config
stn up --provider openai --api-key sk-your-key
# or
stn up --provider gemini --api-key your-gemini-key
```

### Passthrough Mode

If `--ai-instruction` is not provided, faker operates in **passthrough mode** - all responses are forwarded unchanged from the target MCP server:

```bash
# No AI enrichment, just proxies the real server
stn faker --command npx --args "-y,@modelcontextprotocol/server-filesystem,/tmp"
```

## AI Instruction Best Practices

### Writing Effective Instructions

**Good instructions are:**
- Specific about the scenario (high-cost, low-usage, critical alerts, etc.)
- Include relevant metrics and thresholds
- Mention resource types and identifiers
- Describe the desired outcome

**Examples:**

**Generic (Less Effective)**:
```
"Generate some data"
```

**Specific (More Effective)**:
```
"Generate AWS CloudWatch data showing a production incident: high CPU (>90%),
memory pressure, database performance degradation, and elevated costs. Include
alarm states, timestamps, and specific resource IDs."
```

**Domain-Specific Examples:**

```bash
# FinOps - Cost Spike Scenario
--ai-instruction "Generate AWS cost data showing unexpected spike in EC2 and RDS costs. Include specific instance types, usage hours, and cost breakdown by service. Show 40% month-over-month increase with detailed attribution."

# Security - Vulnerability Scenario
--ai-instruction "Generate container scan results with critical CVEs in base image, outdated dependencies, and exposed secrets. Include CVE IDs, severity scores, and remediation recommendations."

# Infrastructure - Capacity Planning
--ai-instruction "Generate monitoring data showing infrastructure approaching capacity limits: 80%+ CPU utilization, memory pressure, disk space warnings, and network saturation on multiple hosts."

# Database - Performance Issues
--ai-instruction "Generate database metrics showing performance degradation: slow query times, connection pool exhaustion, lock contention, and elevated IOPS. Include specific query patterns and wait events."
```

## Advanced Configuration

### Multiple Faker Servers

You can run multiple faker proxies in the same environment for different MCP servers:

```json
{
  "mcpServers": {
    "aws-faker": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "uvx",
        "--args", "awslabs.cloudwatch-mcp-server@latest",
        "--ai-instruction", "High-cost production scenario"
      ],
      "env": {
        "AWS_ACCESS_KEY_ID": "{{ .AWS_ACCESS_KEY_ID }}",
        "AWS_SECRET_ACCESS_KEY": "{{ .AWS_SECRET_ACCESS_KEY }}",
        "AWS_REGION": "us-east-1"
      }
    },
    "database-faker": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "docker",
        "--args", "run,postgres-mcp-server",
        "--ai-instruction", "Database performance degradation scenario"
      ]
    },
    "real-filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
    }
  }
}
```

### Template Variables

Use template variables for credentials and configuration:

```json
{
  "mcpServers": {
    "aws-faker": {
      "env": {
        "AWS_ACCESS_KEY_ID": "{{ .AWS_ACCESS_KEY_ID }}",
        "AWS_SECRET_ACCESS_KEY": "{{ .AWS_SECRET_ACCESS_KEY }}",
        "AWS_REGION": "{{ .AWS_REGION }}"
      }
    }
  }
}
```

**Set variables**:

```bash
# In variables.yml
AWS_ACCESS_KEY_ID: "AKIA..."
AWS_SECRET_ACCESS_KEY: "secret..."
AWS_REGION: "us-east-1"
```

## Architecture & Implementation

### GenKit Integration

The faker uses Firebase GenKit for AI-powered data generation with structured output schemas:

```go
// Define output schema matching MCP Content structure
type ContentItem struct {
    Type string `json:"type"`
    Text string `json:"text"`
}
type OutputSchema struct {
    Content []ContentItem `json:"content"`
}

// Generate structured data with GenKit
output, _, err := genkit.GenerateData[OutputSchema](ctx, app,
    ai.WithPrompt(instruction),
    ai.WithModelName(modelName))
```

**Supported AI Providers:**
- OpenAI (GPT-4, GPT-4o, GPT-4o-mini)
- Google Gemini (gemini-2.0-flash-exp, gemini-pro)
- Any OpenAI-compatible endpoint (Ollama, Meta Llama, etc.)

### Proxy Architecture

```go
// Faker proxies MCP protocol
func (f *MCPFaker) handleToolCall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. Forward to real MCP server
    result, err := f.targetClient.CallTool(ctx, request)

    // 2. Enrich with AI-generated data
    enrichedResult, err := f.enrichToolResult(ctx, request.Params.Name, result)

    // 3. Return enriched response
    return enrichedResult, nil
}
```

The faker implements the full MCP protocol:
- Tool discovery (lists tools from target server)
- Tool execution (forwards calls to target)
- Response enrichment (AI-generated mock data)
- Error handling (graceful fallback to original data)

### Debug Logging

Enable debug logging to see enrichment process:

```bash
stn faker --debug ...
```

**Debug output shows:**
```
[FAKER] Initializing GenKit with provider: openai, model: gpt-4o-mini
[FAKER] Creating MCP client to target: npx ["-y" "@modelcontextprotocol/server-filesystem" "/tmp"]
[FAKER] Found 14 tools from target, registering...
[FAKER] Starting enrichment for tool: list_directory
[FAKER] Using instruction: Generate realistic filesystem data...
[FAKER] Calling GenKit with model: openai/gpt-4o-mini and output schema
[FAKER] GenKit structured response: {...}
[FAKER] Successfully enriched content with 17 items
```

## Testing & Development

### Unit Testing

Test enrichment logic in isolation:

```go
func TestEnrichmentLogic(t *testing.T) {
    // Initialize faker with test config
    f := &MCPFaker{
        genkitApp:     app,
        stationConfig: config,
        instruction:   "Generate realistic mock filesystem data",
        debug:         true,
    }

    // Create simple test input
    result := &mcp.CallToolResult{
        Content: []mcp.Content{
            mcp.NewTextContent("[DIR] folder1\n[FILE] test.txt"),
        },
    }

    // Test enrichment
    enriched, err := f.enrichToolResult(ctx, "list_directory", result)

    // Verify enriched data
    assert.Greater(t, len(enriched.Content), len(result.Content))
}
```

### E2E Testing

Test complete faker integration with agents:

```bash
# 1. Build and install with UI
make local-install-ui

# 2. Sync faker environment
stn sync faker-test

# 3. Run agent with faker proxy
stn agent run <agent-id> "Query the system"

# 4. Verify AI-generated mock data in response
```

## Troubleshooting

### Common Issues

**No enrichment occurring:**
- Check debug logs with `--debug` flag
- Verify AI provider is configured: `stn config get ai.provider`
- Ensure `--ai-instruction` is provided (otherwise passthrough mode)
- Check API key is valid and has quota

**Empty responses:**
- Original responses might be empty (real server returned no data)
- AI instruction might be too vague - make it more specific
- Check GenKit model supports structured output (GPT-4o-mini recommended)

**Incorrect data format:**
- Verify instruction describes desired schema clearly
- Check debug logs for GenKit response structure
- Consider adjusting instruction to match expected format

**Timeout errors:**
- AI generation might be slow for complex instructions
- Simplify instruction or reduce data volume
- Increase timeout in agent configuration

### Debug Commands

```bash
# Check faker is running
stn mcp list --env faker-test

# See discovered tools
stn sync faker-test --verbose

# Test with simple agent
stn agent run <agent-id> "Simple query" --tail

# View execution logs
stn runs inspect <run-id> -v
```

## Performance Considerations

### Cost Optimization

AI enrichment uses LLM API calls, which have costs:

**Recommendations:**
- Use `gpt-4o-mini` for development and testing (lowest cost)
- Use `gpt-4o` for production demos requiring high quality
- Consider Gemini Flash for cost-effective enrichment
- Cache results when possible for repeated queries
- Use passthrough mode when enrichment not needed

**Token Usage:**

Typical enrichment request:
- Input: ~100-500 tokens (instruction + original data)
- Output: ~200-1000 tokens (enriched mock data)
- Cost with GPT-4o-mini: ~$0.001-0.003 per request

### Response Time

AI enrichment adds latency:
- Real MCP call: 50-200ms
- AI enrichment: 1-5 seconds
- Total: ~2-6 seconds per tool call

**Optimization:**
- Use structured output schemas (faster than text parsing)
- Keep instructions concise
- Use faster models (gpt-4o-mini, gemini-flash)
- Consider parallel enrichment for multiple tools

## Use Case Gallery

### Demo Scenarios

**High-Cost Production Incident:**
```bash
--ai-instruction "Generate production incident data: multiple critical alarms,
high resource utilization across EC2/RDS/Lambda, cost spike of 60%, performance
degradation with slow response times and error rate increase. Include specific
timestamps within last 2 hours."
```

**Security Vulnerability Discovery:**
```bash
--ai-instruction "Generate security scan results showing critical vulnerabilities:
CVE-2024-XXXX in base image, exposed AWS credentials in environment variables,
SQL injection vulnerabilities in application code, and insecure container
configurations. Include severity scores and remediation steps."
```

**Cost Optimization Opportunity:**
```bash
--ai-instruction "Generate AWS cost data showing optimization opportunities:
underutilized EC2 instances running at <20% CPU, unused RDS snapshots,
unattached EBS volumes, and old S3 data eligible for glacier archival.
Include potential monthly savings."
```

### Development Workflows

**1. Build Agent Without Credentials:**

Develop and test AWS agents without real AWS credentials:

```json
{
  "mcpServers": {
    "aws-mock": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "uvx",
        "--args", "awslabs.cloudwatch-mcp-server@latest",
        "--ai-instruction", "Generate realistic AWS data for development"
      ],
      "env": {
        "AWS_ACCESS_KEY_ID": "fake-key",
        "AWS_SECRET_ACCESS_KEY": "fake-secret",
        "AWS_REGION": "us-east-1"
      }
    }
  }
}
```

**2. Test Agent Behavior:**

Verify agent correctly handles different scenarios:

```bash
# Test high-cost scenario
--ai-instruction "Generate data showing costs 50% over budget"

# Test low-usage scenario
--ai-instruction "Generate data showing minimal usage and low costs"

# Test mixed scenario
--ai-instruction "Generate data showing some services over budget, others under"
```

**3. Demo Agent Capabilities:**

Create compelling demos without production access:

```bash
--ai-instruction "Generate impressive demo data: multi-region deployment with
50+ EC2 instances, 10 RDS databases, 100+ Lambda functions, realistic CloudWatch
metrics showing healthy production workload with some optimization opportunities"
```

## FAQ

**Q: Does faker require production credentials?**
A: No! The faker can work with fake/invalid credentials since it generates mock data. The real MCP server might still require credentials to initialize, but responses are AI-generated.

**Q: Can I use faker in production?**
A: Faker is designed for development, testing, and demos. Do not use in production pipelines where real data is required for decisions.

**Q: Which MCP servers work with faker?**
A: Any MCP server! Faker proxies the MCP protocol, so it works with filesystem, AWS, database, security tools, or custom MCP servers.

**Q: How does faker handle tool schemas?**
A: Faker discovers tools from the target MCP server and exposes them with identical schemas. The enrichment happens on responses, not tool definitions.

**Q: Can I customize the enrichment logic?**
A: Currently enrichment uses GenKit with natural language instructions. For custom logic, modify the `enrichToolResult` function in `pkg/faker/mcp_faker.go`.

**Q: Does faker cache AI responses?**
A: No, each tool call generates fresh mock data. Caching could be added as a future enhancement.

## Safety Mode: Write Operation Protection

The faker includes automatic write operation detection and interception to prevent accidental data modification during development and testing.

### How Safety Mode Works

**1. AI-Powered Tool Classification:**
When the faker starts, it uses AI to analyze each tool from the target MCP server:

```
[FAKER] Classifying tools for write operation detection...
[FAKER] Tool write_file: write=true, risk=high, reason=Creates or overwrites files
[FAKER] Tool edit_file: write=true, risk=moderate, reason=Modifies file content
[FAKER] Tool create_directory: write=true, risk=medium, reason=Creates directories
[FAKER] Tool list_directory: write=false, risk=low, reason=Read-only listing
```

**2. User Notification:**
The faker displays which operations will be intercepted:

```
🛡️  SAFETY MODE: 4 write operations detected and will be INTERCEPTED:
  1. move_file
  2. write_file
  3. edit_file
  4. create_directory
These tools will return mock success responses without executing real operations.
```

**3. Automatic Interception:**
When an agent tries to call a write operation:
- Faker intercepts the call **before** it reaches the target server
- Returns a mock success response to the agent
- Real data remains completely untouched

### Example: Protected Write Operation

**Agent Request:**
```
Create a directory called test-data and write a file config.json
```

**Faker Response:**
```
✅ MOCK SUCCESS (Write operation intercepted by faker)

Tool: create_directory
Arguments: {
  "path": "/tmp/test-data"
}

Operation Status: Simulated Success
Message: This write operation was safely intercepted and did not execute on the real target.
         The faker is in safety mode and returns mock success responses for all write operations.
```

**Result:**
- Agent believes operation succeeded
- Real filesystem unchanged
- No actual directory created

### Classification Examples

The AI classifies tools based on semantic understanding:

**Write Operations (Intercepted):**
- `create`, `update`, `delete`, `modify` operations
- `write`, `edit`, `move`, `remove` operations
- `execute` (for command execution)
- `deploy`, `start`, `stop`, `cancel` operations

**Read Operations (Proxied):**
- `get`, `list`, `describe`, `fetch` operations
- `read`, `query`, `search`, `analyze` operations
- `show`, `view`, `inspect` operations

### Benefits

✅ **No Accidental Writes**: Impossible to modify production data during testing
✅ **Transparent**: See exactly what's protected before running agents
✅ **Intelligent**: AI understands tool semantics, not just names
✅ **Zero Config**: Safety mode always enabled by default
✅ **Risk Assessment**: Each tool rated by risk level for awareness

### Safety Mode in Action

**Test Scenario:**
```bash
# Agent with filesystem faker
stn agent run filesystem-test "List /tmp, create directory test-faker, write file test.txt"
```

**Results:**
- ✅ List operation: Returns AI-enriched mock filesystem data
- ⚠️ Create directory: Intercepted, mock success returned
- ⚠️ Write file: Intercepted, mock success returned
- ✅ Real `/tmp` filesystem: Completely unchanged

**Verification:**
```bash
ls /tmp/test-faker
# Output: No such file or directory - write was intercepted!
```

## Resources

- [Architecture Documentation](./architecture.md)
- [MCP Tools Guide](./mcp-tools.md)
- [Agent Development](./agent-development.md)
- [GenKit Documentation](https://firebase.google.com/docs/genkit)
- [MCP Protocol Specification](https://modelcontextprotocol.io)

## Contributing

The AI Faker is part of Station's open-source codebase. Contributions welcome:

- **Code**: `pkg/faker/` directory
- **Tests**: `pkg/faker/*_test.go`
- **Issues**: Report bugs or request features on GitHub

Key areas for contribution:
- Response caching for performance
- Additional AI providers
- Custom enrichment strategies
- Multi-modal data generation (images, files)
- Streaming support for large responses
