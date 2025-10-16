# Documentation TODO

## Completed Documentation

- ✅ architecture.md (8.5KB) - Complete with design decisions and rationale
- ✅ deployment-modes.md (11KB) - All modes: up, stdio, serve, Docker
- ✅ examples.md (17KB) - Real-world FinOps, Security, Deployment agents
- ✅ installation.md (10KB) - Linux, macOS, Windows, Docker guides
- ✅ docs/station/README.md - Table of contents
- ✅ .gitignore - Fixed to allow docs/station/

## Remaining Stub Files

These files exist but contain only placeholder content:

### agent-development.md
**Priority**: High (linked from README)
**Content Needed**:
- Complete dotprompt format reference
- Agent development workflow (UI + Claude Code/Cursor)
- Testing agents locally
- Debugging agent executions
- Best practices
- Tool selection guidance

### mcp-tools.md
**Priority**: High (linked from README and examples)
**Content Needed**:
- MCP protocol overview
- Available MCP servers (AWS, Stripe, Grafana, filesystem, etc.)
- Adding MCP tools to environments
- Creating custom MCP servers
- Tool naming conventions
- Fine-grained permissions

### templates.md
**Priority**: High (linked from README)
**Content Needed**:
- Go template syntax guide
- Variable resolution process
- Common patterns (AWS credentials, database URLs, etc.)
- Environment-specific configuration
- Secrets management
- Template debugging

### bundles.md
**Priority**: Medium (linked from README)
**Content Needed**:
- Bundle structure and format
- Creating bundles from environments
- Installing bundles
- Sharing bundles (registry)
- Versioning bundles
- Bundle best practices

### zero-config-deployments.md
**Priority**: Medium (linked from README)
**Content Needed**:
- IAM role-based AWS deployments
- GCP service account auth
- Azure managed identity
- Kubernetes service accounts
- Docker credential passthrough
- Environment variable discovery

### docker-compose-deployments.md
**Priority**: Low (linked from README)
**Content Needed**:
- Production docker-compose examples
- Multi-container setups
- Volume management
- Network configuration
- Health checks
- Scaling considerations

## Quick Fill Template

For each remaining doc, include:
1. **Overview** - What it is and why it matters
2. **Concepts** - Key concepts users need to understand
3. **Usage Examples** - Real-world usage patterns
4. **Best Practices** - Do's and don'ts
5. **Troubleshooting** - Common issues and solutions
6. **Next Steps** - Links to related docs

## Estimated Effort

- agent-development.md: ~2 hours
- mcp-tools.md: ~2 hours
- templates.md: ~1.5 hours
- bundles.md: ~1.5 hours
- zero-config-deployments.md: ~1 hour
- docker-compose-deployments.md: ~1 hour

Total: ~9 hours to complete all remaining documentation

## Notes

- Focus on user-facing concepts and usage patterns
- Avoid implementation details unless necessary for understanding
- Include real examples from the codebase where possible
- Cross-reference between docs to help users navigate
- Keep examples practical and production-ready
