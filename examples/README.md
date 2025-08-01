# Station MCP Server Examples

This directory contains example MCP server configurations that you can load into Station.

## Usage

### Load a specific MCP configuration:
```bash
cd examples/mcps
stn load aws-cli.json --env default
```

### Load all configurations in this directory:
```bash
cd examples/mcps
for config in *.json; do
  stn load "$config" --env default
done
```

## Template System

These configurations use Station's template system with placeholders like `{{AWS_ACCESS_KEY_ID}}`. When you load them, Station will:

1. **Detect templates** - Identify configurations with template variables
2. **Show credential forms** - Present forms for required API keys and secrets
3. **Validate input** - Check required fields and formats
4. **Store securely** - Encrypt sensitive values using Station's encryption

## Available MCP Servers

### Cloud Infrastructure
- **`aws-cli.json`** - Official AWS MCP server for managing AWS resources
- **`terraform.json`** - HashiCorp Cloud Platform Terraform integration
- **`pulumi.json`** - Pulumi infrastructure as code operations

### Container & Orchestration  
- **`kubernetes.json`** - Kubernetes kubectl integration for cluster management

### Development & Security
- **`github.json`** - Official GitHub repository management
- **`semgrep.json`** - Security vulnerability scanning

## Template Field Types

- **`string`** - Plain text input
- **`password`** - Hidden input for sensitive data
- **`required: true`** - Field must be provided
- **`sensitive: true`** - Value will be encrypted
- **`default`** - Pre-filled value
- **`help`** - Additional guidance for users

## Creating Your Own

Copy an existing configuration and modify:

```json
{
  "name": "My Service",
  "description": "Description of what this MCP server does",
  "mcpServers": {
    "my-service": {
      "command": "npx",
      "args": ["-y", "my-mcp-server"],
      "env": {
        "API_KEY": "{{API_KEY}}"
      }
    }
  },
  "templates": {
    "API_KEY": {
      "description": "Your service API key",
      "type": "password",
      "required": true,
      "sensitive": true
    }
  }
}
```

## More MCP Servers

Find more MCP servers at:
- [Awesome DevOps MCP Servers](https://github.com/rohitg00/awesome-devops-mcp-servers)
- [MCP Official Registry](https://github.com/modelcontextprotocol)
- [AWS Labs MCP](https://github.com/awslabs/mcp)