# MCP Template Examples

This document shows how to handle multiple templates with overlapping variable names using different strategies.

## Problem: Multiple Templates, Same Variables

Consider these templates that both use `ApiKey` but need different values:

### Template 1: `github-tools.json`
```json
{
  "mcpServers": {
    "{{.ServerName}}": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{.ApiKey}}",
        "GITHUB_REPOSITORY": "{{.Repository | default \"owner/repo\"}}"
      }
    }
  }
}
```

### Template 2: `aws-tools.json`
```json
{
  "mcpServers": {
    "{{.ServerName}}": {
      "command": "aws-mcp-server",
      "args": ["--region", "{{.Region}}"],
      "env": {
        "AWS_ACCESS_KEY_ID": "{{.ApiKey}}",
        "AWS_SECRET_ACCESS_KEY": "{{.SecretKey}}",
        "AWS_DEFAULT_REGION": "{{.Region | default \"us-east-1\"}}"
      }
    }
  }
}
```

## Solution Strategies

### Strategy 1: Template-Specific Variable Files (Recommended)

**Directory Structure:**
```
~/.config/station/environments/dev/
├── variables.env              # Global variables
├── template-vars/
│   ├── github-tools.env      # GitHub-specific variables
│   └── aws-tools.env         # AWS-specific variables
├── github-tools.json         # GitHub template
└── aws-tools.json            # AWS template
```

**Global Variables** (`variables.env`):
```bash
# Global settings that apply to all templates
Environment=development
LogLevel=info
```

**GitHub Variables** (`template-vars/github-tools.env`):
```bash
# GitHub-specific variables
ServerName=github
ApiKey=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxx
Repository=myorg/myrepo
```

**AWS Variables** (`template-vars/aws-tools.env`):
```bash
# AWS-specific variables  
ServerName=aws-tools
ApiKey=AKIAIOSFODNN7EXAMPLE
SecretKey=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
Region=us-west-2
```

### Strategy 2: Namespaced Variables

**Single Variables File** (`variables.env`):
```bash
# GitHub variables
GitHub_ServerName=github
GitHub_ApiKey=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxx
GitHub_Repository=myorg/myrepo

# AWS variables
AWS_ServerName=aws-tools
AWS_ApiKey=AKIAIOSFODNN7EXAMPLE
AWS_SecretKey=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
AWS_Region=us-west-2
```

**Updated Templates** use namespaced variables:

`github-tools.json`:
```json
{
  "mcpServers": {
    "{{.GitHub.ServerName}}": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{.GitHub.ApiKey}}",
        "GITHUB_REPOSITORY": "{{.GitHub.Repository | default \"owner/repo\"}}"
      }
    }
  }
}
```

`aws-tools.json`:
```json
{
  "mcpServers": {
    "{{.AWS.ServerName}}": {
      "command": "aws-mcp-server",
      "args": ["--region", "{{.AWS.Region}}"],
      "env": {
        "AWS_ACCESS_KEY_ID": "{{.AWS.ApiKey}}",
        "AWS_SECRET_ACCESS_KEY": "{{.AWS.SecretKey}}",
        "AWS_DEFAULT_REGION": "{{.AWS.Region | default \"us-east-1\"}}"
      }
    }
  }
}
```

### Strategy 3: Context-Aware Templates

**Variables File** (`variables.env`):
```bash
# Service configurations as nested structures
GitHub_Config={"ServerName":"github","ApiKey":"ghp_xxx","Repository":"myorg/myrepo"}
AWS_Config={"ServerName":"aws-tools","ApiKey":"AKIA_xxx","SecretKey":"secret","Region":"us-west-2"}
```

**Templates** access nested configuration:

`github-tools.json`:
```json
{
  "mcpServers": {
    "{{.GitHub_Config.ServerName}}": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{.GitHub_Config.ApiKey}}",
        "GITHUB_REPOSITORY": "{{.GitHub_Config.Repository | default \"owner/repo\"}}"
      }
    }
  }
}
```

## Template Functions and Validation

### Advanced Template Features

**Template with Validation**:
```json
{
  "mcpServers": {
    "{{required \"Server name is required\" .ServerName}}": {
      "command": "{{required \"Command is required\" .Command}}",
      "args": {{.Args | toJSON}},
      "env": {
        "API_TOKEN": "{{required \"API token is required\" .ApiKey}}",
        "DEBUG": "{{.Debug | default false}}",
        "TIMEOUT": "{{.Timeout | default 30}}"
      }
    }
  }
}
```

**Template with Environment Fallback**:
```json
{
  "mcpServers": {
    "{{.ServerName}}": {
      "command": "{{.Command}}",
      "env": {
        "API_KEY": "{{.ApiKey | default (env \"DEFAULT_API_KEY\")}}",
        "REGION": "{{.Region | default (env \"AWS_DEFAULT_REGION\") | default \"us-east-1\"}}"
      }
    }
  }
}
```

## Configuration Management

### Variable Resolution Order

When using **Template-Specific Strategy**:

1. **Template-specific variables** (highest precedence)
2. **Global variables** 
3. **Environment variables** (lowest precedence)

### CLI Usage Examples

```bash
# List templates and their variables
stn mcp config list --env dev

# Create template-specific variables
stn mcp vars set ApiKey=ghp_xxx --template github-tools --env dev

# Edit template variables interactively
stn mcp vars edit --template aws-tools --env dev

# Preview rendered template
stn mcp config render github-tools --env dev

# Validate all templates in environment
stn mcp config validate --env dev
```

### Migration from Single Variables

```bash
# Export existing variables to template-specific files
stn mcp migrate split-vars --env dev

# Import template-specific variables
stn mcp vars import github-tools.env --template github-tools --env dev
```

## Best Practices

### 1. Use Template-Specific Variables for Different Services
```
✅ Good: Separate files for each service
template-vars/
├── github-tools.env
├── aws-tools.env
└── datadog-tools.env

❌ Avoid: Mixing all variables in one file with conflicts
variables.env with GitHub_ApiKey, AWS_ApiKey, Datadog_ApiKey
```

### 2. Keep Global Variables Generic
```
✅ Good: Environment-wide settings
Environment=production
LogLevel=warn
MaxRetries=3

❌ Avoid: Service-specific details in global
GitHubToken=xxx  # This should be template-specific
```

### 3. Use Descriptive Variable Names
```
✅ Good: Clear purpose
DatabaseConnectionString=postgresql://...
SlackWebhookURL=https://hooks.slack.com/...

❌ Avoid: Generic names
URL=https://...
Token=xxx
```

### 4. Document Template Variables
```json
{
  "// Variables": {
    "ServerName": "Name of the MCP server instance",
    "ApiKey": "GitHub Personal Access Token with repo permissions",
    "Repository": "GitHub repository in owner/repo format"
  },
  "mcpServers": {
    "{{.ServerName}}": {
      // ... template content
    }
  }
}
```

## Security Considerations

### Variable File Permissions
```bash
# Global variables (may contain non-secrets)
chmod 644 variables.env

# Template-specific variables (often contain secrets)
chmod 600 template-vars/*.env
```

### Git Configuration
```gitignore
# Exclude all secret files
environments/*/variables.env
environments/*/template-vars/*.env
secrets/

# Allow template files
!*.json
!*.yaml
```

### Environment Separation
```
# Development
environments/dev/template-vars/github-tools.env  # Dev GitHub token

# Production  
environments/prod/template-vars/github-tools.env # Prod GitHub token
```

This approach ensures that:
- Templates can be version controlled safely
- Secrets are kept separate and environment-specific  
- Variable conflicts are resolved clearly
- Teams can share templates while managing their own secrets