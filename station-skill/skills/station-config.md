# Station Configuration

Configure Station settings including AI provider, coding backend, CloudShip integration, and more.

## Browser-Based Configuration (Recommended)

The easiest way to configure Station is via the browser UI:

```bash
# Opens browser-based config editor
# Automatically starts server if not running
stn config --browser
```

This opens a visual editor with:
- All configuration sections organized
- Conditional fields (e.g., backend-specific settings only show for selected backend)
- Secret fields with show/hide toggle
- Validation and defaults

## CLI Configuration Commands

### View Configuration

```bash
# Show all config (secrets redacted)
stn config show

# Show specific section
stn config show ai
stn config show coding
stn config show cloudship

# Show config file path
stn config path

# Show all available config keys
stn config schema
```

### Set Configuration Values

```bash
# Set a value
stn config set <key> <value>

# Examples
stn config set ai_provider anthropic
stn config set ai_model claude-sonnet-4-20250514
stn config set coding.backend opencode
stn config set cloudship.enabled true

# Set nested values
stn config set coding.nats.url nats://localhost:4222
stn config set cloudship.api_key cst_xxx

# Reset to default
stn config reset <key>
```

### Edit Config File Directly

```bash
# Open in $EDITOR
stn config edit
```

## Configuration Sections

### AI Provider (`ai_*`)

| Key | Description | Default |
|-----|-------------|---------|
| `ai_provider` | Provider: openai, anthropic, ollama, gemini | openai |
| `ai_model` | Model name | gpt-4o |
| `ai_api_key` | API key (secret) | - |
| `ai_base_url` | Custom base URL for OpenAI-compatible | - |

### Coding Backend (`coding.*`)

| Key | Description | Default |
|-----|-------------|---------|
| `coding.backend` | Backend: opencode, opencode-nats, opencode-cli, claudecode | opencode-cli |
| `coding.workspace_base_path` | Base path for workspaces | /tmp/station-coding |
| `coding.max_attempts` | Max retry attempts | 3 |
| `coding.task_timeout_min` | Task timeout in minutes | 30 |

**Backend-specific settings:**

For `opencode`:
- `coding.opencode.url` - OpenCode HTTP server URL

For `opencode-nats`:
- `coding.nats.url` - NATS server URL
- `coding.nats.subjects.task` - Task subject
- `coding.nats.subjects.result` - Result subject
- `coding.nats.subjects.stream` - Stream subject

For `opencode-cli`:
- `coding.cli.binary_path` - Path to opencode binary
- `coding.cli.timeout_sec` - CLI timeout

For `claudecode`:
- `coding.claudecode.binary_path` - Path to claude binary
- `coding.claudecode.timeout_sec` - Timeout
- `coding.claudecode.model` - Model: sonnet, opus, haiku
- `coding.claudecode.max_turns` - Max conversation turns
- `coding.claudecode.allowed_tools` - Tool whitelist
- `coding.claudecode.disallowed_tools` - Tool blacklist

### CloudShip Integration (`cloudship.*`)

| Key | Description | Default |
|-----|-------------|---------|
| `cloudship.enabled` | Enable CloudShip | false |
| `cloudship.api_key` | Personal API key (cst_...) | - |
| `cloudship.registration_key` | Station registration key | - |
| `cloudship.endpoint` | Lighthouse gRPC endpoint | lighthouse.cloudshipai.com:443 |
| `cloudship.use_tls` | Use TLS | true |
| `cloudship.name` | Station name (unique) | - |
| `cloudship.tags` | Tags for filtering | - |

### Server Settings (`api_port`, `mcp_port`, etc.)

| Key | Description | Default |
|-----|-------------|---------|
| `api_port` | API server port | 8585 |
| `mcp_port` | MCP server port | 8586 |
| `debug` | Debug mode | false |
| `workspace` | Custom workspace path | - |

### Other Sections

- **Telemetry** (`telemetry.*`) - Tracing/observability settings
- **Sandbox** (`sandbox.*`) - Code execution sandbox settings
- **Webhook** (`webhook.*`) - Webhook endpoint settings
- **Notifications** (`notifications.*`, `notify.*`) - Alert/notification settings

## Common Workflows

### Initial Setup

```bash
# Configure AI provider
stn config set ai_provider anthropic
stn config set ai_model claude-sonnet-4-20250514

# Or use browser for full setup
stn config --browser
```

### Connect to CloudShip

```bash
stn config set cloudship.enabled true
stn config set cloudship.api_key cst_your_api_key
stn config set cloudship.registration_key your_reg_key
stn config set cloudship.name my-station
```

### Change Coding Backend

```bash
# Switch to OpenCode with NATS
stn config set coding.backend opencode-nats
stn config set coding.nats.url nats://localhost:4222

# Or use Claude Code
stn config set coding.backend claudecode
stn config set coding.claudecode.model opus
```

## Config File Location

Default: `~/.config/station/config.yaml`

Override with `--config` flag or `STN_CONFIG` environment variable.
