![](./image.png)

# Station - Self-Hosted MCP Agent Runtime

**Station** is a lightweight, self-hosted runtime for executing background AI agents within your infrastructure. Deploy a single 40MB binary to run scheduled Claude/LLM agents that can access your internal tools, APIs, and environments without exposing credentials to third-party services.

## Problem

Engineering teams want to leverage AI agents for operational tasks (monitoring, automation, alerting) but face a critical blocker: there's no secure way to give AI agents access to production infrastructure without handing over the keys to external services.

## Solution

Station acts as a secure bridge between your AI (Claude, GPT, etc.) and your infrastructure. You deploy Station inside your network, configure it with your tools and credentials, then use natural language to create agents that run on your schedule with your permissions.

## How It Works

```
Your LLM (Claude) → MCP Protocol → Station (in your infra) → Your Tools
                                         ↓
                                  Background Agents
                                  (scheduled tasks)
```

1. **Deploy Station** in your infrastructure (single binary, 40MB)
2. **Load MCP tools** for your environments (AWS, GitHub, Slack, etc.)
3. **Connect Claude** to Station via MCP
4. **Describe agents** in natural language - Claude creates them automatically
5. **Agents run** on schedule with access to your configured tools

## Quick Start

### 1. Install Station (30 seconds)
```bash
# Download and install
curl -sSL https://getstation.cloudshipai.com | bash

# Initialize with encryption keys
stn init
```

### 2. Load Your Tools (2 minutes)
```bash
# Load AWS tools for three environments
stn env create dev
stn load https://github.com/awslabs/mcp-server-aws --env dev

stn env create staging  
stn load https://github.com/awslabs/mcp-server-aws --env staging

stn env create prod
stn load https://github.com/awslabs/mcp-server-aws --env prod
```

### 3. Create an Agent (1 minute)
```bash
# Start Station
stn serve

# Connect Claude to localhost:3000 (Station's MCP endpoint)
# Ask Claude: "Create an agent that checks CloudWatch logs every 6 hours 
# across all environments for errors and posts summaries to Slack"
```

Station automatically:
- Parses your request
- Selects appropriate tools (CloudWatch logs reader, Slack poster)  
- Sets up the schedule
- Configures environment access
- Deploys the agent

## Key Features

### 🔐 Security First
- **Zero credential exposure**: All secrets stay in your infrastructure
- **Encrypted storage**: MCP configs encrypted at rest with NaCl
- **Fine-grained permissions**: Control exactly which sub-tools agents can access
- **Environment isolation**: Separate configs for dev/staging/prod

### 🎯 Practical Design
- **Single binary**: 40MB, no dependencies, runs anywhere
- **Natural language**: Describe agents to Claude, it handles the rest
- **Tool discovery**: Analyzes GitHub repos to auto-configure MCP servers
- **Webhook notifications**: Real-time notifications when agents complete tasks
- **Observability**: OpenTelemetry compatible for standard monitoring

### 🚀 Real Use Cases

**Multi-Environment Monitoring**
```
"Every 6 hours, check Grafana dashboards for anomalies. If found, 
analyze CloudWatch logs and EC2 metrics across dev/staging/prod 
to identify the root cause and create a GitHub issue."
```
*→ Webhook notifications alert your team in Slack when issues are detected*

**Resource Optimization**
```
"Every 10 minutes, run Ansible commands on dev clusters to check 
resource usage. Scale down idle instances automatically."
```
*→ Webhooks trigger cost reporting dashboards when optimizations occur*

**Deployment Verification**  
```
"After each staging deployment, run test suites and check error rates.
If issues detected, rollback and notify the team on Slack."
```
*→ Webhook integration with your CI/CD pipeline for automated responses*

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Claude    │────▶│   Station    │────▶│   Your Tools    │
│ (MCP Client)│ MCP │  (Runtime)   │     │ AWS, GH, Slack  │
└─────────────┘     └──────────────┘     └─────────────────┘
                           │
                    ┌──────▼───────┐
                    │ Agent Queue  │ 
                    │ (Scheduled)  │
                    └──────────────┘
```

## Advanced Configuration

### Environment Management
```bash
# Create isolated environments
stn env create production --encrypted

# Load multiple configs per environment
stn load aws-config.json --env production
stn load github-config.json --env production
stn load pagerduty-config.json --env production
```

### Team Access
```bash
# Enable team mode
stn serve --remote --host 0.0.0.0

# Create team members
stn user create alice --role admin
stn user create bob --role user

# Team members connect with API keys
export STATION_API_KEY=xxx
stn agent list --endpoint https://station.internal
```

### Tool Permissions
```yaml
# Fine-grained tool control in agent config
agent:
  name: "CloudWatch Monitor"
  tools:
    - server: "aws-mcp"
      environment: "production"
      allowed_tools: ["logs:GetLogEvents", "cloudwatch:GetMetricData"]
      denied_tools: ["ec2:TerminateInstances"]
```

### Webhook Notifications

Get real-time notifications when your agents complete tasks. Perfect for integrating Station with your existing alerting and monitoring systems.

```bash
# Enable webhook notifications
stn settings set notifications_enabled true

# Create a webhook for Slack notifications
stn webhook create --name "Slack Alerts" \
  --url "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK" \
  --events "agent_run_completed" \
  --secret "your-webhook-secret"

# List and manage webhooks
stn webhook list
stn webhook show <webhook-id>
stn webhook deliveries  # View delivery history
```

**Webhook Payload Example:**
```json
{
  "event": "agent_run_completed",
  "timestamp": "2024-01-15T10:30:45Z",
  "agent": {
    "id": 1,
    "name": "CloudWatch Monitor",
    "environment_id": 1
  },
  "run": {
    "id": 123,
    "task": "Check error rates",
    "final_response": "No critical errors found",
    "status": "completed",
    "steps_taken": 3
  }
}
```

**Integrations:** Works with Slack, Discord, Microsoft Teams, PagerDuty, custom endpoints, and any HTTP webhook receiver.

**Security:** All webhooks include HMAC-SHA256 signatures for payload verification and support custom headers for authentication.

## Installation Options

### Quick Install (Recommended)
```bash
curl -sSL https://getstation.cloudshipai.com | bash
```

### Docker
```bash
docker run -d \
  -v ~/.config/station:/config \
  -p 3000:3000 \
  -p 2222:2222 \
  ghcr.io/cloudshipai/station:latest
```

### From Source
```bash
git clone https://github.com/cloudshipai/station
cd station
go build -o stn cmd/stn/main.go
```

## Why Station?

**For Security Teams**: Agents run inside your network with your IAM roles. No external service ever sees your credentials.

**For DevOps**: Deploy once, create unlimited agents. Observability built-in. Single binary means trivial deployment.

**For Developers**: Natural language agent creation. No complex frameworks. Just describe what you want.

## Common Patterns

### Multi-Environment Sync
```bash
# Load same tool across environments with different credentials
for env in dev stage prod; do
  stn load github-mcp.json --env $env
done
```

### Monitoring Pipeline
```bash
# Chain multiple tools for complex workflows
"Monitor Datadog → Analyze with AWS → Create Jira ticket → Notify Slack"
```

### Scheduled Reports
```bash
# Daily/weekly operational intelligence
"Every Monday at 9am, gather metrics from all environments and 
create an executive summary with trends and recommendations"
```

### Webhook Integration Patterns
```bash
# Connect Station to your existing tools
stn webhook create --name "PagerDuty" --url "https://events.pagerduty.com/integration/xxx/enqueue"
stn webhook create --name "DataDog" --url "https://webhooks.datadoghq.com/v1/webhooks/xxx"
stn webhook create --name "Custom Dashboard" --url "https://dashboard.company.com/api/station-events"

# Chain webhooks for complex workflows
# Agent completes → Webhook → Triggers deployment → Another agent validates
```

## Troubleshooting

**Agent not running?**
```bash
stn agent status my-agent
stn agent logs my-agent --tail 50
```

**Tool permission denied?**
```bash
# Check agent's allowed tools
stn agent inspect my-agent | jq .tools
```

**Can't connect Claude?**
```bash
# Verify Station is serving MCP
curl http://localhost:3000/mcp/describe
```

**Webhooks not firing?**
```bash
# Check notification settings and webhook status
stn settings get notifications_enabled
stn webhook list
stn webhook deliveries --limit 10
```

## CloudshipAI Integration

Station integrates with [CloudshipAI](https://cloudship.ai) for fleet management:

```bash
# Connect Station to Cloudship fleet
stn cloudship connect --fleet-id xxx

# Cloudship can now orchestrate agents across all your Stations
# Perfect for multi-region, multi-cloud, edge deployments
```

## Requirements

- Linux, macOS, or Windows
- 256MB RAM
- 100MB disk space
- Network access to your tools/APIs

## License

AGPL-3.0 - See [LICENSE](LICENSE) for details.

## Contributing

Station is open source. We welcome contributions:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing`)
5. Open a Pull Request

---

**Station** - Run AI agents where they're needed, not where they're allowed.

Built by engineers who were tired of choosing between AI capabilities and infrastructure security.
