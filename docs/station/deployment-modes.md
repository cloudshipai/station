# Deployment Modes

Station supports multiple deployment modes to fit different workflows and environments. Choose the mode that best matches your use case.

## Overview

| Mode | Use Case | Interface | MCP Access | Web UI |
|------|----------|-----------|------------|--------|
| **`stn up`** | Local development | Web UI + API + MCP | Yes | Yes (8585) |
| **`stn stdio`** | Claude Desktop/Cursor | Stdio protocol | Yes | No |
| **`stn serve`** | Production API | HTTP API only | Yes | Optional |
| **Docker** | Containerized deployment | All modes | Yes | Optional |

## Server Mode (`stn up`)

Full-featured mode with web UI, API, and MCP server integration.

### When to Use

- Local agent development and testing
- Adding MCP tools via UI
- Creating and managing agents
- Bundling agents for distribution
- Debugging agent executions

### Starting Server Mode

```bash
# Start with default provider (OpenAI)
export OPENAI_API_KEY=sk-...
stn up

# Start with specific provider
stn up --provider anthropic --api-key sk-ant-...
stn up --provider gemini --api-key <key> --model gemini-2.5-flash
stn up --provider custom --base-url http://localhost:11434/v1 --model llama3-groq-tool-use

# Start with CloudShip registration
stn up --cloudshipai-registration-key <key>
```

### What You Get

**Web UI** (`http://localhost:8585`):
- Browse and add MCP tools
- Create environments
- Manage agent bundles
- Build Docker containers
- View execution history

**MCP Server** (`http://localhost:8586/mcp`):
- Automatically configured in `.mcp.json` for Claude Code/Cursor
- Exposes Station's management tools
- Access to all environment MCP tools

**Dynamic Agent MCP** (`http://localhost:3030/mcp`):
- Each agent becomes an MCP tool
- Call agents from Claude Desktop/Cursor
- Real-time agent execution

### Stopping Server Mode

```bash
stn down
```

This gracefully shuts down:
- Web UI server
- MCP servers
- Database connections
- Background processes

## Stdio Mode (`stn stdio`)

Stdio protocol mode for direct integration with Claude Desktop and Cursor.

### When to Use

- Using Station agents from Claude Desktop
- Cursor IDE integration
- Building custom MCP clients
- Programmatic agent execution via stdio

### Configuration

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"],
      "env": {
        "OPENAI_API_KEY": "sk-..."
      }
    }
  }
}
```

**Cursor** (`.cursorrules` or settings):
```json
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"],
      "env": {
        "OPENAI_API_KEY": "sk-..."
      }
    }
  }
}
```

### How It Works

```
Claude Desktop starts Station with stdio
  ↓
Station loads default environment
  ↓
Discovers agents and MCP tools
  ↓
Exposes as MCP tools via stdio
  ↓
Claude can call Station tools and agents
```

### Available Tools in Stdio Mode

Station exposes management tools via stdio:
- `call_agent` - Execute an agent with a task
- `list_agents` - View available agents
- `create_agent` - Create new agents
- `list_environments` - View environments
- `list_tools` - See available MCP tools

Example interaction:
```
User (in Claude Desktop): "Use Station to analyze my AWS costs"
Claude: [Calls call_agent tool with cost analysis task]
Station: [Executes agent, returns results]
Claude: [Presents findings to user]
```

## Standalone Binary Mode

Station is a single executable with no external dependencies (except for MCP servers you want to use).

### When to Use

- Quick agent execution in scripts
- CI/CD pipelines
- Scheduled jobs (cron, GitHub Actions)
- Serverless functions
- No persistent server needed

### Command-Line Agent Execution

```bash
# Run agent by name
stn agent run "Cost Analyzer" "analyze EC2 costs in us-east-1"

# Run agent by ID
stn agent run 7 "analyze costs" --env prod

# Stream output
stn agent run "Security Scanner" "scan terraform files" --tail

# Save output to file
stn agent run "Report Generator" "generate monthly report" > report.txt
```

### Scripting Example

```bash
#!/bin/bash
# Daily cost analysis script

export OPENAI_API_KEY=sk-...

# Run cost analyzer
RESULT=$(stn agent run "AWS Cost Analyzer" \
  "Analyze yesterday's EC2 costs and identify anomalies" \
  --env prod --format json)

# Parse results
ANOMALIES=$(echo $RESULT | jq '.anomalies | length')

# Alert if anomalies found
if [ $ANOMALIES -gt 0 ]; then
  echo "Cost anomalies detected!"
  # Send alert via Slack, email, etc.
fi
```

### CI/CD Integration

**GitHub Actions**:
```yaml
name: Security Scan
on: [push]

jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Station
        run: curl -fsSL https://install.station.dev | bash

      - name: Run Security Scanner
        run: |
          stn agent run "Infrastructure Security Scanner" \
            "Scan terraform files for security issues" \
            --env cicd
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

## Production Server Mode (`stn serve`)

HTTP API mode for production deployments.

### When to Use

- Production API deployments
- Kubernetes/ECS deployments
- Multi-tenant environments
- High-availability setups
- API-first workflows

### Starting Server Mode

```bash
# Start API server
stn serve --port 8080

# With specific environment
stn serve --env prod --port 8080

# With TLS
stn serve --tls-cert cert.pem --tls-key key.pem
```

### API Endpoints

**Agent Execution**:
```bash
# Execute agent
curl -X POST http://localhost:8080/api/v1/agents/:id/execute \
  -H "Content-Type: application/json" \
  -d '{"task": "analyze costs", "environment": "prod"}'

# Stream execution
curl -X POST http://localhost:8080/api/v1/agents/:id/execute?stream=true \
  -H "Content-Type: application/json" \
  -d '{"task": "analyze costs"}'
```

**Agent Management**:
```bash
# List agents
curl http://localhost:8080/api/v1/agents

# Get agent details
curl http://localhost:8080/api/v1/agents/:id

# Create agent
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Content-Type: application/json" \
  -d @agent-definition.json
```

**Execution History**:
```bash
# List runs
curl http://localhost:8080/api/v1/runs

# Get run details
curl http://localhost:8080/api/v1/runs/:id

# Get run logs
curl http://localhost:8080/api/v1/runs/:id/logs
```

### Production Deployment Example

**Docker Compose**:
```yaml
version: '3.8'
services:
  station:
    image: station:latest
    command: ["serve", "--port", "8080"]
    ports:
      - "8080:8080"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - STATION_ENV=prod
    volumes:
      - ./environments:/app/environments
      - ./data:/app/data
```

**Kubernetes Deployment**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station
spec:
  replicas: 3
  selector:
    matchLabels:
      app: station
  template:
    metadata:
      labels:
        app: station
    spec:
      containers:
      - name: station
        image: station:latest
        args: ["serve", "--port", "8080"]
        ports:
        - containerPort: 8080
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        volumeMounts:
        - name: config
          mountPath: /app/environments
      volumes:
      - name: config
        configMap:
          name: station-config
```

## Docker Mode

Containerized Station for consistent deployments.

### When to Use

- Zero-config deployments
- Cloud environments (AWS ECS, GCP Cloud Run)
- Reproducible environments
- Multi-platform support

### Building Docker Image

```bash
# Build image from environment
stn build env prod

# This creates a Docker image with:
# - Station binary
# - Environment configuration
# - Agent definitions
# - MCP server configs
```

### Running Docker Container

```bash
# Run with default settings
docker run -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  station:prod

# Run with volume mounts
docker run -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  -v $(pwd)/environments:/app/environments \
  -v $(pwd)/data:/app/data \
  station:prod

# Run in stdio mode (for Claude Desktop in Docker)
docker run -i \
  -e OPENAI_API_KEY=sk-... \
  station:prod stdio
```

### Zero-Config AWS Deployment

Station automatically discovers AWS credentials in cloud environments:

```yaml
# ECS Task Definition
{
  "taskDefinition": {
    "containerDefinitions": [{
      "name": "station",
      "image": "station:prod",
      "command": ["serve", "--port", "8080"],
      "portMappings": [{
        "containerPort": 8080,
        "protocol": "tcp"
      }],
      "environment": [
        {"name": "STATION_ENV", "value": "prod"}
      ]
    }],
    "taskRoleArn": "arn:aws:iam::ACCOUNT:role/StationTaskRole"
  }
}
```

Station will automatically use the ECS task IAM role for AWS API access (no credentials needed in config).

## Mode Comparison

### Feature Matrix

| Feature | `stn up` | `stn stdio` | `stn serve` | Standalone |
|---------|----------|-------------|-------------|------------|
| Web UI | ✅ | ❌ | Optional | ❌ |
| MCP Server | ✅ | ✅ | ✅ | ❌ |
| API Access | ✅ | ❌ | ✅ | ❌ |
| Claude Integration | ✅ | ✅ | ✅ | ❌ |
| CLI Execution | ✅ | ✅ | ✅ | ✅ |
| Streaming Output | ✅ | ✅ | ✅ | ✅ |
| Persistent Process | ✅ | ❌ | ✅ | ❌ |
| Production Ready | ⚠️  | ❌ | ✅ | ✅ |

### Performance Characteristics

**`stn up` (Development)**:
- Memory: ~100-200MB
- Startup: ~2-3 seconds
- Latency: Low (local)
- Concurrent agents: 10-20

**`stn serve` (Production)**:
- Memory: ~50-100MB per instance
- Startup: ~1-2 seconds
- Latency: Low (optimized)
- Concurrent agents: 50-100 per instance

**Standalone Binary**:
- Memory: ~30-50MB per execution
- Startup: ~500ms-1s
- Latency: Lowest (no overhead)
- Concurrent agents: Limited by system

## Choosing the Right Mode

### Development Workflow
1. **Start with `stn up`**: Add MCP tools, create agents via UI
2. **Use Claude Code/Cursor**: Create and test agents interactively
3. **Test with CLI**: Run agents directly for debugging
4. **Package**: Create bundles for distribution

### Production Workflow
1. **Deploy with `stn serve`**: API-first for production workloads
2. **Or use standalone**: For scheduled/scripted executions
3. **Monitor**: Track runs via API or database
4. **Scale**: Add instances behind load balancer

### Integration Workflow
1. **`stn stdio` for Claude Desktop**: Direct integration
2. **API for custom clients**: Use `stn serve` + HTTP API
3. **CLI for scripts**: Use standalone binary in automation

## Next Steps

- [Architecture](./architecture.md) - Understand how Station works
- [Agent Development](./agent-development.md) - Create your first agent
- [Zero-Config Deployments](./zero-config-deployments.md) - Production deployment guide
- [Examples](./examples.md) - Real-world deployment examples
