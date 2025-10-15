# Template Variables

Secure configuration management with Go template variables.

## Why Template Variables?

Traditional configuration files hardcode sensitive values:
- ❌ API keys in plain text
- ❌ Paths and credentials exposed
- ❌ Can't share configs safely

Station uses template variables to keep secrets secure:
- ✅ Values resolved at runtime
- ✅ Environment-specific configuration
- ✅ Share configs without exposing secrets

## Template Syntax

Station uses Go template syntax:

```json
{
  "mcpServers": {
    "aws-cost-explorer": {
      "command": "mcp-server-aws",
      "env": {
        "AWS_REGION": "{{ .AWS_REGION }}",
        "AWS_PROFILE": "{{ .AWS_PROFILE }}"
      }
    }
  }
}
```

## Variables File

Define variables in `variables.yml`:

```yaml
AWS_REGION: "us-east-1"
AWS_PROFILE: "production"
PROJECT_ROOT: "/workspace"
```

## Runtime Resolution

[Content to be added - explain how variables are resolved]

## Environment-Specific Configuration

[Content to be added - dev/staging/prod examples]

## Best Practices

[Content to be added]
