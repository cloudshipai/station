# Station Faker Architecture

## Overview
Station Faker is a **universal MCP tool wrapper** that can simulate ANY MCP server behavior with AI-generated realistic data. It's NOT limited to filesystem operations - it can wrap and simulate any type of MCP tool.

## Core Concept
Faker acts as an intelligent proxy between Station agents and real MCP servers, intercepting tool calls and generating realistic simulated responses based on AI instructions.

## Universal Simulation Capabilities

### What Faker Can Simulate
Faker can wrap ANY MCP tool and simulate diverse scenarios:

#### Cloud Infrastructure (AWS, Azure, GCP)
- **High Traffic Environments**: Simulate peak load with realistic metrics
- **Cost Spikes**: Generate anomalous billing data for testing cost alerts
- **Security Incidents**: Simulate suspicious CloudTrail events
- **Performance Issues**: Generate slow response times and timeout scenarios
- **Quota Exhaustion**: Simulate resource limit warnings

#### Monitoring & Observability (Grafana, Prometheus)
- **Alert Storms**: Generate multiple correlated alerts
- **Metric Anomalies**: Simulate sudden CPU/memory spikes
- **Dashboard Failures**: Test agent handling of missing data
- **Time-series Patterns**: Generate realistic seasonal/trending data

#### Databases (MySQL, PostgreSQL)
- **High Query Load**: Simulate database under stress
- **Slow Queries**: Generate performance degradation scenarios
- **Connection Exhaustion**: Test connection pool limits
- **Replication Lag**: Simulate sync delays

#### Payment Systems (Stripe, Payment Gateways)
- **Transaction Spikes**: Simulate Black Friday traffic
- **Failed Payments**: Test retry and reconciliation logic
- **Fraud Patterns**: Generate suspicious transaction sequences
- **Refund Waves**: Simulate mass refund scenarios

#### Version Control (GitHub, GitLab)
- **Large PRs**: Simulate massive code review scenarios
- **CI/CD Failures**: Generate build/test failure patterns
- **Security Vulnerabilities**: Simulate Dependabot alerts
- **Repository Activity**: Generate realistic commit patterns

## Architecture Components

### 1. Faker Proxy Server
```
Agent → Station → Faker Proxy → AI Model → Simulated Response
                      ↓
                 (Optional) Real MCP Server (for schema/validation)
```

### 2. AI Instruction System
Each faker instance is configured with custom AI instructions that define:
- **Scenario Context**: What situation to simulate (e.g., "AWS production under high load")
- **Data Patterns**: What kind of data to generate (e.g., "cost spike in us-east-1")
- **Anomalies**: What unusual conditions to include (e.g., "unexpected EC2 instance types")
- **Realism Requirements**: How authentic the data should be (e.g., "valid AWS ARNs")

### 3. Session Management
Faker maintains session state to ensure:
- **Consistency**: Related tool calls return coherent data
- **Temporal Coherence**: Time-based queries show realistic progression
- **Relationship Integrity**: Related resources reference each other correctly

### 4. OpenTelemetry Instrumentation
All faker operations emit spans with:
- **Label**: `station.faker` (distinguished from real tool calls)
- **Attributes**: 
  - `faker.tool_name`: Original MCP tool being simulated
  - `faker.ai_instruction`: Brief description of simulation scenario
  - `faker.session_id`: Session identifier for replay
  - `faker.response_time`: Simulated latency
  - `faker.real_mcp_used`: Whether real MCP was consulted

### 5. Session Replay
Faker records all tool calls and responses for:
- **Debugging**: Understand what data agents received
- **Testing**: Replay scenarios to test agent improvements
- **Analysis**: Study agent behavior under specific conditions
- **Sharing**: Distribute realistic test scenarios as bundles

## Creating a Faker Instance

### From Existing MCP Server
```bash
# Convert filesystem MCP to faker
stn faker create-from-mcp-server \
  --environment cicd-demo \
  --mcp-server-name filesystem \
  --faker-name filesystem-faker \
  --ai-instruction "Simulate a large production codebase with 50,000+ files, slow disk I/O, and occasional permission errors"

# Convert AWS MCP to faker  
stn faker create-from-mcp-server \
  --environment aws-ops \
  --mcp-server-name aws-billing \
  --faker-name aws-billing-faker \
  --ai-instruction "Simulate AWS production account with $50k/month baseline spend, sudden 3x cost spike in us-east-1 EC2, and unusual RDS instance types appearing"
```

### Standalone Faker (Offline)
```bash
# Create faker without real MCP backend
stn faker create-from-mcp-server \
  --environment security-test \
  --mcp-server-name github \
  --faker-name github-faker \
  --ai-instruction "Simulate a monorepo with 500+ contributors, 20 daily PRs, and 5 critical Dependabot security alerts" \
  --offline
```

## AI Instruction Examples

### Cloud Cost Anomaly Detection
```
Simulate an AWS production environment with:
- Baseline monthly cost: $75,000
- Sudden cost spike: +250% over 6 hours
- Root cause: Misconfigured autoscaling group launching 200+ r6i.8xlarge instances
- Secondary anomaly: DynamoDB read capacity spike (possible attack)
- Include realistic CloudTrail events showing who made the config change
```

### High-Traffic E-commerce Platform
```
Simulate a Stripe payment system during Black Friday:
- Normal: 50 transactions/minute
- Peak: 2,000 transactions/minute  
- Include: 3% failed payments (expired cards, insufficient funds)
- Generate: Realistic fraud patterns (10 rapid small-value test charges)
- Webhooks: Simulate 100ms-5s delivery delays under load
```

### Database Performance Crisis
```
Simulate a PostgreSQL database experiencing performance degradation:
- Baseline: 50ms average query time
- Current: 5,000ms average query time
- Root cause: Missing index on frequently queried table
- Symptoms: Connection pool exhaustion, query timeout errors
- Include realistic pg_stat_activity output showing blocked queries
```

## Integration with Agents

### Agent Bundle Creation
Agents designed to work with faker-wrapped tools can be packaged as bundles:

```yaml
# bundles/aws-cost-anomaly-detector/template.json
{
  "name": "aws-cost-anomaly-detector",
  "mcpServers": {
    "aws-billing-faker": {
      "command": "station-faker",
      "args": ["--mcp-server", "aws-billing", 
               "--ai-instruction", "{{ .FAKER_SCENARIO }}"]
    }
  }
}

# bundles/aws-cost-anomaly-detector/variables.yml
FAKER_SCENARIO: "Simulate realistic AWS cost spike scenario"
```

### Testing with Faker
```bash
# Test agent with simulated high-cost scenario
stn agent run "AWS Cost Analyzer" \
  "Analyze the current AWS spending and identify any cost anomalies" \
  --env aws-cost-test-faker

# Review faker session to see what data the agent received
stn faker session replay <session-id>
```

## Session Replay Architecture

### Recording
Every faker tool call is recorded with:
- **Timestamp**: When the call was made
- **Tool Name**: Which MCP tool was invoked
- **Parameters**: What arguments were passed
- **Response**: What simulated data was returned
- **Context**: AI instruction and session state

### Replay
Sessions can be replayed to:
1. **Debug Agent Behavior**: See exactly what data the agent saw
2. **Test Agent Changes**: Re-run the agent with the same simulated environment
3. **Share Scenarios**: Export faker sessions as test cases
4. **Training**: Use faker sessions to improve agent prompts

### Storage Format
```json
{
  "session_id": "faker-session-20250109-123456",
  "faker_name": "aws-billing-faker",
  "ai_instruction": "Simulate cost spike...",
  "started_at": "2025-01-09T12:34:56Z",
  "tool_calls": [
    {
      "seq": 1,
      "timestamp": "2025-01-09T12:35:01Z",
      "tool": "aws_get_cost_and_usage",
      "parameters": {"granularity": "DAILY", "start": "2025-01-01"},
      "response": {
        "ResultsByTime": [...],
        "faker_metadata": {
          "generation_time_ms": 250,
          "anomaly_injected": true
        }
      }
    }
  ]
}
```

## Default Faker Bundles

Station ships with pre-configured faker bundles for common scenarios:

### Cloud Operations
- `aws-production-faker-bundle`: High-scale AWS environment simulator
- `azure-devops-faker-bundle`: Azure DevOps CI/CD simulator
- `gcp-cost-faker-bundle`: GCP billing and cost simulator

### Monitoring & Observability  
- `grafana-incident-faker-bundle`: Incident response simulator
- `prometheus-alerts-faker-bundle`: Alert storm simulator

### Development & CI/CD
- `github-monorepo-faker-bundle`: Large repository simulator
- `terraform-infra-faker-bundle`: Infrastructure as Code simulator

### E-commerce & Payments
- `stripe-payments-faker-bundle`: Payment processing simulator
- `high-traffic-api-faker-bundle`: API under load simulator

## Best Practices

### 1. Realistic AI Instructions
- Be specific about scale and patterns
- Include both normal and anomalous behavior
- Reference actual resource types/names
- Specify realistic timelines and magnitudes

### 2. Session Management
- Use meaningful session IDs for tracking
- Record all faker sessions for debugging
- Clean up old sessions periodically

### 3. Performance Tuning
- Adjust simulated latency to match real systems
- Cache AI-generated data for repeated queries
- Use offline mode for deterministic testing

### 4. Testing Strategy
- Test agents with faker first before real systems
- Create faker bundles for regression testing
- Use faker for CI/CD integration tests
- Share faker sessions as reproducible test cases

## Future Enhancements
- [ ] Faker scenario library with pre-built simulations
- [ ] Faker replay UI for visual session analysis
- [ ] Multi-faker orchestration for complex scenarios
- [ ] Faker telemetry dashboard showing simulation quality
- [ ] Faker session diffing for comparing agent behavior
- [ ] Faker SDK for custom simulation logic
