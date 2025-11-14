# MCP Faker Proxy

The MCP Faker Proxy is a transparent proxy for MCP (Model Context Protocol) servers that learns response schemas and generates realistic mock data. This is useful for development and testing when you don't have access to real external services or want to work offline.

## How It Works

The faker proxy sits between your MCP client and a target MCP server:

1. **Transparent Proxying**: Forwards all JSON-RPC messages bidirectionally
2. **Schema Learning**: Analyzes successful responses and builds a schema cache
3. **Smart Enrichment**: Generates realistic mock data based on learned patterns

## Features

- **Pattern Recognition**: Detects emails, UUIDs, URLs, timestamps, IP addresses from sample data
- **Field Name Mapping**: Generates appropriate data based on field names (id, name, email, address, etc.)
- **Nested Structures**: Handles complex objects and arrays with proper type preservation
- **Disk Persistence**: Schemas are cached to `~/.cache/station/faker/` for reuse across sessions
- **Passthrough Mode**: Can run in pure proxy mode without enrichment for debugging

## MCP Server Configuration

### Basic Configuration

Add the faker proxy as an MCP server in your Station environment's `template.json`:

```json
{
  "mcpServers": {
    "faker-filesystem": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-filesystem@latest,/tmp"
      ]
    }
  }
}
```

Or use with Station's built-in mock servers:

```json
{
  "mcpServers": {
    "faker-github": {
      "command": "stn",
      "args": ["faker", "--command", "stn", "--args", "mock,github"]
    }
  }
}
```

### With Environment Variables

Pass environment variables to the target MCP server:

```json
{
  "mcpServers": {
    "faker-aws": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-aws@latest",
        "--env", "AWS_REGION=us-east-1",
        "--env", "AWS_PROFILE=dev"
      ]
    }
  }
}
```

### With Debug Logging

Enable debug logging to see schema analysis and enrichment activity:

```json
{
  "mcpServers": {
    "faker-aws-debug": {
      "command": "stn",
      "args": [
        "faker",
        "--debug",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-aws@latest",
        "--env", "AWS_REGION=us-east-1"
      ]
    }
  }
}
```

### Passthrough Mode (No Enrichment)

Use passthrough mode to proxy without enrichment, useful for debugging:

```json
{
  "mcpServers": {
    "faker-datadog": {
      "command": "stn",
      "args": [
        "faker",
        "--passthrough",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-datadog@latest",
        "--env", "DATADOG_API_KEY=test-key",
        "--env", "DATADOG_APP_KEY=test-app-key"
      ]
    }
  }
}
```

### Custom Cache Directory

Specify a custom cache directory for schemas:

```json
{
  "mcpServers": {
    "faker-github": {
      "command": "stn",
      "args": [
        "faker",
        "--cache-dir", "/custom/path/to/cache",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-github@latest",
        "--env", "GITHUB_TOKEN={{ .GITHUB_TOKEN }}"
      ]
    }
  }
}
```

**Note**: Environment variables in template.json are passed to Station, not the target MCP server. Use `--env` flags to pass environment variables to the target MCP server.

## Command-Line Usage

The faker proxy can also be used directly from the command line:

```bash
# Proxy the filesystem MCP server
stn faker \
  --target-command npx \
  --target-args "-y,@modelcontextprotocol/server-filesystem@latest,/home/user/projects" \
  --debug

# Proxy with environment variables
stn faker \
  --target-command npx \
  --target-args "-y,@modelcontextprotocol/server-aws@latest" \
  --target-env "AWS_REGION=us-east-1,AWS_PROFILE=dev" \
  --cache-dir ~/.cache/station/faker-aws

# Pure passthrough mode (no enrichment)
stn faker \
  --passthrough \
  --target-command npx \
  --target-args "-y,@modelcontextprotocol/server-stripe@latest"
```

## Argument Format

- **--target-args**: Comma-separated list of arguments (e.g., `-y,@scope/package,arg1,arg2`)
- **--target-env**: Comma-separated KEY=VALUE pairs (e.g., `API_KEY=abc,REGION=us-east-1`)
- **--cache-dir**: Directory path for schema cache (default: `~/.cache/station/faker`)
- **--debug**: Enable verbose logging to stderr
- **--passthrough**: Disable enrichment, pure proxy mode

## Use Cases

### Development Without External Services

Test agent workflows without connecting to real AWS/Datadog/GitHub:

```json
{
  "mcpServers": {
    "faker-aws-dev": {
      "command": "stn",
      "args": [
        "faker",
        "--target-command", "npx",
        "--target-args", "-y,@modelcontextprotocol/server-aws@latest"
      ]
    }
  }
}
```

After a few real requests, the faker will learn the schema and generate realistic mock responses offline.

### Testing MCP Servers

Validate that your MCP client handles various response shapes correctly:

```bash
stn faker \
  --debug \
  --target-command /path/to/your/mcp-server \
  --target-args "arg1,arg2"
```

### Offline Development

Work on agents without internet or API credentials by using cached schemas:

```json
{
  "mcpServers": {
    "faker-stripe-offline": {
      "command": "stn",
      "args": [
        "faker",
        "--cache-dir", "~/.cache/station/faker-stripe",
        "--command", "npx",
        "--args", "-y,@modelcontextprotocol/server-stripe@latest"
      ]
    }
  }
}
```

Or wrap Station's mock servers to add learned enrichment:

```json
{
  "mcpServers": {
    "faker-aws": {
      "command": "stn",
      "args": ["faker", "--command", "stn", "--args", "mock,aws-cost-explorer"]
    },
    "faker-grafana": {
      "command": "stn",
      "args": ["faker", "--command", "stn", "--args", "mock,grafana"]
    }
  }
}
```

## Schema Cache

Schemas are stored as JSON files in the cache directory:

```
~/.cache/station/faker/
├── tool_response.json       # Generic tool responses
├── aws_list_instances.json  # AWS-specific tools
├── datadog_metrics.json     # Datadog-specific tools
└── ...
```

Each schema file contains:
- Type information (string, number, bool, array, object)
- Array bounds (min/max observed lengths)
- Sample values for pattern detection
- Nested structure definitions

## Pattern Detection

The enricher recognizes these patterns automatically:

**String Patterns:**
- Email addresses (contains `@`)
- URLs (starts with `http://` or `https://`)
- UUIDs (36 chars with 4 dashes)
- ISO timestamps (contains `T` and `Z` or `+`)
- File paths (starts with `/` and multiple `/`)
- IP addresses (3 dots)

**Field Name Patterns:**
- `id`, `*_id` → UUID
- `name`, `*_name` → Person name
- `email` → Email address
- `username`, `user` → Username
- `*created*`, `*updated*`, `*date*`, `*time*` → RFC3339 timestamp
- `*address*` → Street address
- `*city*` → City name
- `*country*` → Country name
- `*ip*` → IPv4 address
- `*url*`, `*uri*` → URL
- `*status*` → Random status (active, inactive, pending, completed)
- `*count*`, `*total*` → Number (0-1000)
- `*price*`, `*amount*` → Price ($1-$1000)

## Limitations

- **Schema Evolution**: The proxy learns from successful responses only. Initial requests must reach the real server.
- **Complex Logic**: Cannot replicate server-side business logic or computations.
- **Credentials**: Still requires valid credentials for the initial learning phase.
- **Tool Name Tracking**: Currently uses generic tool names; future versions will track per-tool schemas.

## Future Enhancements

- Per-tool schema tracking with request-response correlation
- Manual schema seeding from JSON Schema definitions
- Custom field name pattern configuration
- Schema versioning and migration tools
- Interactive schema editor UI
