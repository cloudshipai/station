# Station Configuration Reference

This document provides a complete reference of all configuration options available in Station's `config.yaml` file and their corresponding environment variables.

## Configuration Precedence

Configuration values are loaded in the following order (later values override earlier):

1. Default values
2. `config.yaml` file
3. Environment variables

## Quick Start

Create a `config.yaml` in your workspace:

```yaml
# Minimal configuration
ai_provider: openai
ai_api_key: ${OPENAI_API_KEY}
ai_model: gpt-4o

# Optional: Enable sandbox
sandbox:
  enabled: true
```

Or use environment variables:

```bash
export STN_AI_PROVIDER=openai
export STN_AI_API_KEY=sk-...
export STN_AI_MODEL=gpt-4o
```

---

## AI Provider Settings

Configure the AI provider for agent execution.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `ai_provider` | string | `openai` | AI provider: `openai`, `anthropic`, `ollama`, `gemini` |
| `ai_model` | string | `gpt-4o` | Model name (e.g., `gpt-4o`, `claude-sonnet-4-20250514`) |
| `ai_api_key` | string | - | API key for the AI provider |
| `ai_base_url` | string | - | Base URL for OpenAI-compatible endpoints |

### Environment Variables

| Variable | Maps To |
|----------|---------|
| `STN_AI_PROVIDER`, `OPENAI_API_KEY` | `ai_provider` |
| `STN_AI_MODEL` | `ai_model` |
| `STN_AI_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` | `ai_api_key` |
| `STN_AI_BASE_URL`, `OPENAI_BASE_URL` | `ai_base_url` |

### Example

```yaml
ai_provider: anthropic
ai_api_key: ${ANTHROPIC_API_KEY}
ai_model: claude-sonnet-4-20250514
```

---

## Coding Backend Settings

Configure the AI coding backend for development tasks.

### Common Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `coding.backend` | string | `opencode-cli` | Backend: `opencode`, `opencode-nats`, `opencode-cli`, `claudecode` |
| `coding.workspace_base_path` | string | `/tmp/station-coding` | Base path for coding workspaces |
| `coding.max_attempts` | int | `3` | Maximum retry attempts |
| `coding.task_timeout_min` | int | `30` | Task timeout in minutes |

### OpenCode HTTP Backend (`opencode`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `coding.opencode.url` | string | `http://localhost:4096` | OpenCode HTTP server URL |

### OpenCode NATS Backend (`opencode-nats`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `coding.nats.url` | string | `nats://127.0.0.1:4222` | NATS server URL |
| `coding.nats.subjects.task` | string | `station.coding.task` | Subject for task requests |
| `coding.nats.subjects.result` | string | `station.coding.result` | Subject for results |
| `coding.nats.subjects.stream` | string | `station.coding.stream` | Subject for streaming |

### OpenCode CLI Backend (`opencode-cli`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `coding.cli.binary_path` | string | `opencode` | Path to opencode binary |
| `coding.cli.timeout_sec` | int | `300` | CLI command timeout |

### Claude Code Backend (`claudecode`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `coding.claudecode.binary_path` | string | `claude` | Path to claude binary |
| `coding.claudecode.timeout_sec` | int | `300` | Command timeout |
| `coding.claudecode.model` | string | - | Model: `sonnet`, `opus`, `haiku` |
| `coding.claudecode.max_turns` | int | `10` | Max conversation turns |
| `coding.claudecode.allowed_tools` | []string | - | Tools whitelist |
| `coding.claudecode.disallowed_tools` | []string | - | Tools blacklist |

### Git Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `coding.git.token_env` | string | `GITHUB_TOKEN` | Env var for GitHub token |
| `coding.git.user_name` | string | `Station Bot` | Git commit author name |
| `coding.git.user_email` | string | `station@cloudship.ai` | Git commit author email |

### Environment Variables

| Variable | Maps To |
|----------|---------|
| `STN_CODING_BACKEND` | `coding.backend` |
| `STN_CODING_OPENCODE_URL` | `coding.opencode.url` |
| `STN_NATS_URL` | `coding.nats.url` |

### Example

```yaml
coding:
  backend: claudecode
  claudecode:
    model: sonnet
    max_turns: 20
  git:
    user_name: "My Bot"
    user_email: "bot@example.com"
```

---

## Sandbox Settings

Configure isolated code execution in Docker containers.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `sandbox.enabled` | bool | `false` | Enable sandbox (compute mode) |
| `sandbox.code_mode_enabled` | bool | `false` | Enable code mode (persistent sessions) |
| `sandbox.idle_timeout_minutes` | int | `30` | Session idle timeout |
| `sandbox.docker_image` | string | `ubuntu:22.04` | Custom Docker image |

### Private Registry Authentication

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `sandbox.registry_auth.username` | string | - | Registry username |
| `sandbox.registry_auth.password` | string | - | Password or access token |
| `sandbox.registry_auth.identity_token` | string | - | OAuth token (ECR/GCR/ACR) |
| `sandbox.registry_auth.server_address` | string | - | Registry server URL |
| `sandbox.registry_auth.docker_config_path` | string | - | Path to Docker config.json |

### Environment Variables

| Variable | Maps To |
|----------|---------|
| `STN_SANDBOX_ENABLED`, `STATION_SANDBOX_ENABLED` | `sandbox.enabled` |
| `STN_SANDBOX_CODE_MODE_ENABLED`, `STATION_SANDBOX_CODE_MODE_ENABLED` | `sandbox.code_mode_enabled` |
| `STN_SANDBOX_IDLE_TIMEOUT_MINUTES` | `sandbox.idle_timeout_minutes` |
| `STN_SANDBOX_DOCKER_IMAGE` | `sandbox.docker_image` |
| `STN_SANDBOX_REGISTRY_USERNAME` | `sandbox.registry_auth.username` |
| `STN_SANDBOX_REGISTRY_PASSWORD` | `sandbox.registry_auth.password` |
| `STN_SANDBOX_REGISTRY_TOKEN` | `sandbox.registry_auth.identity_token` |
| `STN_SANDBOX_REGISTRY_SERVER` | `sandbox.registry_auth.server_address` |
| `STN_SANDBOX_REGISTRY_CONFIG` | `sandbox.registry_auth.docker_config_path` |

### Example: Public Image

```yaml
sandbox:
  enabled: true
  code_mode_enabled: true
```

### Example: Private Registry (ghcr.io)

```yaml
sandbox:
  enabled: true
  code_mode_enabled: true
  docker_image: "ghcr.io/myorg/custom-sandbox:latest"
  registry_auth:
    username: "myuser"
    password: "${GITHUB_TOKEN}"
    server_address: "ghcr.io"
```

### Example: AWS ECR

```yaml
sandbox:
  enabled: true
  docker_image: "123456789.dkr.ecr.us-east-1.amazonaws.com/sandbox:latest"
  registry_auth:
    identity_token: "${AWS_ECR_TOKEN}"
    server_address: "123456789.dkr.ecr.us-east-1.amazonaws.com"
```

---

## CloudShip Integration

Connect Station to CloudShip for remote management.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cloudship.enabled` | bool | `false` | Enable CloudShip integration |
| `cloudship.api_key` | string | - | Personal API key (`cst_...`) |
| `cloudship.registration_key` | string | - | Station registration key |
| `cloudship.endpoint` | string | `lighthouse.cloudshipai.com:443` | Lighthouse gRPC endpoint |
| `cloudship.use_tls` | bool | `true` | Use TLS for gRPC |
| `cloudship.name` | string | - | Station name (unique across org) |
| `cloudship.tags` | []string | - | Tags for filtering |

### Environment Variables

| Variable | Maps To |
|----------|---------|
| `STN_CLOUDSHIP_ENABLED` | `cloudship.enabled` |
| `STN_CLOUDSHIP_API_KEY` | `cloudship.api_key` |
| `STN_CLOUDSHIP_REGISTRATION_KEY` | `cloudship.registration_key` |
| `STN_CLOUDSHIP_ENDPOINT` | `cloudship.endpoint` |

### Example

```yaml
cloudship:
  enabled: true
  registration_key: ${CLOUDSHIP_REGISTRATION_KEY}
  name: "prod-station-1"
  tags:
    - production
    - us-east-1
```

---

## Telemetry Settings

Configure observability and tracing.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `telemetry.enabled` | bool | `true` | Enable telemetry |
| `telemetry.provider` | string | `jaeger` | Provider: `none`, `jaeger`, `otlp`, `cloudship` |
| `telemetry.endpoint` | string | `http://localhost:4318` | OTLP endpoint URL |

### Environment Variables

| Variable | Maps To |
|----------|---------|
| `STN_TELEMETRY_ENABLED`, `STATION_TELEMETRY_ENABLED` | `telemetry.enabled` |
| `STN_TELEMETRY_PROVIDER` | `telemetry.provider` |
| `STN_TELEMETRY_ENDPOINT`, `OTEL_EXPORTER_OTLP_ENDPOINT` | `telemetry.endpoint` |

### Example

```yaml
telemetry:
  enabled: true
  provider: otlp
  endpoint: "https://otel-collector.example.com:4318"
```

---

## Webhook Settings

Configure webhook-based agent execution.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `webhook.enabled` | bool | `true` | Enable webhook endpoint |
| `webhook.api_key` | string | - | Static API key for auth |

### Example

```yaml
webhook:
  enabled: true
  api_key: ${WEBHOOK_API_KEY}
```

---

## Notification Settings

Configure agent notifications and approvals.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `notifications.approval_webhook_url` | string | - | URL for approval requests |
| `notifications.approval_webhook_timeout` | int | `10` | Timeout in seconds |
| `notify.webhook_url` | string | - | URL for notifications (ntfy.sh) |
| `notify.format` | string | `ntfy` | Format: `ntfy`, `json`, `auto` |

### Environment Variables

| Variable | Maps To |
|----------|---------|
| `STN_NOTIFY_WEBHOOK_URL` | `notify.webhook_url` |
| `STN_NOTIFY_FORMAT` | `notify.format` |

### Example

```yaml
notify:
  webhook_url: "https://ntfy.sh/my-station-alerts"
  format: ntfy

notifications:
  approval_webhook_url: "https://my-app.com/webhooks/approvals"
  approval_webhook_timeout: 30
```

---

## Server Settings

Configure Station server ports and paths.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `api_port` | int | `8585` | API server port |
| `mcp_port` | int | `8586` | MCP server port |
| `debug` | bool | `false` | Enable debug mode |
| `workspace` | string | - | Custom workspace path |

### Environment Variables

| Variable | Maps To |
|----------|---------|
| `STN_API_PORT` | `api_port` |
| `STN_MCP_PORT` | `mcp_port` |
| `STN_DEBUG` | `debug` |
| `STN_WORKSPACE` | `workspace` |

### Example

```yaml
api_port: 8080
mcp_port: 8081
debug: false
workspace: "/opt/station/workspace"
```

---

## Complete Example

Here's a comprehensive `config.yaml` example:

```yaml
# AI Provider
ai_provider: anthropic
ai_api_key: ${ANTHROPIC_API_KEY}
ai_model: claude-sonnet-4-20250514

# Coding Backend
coding:
  backend: claudecode
  claudecode:
    model: sonnet
    max_turns: 20
  git:
    token_env: GITHUB_TOKEN
    user_name: "Station Bot"
    user_email: "station@example.com"

# Sandbox
sandbox:
  enabled: true
  code_mode_enabled: true
  docker_image: "ghcr.io/myorg/sandbox:latest"
  registry_auth:
    username: "${GITHUB_USER}"
    password: "${GITHUB_TOKEN}"
    server_address: "ghcr.io"

# CloudShip Integration
cloudship:
  enabled: true
  registration_key: ${CLOUDSHIP_REGISTRATION_KEY}
  name: "prod-station-1"
  tags:
    - production
    - us-east-1

# Telemetry
telemetry:
  enabled: true
  provider: otlp
  endpoint: "https://otel.example.com:4318"

# Notifications
notify:
  webhook_url: "https://ntfy.sh/my-alerts"
  format: ntfy

# Server
api_port: 8585
mcp_port: 8586
```

---

## See Also

- [Sandbox Documentation](./sandbox.md) - Detailed sandbox configuration
- [Installation Guide](./installation.md) - Getting started with Station
- [CloudShip Integration](../CLOUDSHIP_INTEGRATION.md) - Remote management setup
