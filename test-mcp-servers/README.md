# Test MCP Servers Collection

This directory contains real-world MCP servers for comprehensive end-to-end testing with the Station platform.

## Server Categories

### Cloud Infrastructure & DevOps
- **aws-mcp-servers/** - AWS official MCP servers for ECS, EKS, Serverless
- **terraform-mcp-server/** - HashiCorp Terraform integration
- **docker-mcp-servers/** - Docker official MCP servers
- **ansible-mcp-server/** - Ansible configuration management

### Database & Local Development
- **mcp-database-server/** - SQLite, SQL Server, PostgreSQL support
- **mcp-sqlite/** - Comprehensive SQLite operations
- **official-mcp-servers/** - Reference implementations (filesystem, etc.)

## Testing Strategy

1. **Authentication Setup** - Configure credentials for cloud services
2. **Local Testing** - Test filesystem and database servers first
3. **Cloud Integration** - Test AWS, Docker, Terraform servers
4. **End-to-End** - Run full station test suite with real credentials

## Configuration Notes

Each server directory contains its own README with setup instructions.
Credentials should be configured via environment variables or config files.