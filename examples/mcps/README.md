# MCP Server Templates

This directory contains a comprehensive collection of MCP (Model Context Protocol) server templates for various services and integrations. These templates can be loaded directly into Station using the `stn load` command.

## Available Templates

### 🗄️ **Database Integrations**
- **`mysql.json`** - MySQL database operations and queries
- **`postgresql.json`** - PostgreSQL advanced SQL operations  
- **`sqlite.json`** - Lightweight SQLite database management
- **`mongodb.json`** - MongoDB document operations and aggregations
- **`redis.json`** - Redis caching and data store operations
- **`firebase.json`** - Firebase/Firestore NoSQL database
- **`elasticsearch.json`** - Full-text search and analytics

### ☁️ **Cloud Services**
- **`aws-cli.json`** - AWS resource management via CLI
- **`cloudflare.json`** - DNS, CDN, and Workers management
- **`kubernetes.json`** - Container orchestration and management
- **`terraform.json`** - Infrastructure as code
- **`pulumi.json`** - Modern infrastructure as code

### 🤖 **AI & LLM Integrations**
- **`anthropic-claude.json`** - Claude AI integration
- **`openai.json`** - OpenAI GPT models access

### 📊 **Productivity & Collaboration**
- **`github.json`** - GitHub repository management
- **`slack.json`** - Slack messaging and workflows
- **`jira.json`** - Project management and issue tracking
- **`google-sheets.json`** - Spreadsheet data operations

### 🌐 **Web & API Services**
- **`rest-api.json`** - Generic REST API client
- **`web-browser.json`** - Browser automation and scraping
- **`email-smtp.json`** - Email sending capabilities

### 🛠️ **Development & Operations**
- **`filesystem.json`** - Local file system operations
- **`git-advanced.json`** - Advanced Git operations
- **`docker.json`** - Container management
- **`ssh-remote.json`** - Remote server management
- **`monitoring-prometheus.json`** - System monitoring and alerts
- **`semgrep.json`** - Code security scanning

## Usage

### Loading Templates

You can load any template using the Station CLI:

```bash
# Load from local file
stn load examples/mcps/github.json

# Load from URL
stn load https://raw.githubusercontent.com/your-org/station/main/examples/mcps/github.json
```

### Interactive Loading

Use the interactive mode to customize templates:

```bash
stn load --interactive examples/mcps/aws-cli.json
```

### Environment-specific Loading

Load templates into specific environments:

```bash
stn load --env production examples/mcps/postgresql.json
```

## Template Structure

All templates follow this standardized structure:

```json
{
  "name": "Human-readable name",
  "description": "What this template provides",
  "mcpServers": {
    "server-key": {
      "command": "npx",
      "args": ["-y", "@package/name"],
      "env": {
        "VAR_NAME": "{{TEMPLATE_VAR}}"
      }
    }
  },
  "templates": {
    "TEMPLATE_VAR": {
      "description": "What this variable is for",
      "type": "string|password|number|boolean|path",
      "required": true|false,
      "sensitive": true|false,
      "default": "default-value",
      "help": "Additional context or instructions"
    }
  }
}
```

## Template Variables

### Types
- **`string`** - Text input
- **`password`** - Sensitive text (hidden input)
- **`number`** - Numeric input
- **`boolean`** - True/false selection
- **`path`** - File or directory path

### Properties
- **`required`** - Whether the variable must be provided
- **`sensitive`** - Whether to treat as secret (encrypted storage)
- **`default`** - Default value if not provided
- **`help`** - Additional context for users

## Security Best Practices

### Sensitive Data
- API keys, passwords, and tokens are marked as `sensitive: true`
- Station encrypts sensitive variables at rest
- Use environment variables for production deployments

### Authentication Tokens
Most services require authentication:
- **GitHub**: Personal access tokens with appropriate scopes
- **AWS**: IAM access keys with least-privilege policies  
- **Google**: Service account credentials or OAuth tokens
- **Slack**: Bot tokens from app configuration

## Contributing Templates

To add new MCP server templates:

1. Follow the standardized structure above
2. Include comprehensive `templates` section with proper types
3. Mark sensitive fields appropriately
4. Provide helpful descriptions and examples
5. Test with Station's load functionality
6. Update this README with your new template

### Template Guidelines

- **Names**: Use kebab-case for filenames (e.g., `my-service.json`)
- **Descriptions**: Be specific about what the integration provides
- **Help Text**: Include links to documentation or setup guides
- **Defaults**: Provide sensible defaults where possible
- **Security**: Always mark credentials as sensitive

## Examples

### Basic Usage
```bash
# Load filesystem tools for development
stn load examples/mcps/filesystem.json

# Set up AWS integration  
stn load examples/mcps/aws-cli.json
# Follow prompts to enter AWS_ACCESS_KEY_ID, etc.
```

### Advanced Scenarios
```bash
# Load multiple templates for a complete development stack
stn load examples/mcps/postgresql.json
stn load examples/mcps/redis.json  
stn load examples/mcps/github.json

# Load with environment override
stn load --env staging examples/mcps/kubernetes.json
```

## Troubleshooting

### Common Issues

1. **"Package not found"** - Some MCP servers may not be published to npm yet
2. **"Authentication failed"** - Verify your API keys and tokens are correct
3. **"Permission denied"** - Check that your credentials have the required permissions
4. **"Connection timeout"** - Verify network connectivity and service URLs

### Debug Mode
```bash
stn load --debug examples/mcps/your-template.json
```

## Community Templates

Looking for more templates? Check out:
- [Official MCP Registry](https://github.com/modelcontextprotocol/servers)
- [Community Templates](https://github.com/your-org/station-templates)
- [Custom Template Guide](../../docs/TEMPLATE_EXAMPLES.md)

## License

These templates are provided under the same license as Station. See [LICENSE](../../LICENSE) for details.