# Environment Management

Station's environment system provides secure isolation for agents and tools across development, staging, and production environments. Each environment maintains its own MCP server pool, agent definitions, and configuration variables.

## Environment Structure

### Directory Layout
```
~/.config/station/environments/
├── default/
│   ├── agents/              # Agent .prompt files
│   │   ├── File Analyzer.prompt
│   │   ├── Security Scanner.prompt
│   │   └── Code Reviewer.prompt
│   ├── template.json        # MCP server configurations
│   └── variables.yml        # Environment-specific variables
├── staging/
│   ├── agents/
│   ├── template.json
│   └── variables.yml
└── production/
    ├── agents/
    ├── template.json
    └── variables.yml
```

### File-Based Configuration

**Template Configuration** (`template.json`):
```json
{
  "name": "development-environment",
  "description": "Development environment with filesystem and security tools",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y", 
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    },
    "ship-security": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"]
    },
    "database": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-postgres@latest"
      ],
      "env": {
        "POSTGRES_URL": "{{ .DATABASE_URL }}"
      }
    }
  }
}
```

**Environment Variables** (`variables.yml`):
```yaml
PROJECT_ROOT: "/home/user/projects"
DATABASE_URL: "postgresql://user:pass@localhost:5432/dev_db"
AWS_REGION: "us-east-1"
SLACK_WEBHOOK_URL: "https://hooks.slack.com/services/..."
LOG_LEVEL: "debug"
```

## Environment Operations

### Creating Environments

```bash
# Create new environment
stn env create production

# Create from template
stn env create staging --from-template development

# Create with custom configuration
stn env create testing --config ./custom-template.json
```

### Environment Synchronization

```bash
# Sync specific environment (connects MCP servers)
stn sync production

# Sync with verbose logging
stn sync development --verbose

# Sync all environments
stn sync --all

# Force reconnect all MCP servers
stn sync production --force-reconnect
```

### Listing and Inspection

```bash
# List all environments
stn env list

# Show environment details
stn env show production

# Check environment status
stn env status development

# Validate environment configuration
stn env validate staging
```

## Variable Management

### Template Variable System

Station uses Go templates for dynamic variable resolution in MCP server configurations.

**Supported Variable Syntax**:
```json
{
  "command": "{{ .BINARY_PATH }}/my-tool",
  "args": ["--config", "{{ .CONFIG_PATH }}", "--env", "{{ .ENVIRONMENT }}"],
  "env": {
    "API_KEY": "{{ .SECRET_API_KEY }}",
    "DEBUG": "{{ .DEBUG_MODE | default \"false\" }}"
  }
}
```

**Variable Functions**:
- `{{ .VAR_NAME }}` - Simple variable substitution
- `{{ .VAR_NAME | default "value" }}` - Default value if variable not set
- `{{ .VAR_NAME | upper }}` - Convert to uppercase
- `{{ .VAR_NAME | lower }}` - Convert to lowercase

### Interactive Variable Prompting

When syncing environments with missing variables, Station provides an interactive UI:

```bash
stn sync production
# Opens browser to http://localhost:8585/sync
# Prompts for missing variables with form validation
# Variables saved to environment's variables.yml
```

**Variable Prompting Features**:
- Real-time validation of required variables
- Monaco Editor for multi-line values
- Auto-detection of all missing variables in configuration
- Encrypted storage of sensitive variables

### Managing Variables

```bash
# Set environment variable
stn env set-var production DATABASE_URL "postgresql://prod:secret@prod-db:5432/app"

# Get environment variable
stn env get-var production PROJECT_ROOT

# List all variables
stn env list-vars staging

# Import variables from file
stn env import-vars development ./dev-vars.yml

# Export variables to file
stn env export-vars production ./prod-vars.yml
```

## Multi-Environment Workflows

### Development to Production Pipeline

**1. Development Environment Setup**:
```bash
# Create development environment
stn env create development
cd ~/.config/station/environments/development

# Configure with development tools and relaxed security
cat > template.json << EOF
{
  "name": "development",
  "description": "Development environment with full tool access",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    },
    "ship-security": {
      "command": "ship", 
      "args": ["mcp", "security", "--stdio"]
    }
  }
}
EOF

# Set development variables
cat > variables.yml << EOF
PROJECT_ROOT: "/home/user/projects"
LOG_LEVEL: "debug"
DEBUG_MODE: "true"
EOF
```

**2. Staging Environment Setup**:
```bash
# Create staging environment
stn env create staging --from-template development

# Configure staging-specific settings
cd ~/.config/station/environments/staging
cat > variables.yml << EOF
PROJECT_ROOT: "/opt/staging/projects"
LOG_LEVEL: "info"
DEBUG_MODE: "false"
DATABASE_URL: "postgresql://staging:password@staging-db:5432/app"
EOF
```

**3. Production Environment Setup**:
```bash
# Create production environment with restricted tools
stn env create production

cd ~/.config/station/environments/production
cat > template.json << EOF
{
  "name": "production",
  "description": "Production environment with security-focused tools",
  "mcpServers": {
    "ship-security": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"]
    },
    "monitoring": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-prometheus@latest"],
      "env": {
        "PROMETHEUS_URL": "{{ .PROMETHEUS_URL }}"
      }
    }
  }
}
EOF

# Set production variables (use secrets management)
cat > variables.yml << EOF
PROMETHEUS_URL: "https://monitoring.company.com"
LOG_LEVEL: "warn"
EOF
```

### Agent Promotion Pipeline

```bash
# Develop agent in development environment
stn agent create "Security Auditor" --env development
stn agent run "Security Auditor" "Test security analysis" --env development

# Export agent for promotion
stn agent export "Security Auditor" ./security-auditor.prompt --env development

# Import to staging for testing
stn agent import ./security-auditor.prompt --env staging
stn agent run "Security Auditor" "Staging security test" --env staging

# Promote to production
stn agent import ./security-auditor.prompt --env production
```

## Environment Security

### Isolation Mechanisms

**1. Tool Access Isolation**:
- Each environment has separate MCP server pools
- Agents only access tools from their environment
- No cross-environment tool sharing

**2. Variable Isolation**:
- Environment variables are scoped to specific environments
- Sensitive variables encrypted at rest
- No variable bleeding between environments

**3. Agent Isolation**:
- Agent definitions stored per environment
- Execution contexts completely separate
- Database-level isolation for runs and metadata

### Security Best Practices

**1. Environment-Specific Secrets**:
```yaml
# Development
API_KEY: "dev_sk_1234567890abcdef"
DATABASE_URL: "postgresql://dev:password@localhost:5432/dev_db"

# Production  
API_KEY: "prod_sk_abcdef1234567890"
DATABASE_URL: "postgresql://prod:secure_pass@prod-db.internal:5432/prod_db"
```

**2. Tool Restriction by Environment**:
```json
{
  "development": {
    "mcpServers": {
      "filesystem": "full-access",
      "ship-security": "all-tools",
      "debug-tools": "enabled"
    }
  },
  "production": {
    "mcpServers": {
      "monitoring": "read-only",
      "ship-security": "security-only"
    }
  }
}
```

**3. Network Access Controls**:
- Development: Local access only
- Staging: Internal network access
- Production: Restricted to approved endpoints

## Bundle Integration

### Creating Environment Bundles

```bash
# Package entire environment into bundle
stn bundle create development-bundle --from-env development

# Package with specific agents only
stn bundle create security-bundle --from-env production \
  --agents "Security Scanner,Vulnerability Auditor"

# Create bundle with custom metadata
stn bundle create devops-bundle --from-env staging \
  --name "DevOps Security Bundle" \
  --description "Comprehensive security tools for DevOps teams" \
  --version "1.0.0"
```

### Installing Environment Bundles

```bash
# Install bundle to new environment
stn bundle install security-bundle.tar.gz security-env

# Install with custom variables during setup
stn bundle install devops-bundle.tar.gz devops \
  --set PROJECT_ROOT="/opt/projects" \
  --set LOG_LEVEL="info"

# Install from registry URL
stn bundle install https://registry.station.dev/bundles/terraform-security.tar.gz terraform
```

## Environment Monitoring

### Health Checks

```bash
# Check all environment health
stn env health

# Check specific environment
stn env health production

# Detailed health report with MCP server status
stn env health development --verbose
```

### Performance Monitoring

```bash
# Monitor environment resource usage
stn env monitor production

# Track MCP server performance
stn env stats development --mcp-servers

# Export metrics for external monitoring
stn env metrics production --export-prometheus
```

### Troubleshooting

**Common Environment Issues**:

```bash
# MCP server connection failures
stn env diagnose production
# Check: template.json syntax, variable resolution, binary availability

# Missing variables during sync
stn sync production --check-variables
# Lists all undefined variables before attempting connection

# Tool discovery failures
stn env test-tools development
# Tests each MCP server tool discovery individually

# Agent execution environment issues
stn agent run "Test Agent" "diagnostic task" --env production --debug
# Shows detailed environment loading and tool assignment
```

## Advanced Environment Features

### Environment Inheritance

```yaml
# base-environment.yml
template:
  mcpServers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]

variables:
  LOG_LEVEL: "info"
  DEBUG_MODE: "false"

# production inherits from base
extends: "base-environment.yml"
variables:
  PROJECT_ROOT: "/opt/production"
  LOG_LEVEL: "error"  # Override base setting
```

### Conditional Tool Loading

```json
{
  "mcpServers": {
    "debug-tools": {
      "command": "{{ if eq .ENVIRONMENT \"development\" }}debug-mcp-server{{ end }}",
      "condition": "{{ eq .ENVIRONMENT \"development\" }}"
    },
    "security-tools": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"],
      "condition": "{{ or (eq .ENVIRONMENT \"staging\") (eq .ENVIRONMENT \"production\") }}"
    }
  }
}
```

### Dynamic Environment Scaling

```bash
# Create environment cluster for load testing
stn env cluster create load-test --count 5 --template development

# Scale environment resources
stn env scale production --max-concurrent-agents 50

# Auto-scaling based on queue depth
stn env autoscale production --min-agents 5 --max-agents 20 --queue-threshold 10
```

This environment management system provides the foundation for secure, scalable multi-environment agent deployments while maintaining clean separation between development, staging, and production workloads.