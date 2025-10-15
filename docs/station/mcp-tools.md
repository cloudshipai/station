# MCP Tools

Give agents access to APIs, databases, filesystems, and services using the Model Context Protocol.

## What are MCP Tools?

MCP (Model Context Protocol) tools are standardized interfaces that let agents interact with external systems:
- AWS APIs (Cost Explorer, EC2, S3, etc.)
- Databases (PostgreSQL, MySQL, Redis)
- Filesystems (local and remote)
- Security scanners (checkov, trivy, semgrep)
- Observability tools (Grafana, Datadog)
- And more...

## Fine-Grained Access Control

Station gives you fine-grained control over what each agent can do:

```yaml
tools:
  - "__get_cost_and_usage"          # AWS Cost Explorer - read only
  - "__list_cost_allocation_tags"   # Read cost tags
  - "__read_text_file"              # Filesystem read
  # No write permissions - agent can analyze but not modify
```

### Read vs Write Tools

[Content to be added]

## Configuring MCP Servers

[Content to be added]

## Available MCP Servers

[Content to be added - list of community MCP servers]

## Creating Custom MCP Tools

[Content to be added]
