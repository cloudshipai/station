# Station Security Guide

Station is designed with security as a core principle, ensuring your credentials and infrastructure remain secure while enabling powerful AI agent automation.

## Security Philosophy

**"Zero Trust, Local First"** - Station assumes no external service can be trusted with your credentials and keeps everything within your infrastructure perimeter.

## Core Security Principles

### 🔒 Credential Isolation
- **Your secrets never leave your infrastructure**
- All API keys, tokens, and credentials stay within Station
- AI providers only receive task descriptions, never credential data
- MCP servers run locally with your permissions

### 🏰 Environment Isolation
- Separate configurations for dev/staging/production
- Environment-specific tool access and credentials
- Agent execution isolated by environment boundaries
- No cross-environment data leakage

### 🔐 Encryption at Rest
- Database encrypted with AES-256 encryption
- Sensitive configuration data encrypted
- Secure key generation and management
- Encrypted backups and exports

## Authentication & Authorization

### User Management
```bash
# Create users with role-based access
./stn user create alice --role admin
./stn user create bob --role developer
./stn user create monitor --role viewer

# API key authentication
export STATION_API_KEY=stn_1234567890abcdef
./stn agent list --endpoint https://station.company.com
```

### Role-Based Access Control
- **admin**: Full access to all environments and configurations
- **developer**: Create and manage agents in assigned environments
- **viewer**: Read-only access to agent status and run history
- **Custom roles**: Define specific permissions per user

### Environment-Based Permissions
```yaml
# Example permission matrix
user: alice
environments:
  - development: admin
  - staging: developer
  - production: viewer

user: bob  
environments:
  - development: developer
  - staging: viewer
  - production: none
```

## Network Security

### Local-First Architecture
- **No external dependencies**: All processing happens locally
- **Your AI provider choice**: Use your existing AI API relationships
- **VPC deployment**: Deploy within your secure network perimeter
- **Firewall friendly**: Minimal port requirements

### Network Ports
```bash
# Required ports
8080  # HTTP API (can be disabled)
2222  # SSH terminal interface (optional)
3000  # MCP server port (internal only)

# Optional ports for team access
443   # HTTPS API with TLS termination
80    # HTTP redirect to HTTPS
```

### TLS Configuration
```yaml
# Enable HTTPS for production
api:
  tls:
    enabled: true
    cert_file: /etc/ssl/certs/station.crt
    key_file: /etc/ssl/private/station.key
    min_version: "1.2"
    cipher_suites:
      - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
      - "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305"
```

## Secrets Management

### Environment Variables
```bash
# Sensitive values as environment variables
export STN_DATABASE_URL="postgresql://user:${DB_PASSWORD}@localhost/station"
export STN_ENCRYPTION_KEY="${ENCRYPTION_KEY}"
export STN_AI_API_KEY="${AI_API_KEY}"

# MCP server credentials
export GITHUB_TOKEN="${GITHUB_TOKEN}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}"
export SLACK_BOT_TOKEN="${SLACK_BOT_TOKEN}"
```

### Template Variable Security
```json
{
  "name": "GitHub Integration",
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@github/github-mcp-server"],
      "env": {
        "GITHUB_TOKEN": "{{GITHUB_TOKEN}}"
      }
    }
  },
  "templates": {
    "GITHUB_TOKEN": {
      "description": "GitHub Personal Access Token",
      "type": "password",
      "required": true,
      "sensitive": true
    }
  }
}
```

### Key Management Best Practices
```bash
# Generate secure encryption key
openssl rand -hex 32 > /etc/station/encryption.key
chmod 600 /etc/station/encryption.key

# Use key management systems in production
export STN_ENCRYPTION_KEY=$(aws ssm get-parameter --name "/station/encryption-key" --with-decryption --query Parameter.Value --output text)
```

## Agent Security

### Tool Permissions
```yaml
# Fine-grained tool assignment
agent: "Database Monitor"
tools:
  - server: "postgresql-mcp"
    allowed_tools: ["query", "stats", "health"]
    denied_tools: ["drop", "delete", "truncate"]
  - server: "filesystem-mcp"  
    allowed_tools: ["read", "list"]
    denied_tools: ["write", "delete", "execute"]
```

### Execution Isolation
- **Sandboxed execution**: Each agent runs in isolated context
- **Resource limits**: CPU, memory, and execution time limits
- **Network restrictions**: Agents inherit Station's network permissions
- **File system access**: Limited by MCP server configuration

### Input Validation
```go
// All user inputs are validated
type AgentRequest struct {
    Name        string `json:"name" validate:"required,min=1,max=100"`
    Description string `json:"description" validate:"required,min=1,max=500"`
    Task        string `json:"task" validate:"required,min=1,max=2000"`
}
```

## MCP Server Security

### Server Validation
```bash
# Only load trusted MCP servers
./stn mcp test server-name  # Test before enabling
./stn mcp list             # Review loaded servers
./stn mcp tools server-name # Audit available tools
```

### Configuration Security
```json
{
  "name": "filesystem",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/restricted/path"],
  "env": {},
  "security": {
    "allowed_paths": ["/app/data", "/tmp/station"],
    "denied_paths": ["/etc", "/root", "/home"],
    "read_only": false,
    "max_file_size": "10MB"
  }
}
```

### Server Communication
- **Local communication**: MCP servers run as local processes
- **Encrypted channels**: Communication encrypted in transit
- **Authentication**: Server-specific authentication mechanisms
- **Timeout handling**: Prevents hung connections

## Audit & Compliance

### Audit Logging
```bash
# Enable comprehensive audit logging
./stn settings set audit_logging true

# Audit log format
{
  "timestamp": "2024-01-15T10:30:45Z",
  "user_id": "alice",
  "action": "agent_run",
  "resource": "agent_1", 
  "environment": "production",
  "ip_address": "10.0.1.100",
  "success": true
}
```

### Compliance Features
- **SOC 2 Ready**: Audit trails and access controls
- **GDPR Compliant**: Data minimization and user consent
- **HIPAA Compatible**: Encryption and access controls
- **ISO 27001**: Security management framework support

### Data Retention
```yaml
# Configure data retention policies
retention:
  agent_runs: "90d"      # Execution history
  audit_logs: "1y"       # Security events  
  webhooks: "30d"        # Delivery history
  sessions: "24h"        # User sessions
```

## Vulnerability Management

### Security Updates
```bash
# Check for security updates
./stn --version
./stn security check

# Update dependencies
go mod tidy
npm audit fix
```

### Known Security Considerations
1. **AI Model Access**: Station requires AI API access (OpenAI, Anthropic, etc.)
2. **MCP Server Trust**: MCP servers run with Station's permissions
3. **Local Network**: Station trusts local network communications
4. **File System**: Limited by OS-level permissions

### Security Scanning
```bash
# Run security scans
govulncheck ./...
npm audit
docker scan station:latest

# Code analysis
golangci-lint run
semgrep --config=auto .
```

## Incident Response

### Security Monitoring
```bash
# Monitor for security events
./stn logs --level error --filter security
./stn audit --since "1h ago"
./stn metrics | grep failed_authentication
```

### Incident Response Plan
1. **Detection**: Automated alerting on security events
2. **Assessment**: Review audit logs and system state
3. **Containment**: Disable affected users/agents if needed
4. **Investigation**: Full forensic analysis
5. **Recovery**: Restore services and implement fixes
6. **Post-Incident**: Update security controls

### Emergency Procedures
```bash
# Emergency shutdown
./stn server stop --force

# Disable all agents
./stn agent disable --all

# Revoke API keys
./stn user revoke alice
./stn user list --disabled

# Backup investigation data
./stn export --audit-logs --since "24h ago"
```

## Production Security Checklist

### Pre-Deployment
- [ ] **Encryption keys generated** and securely stored
- [ ] **TLS certificates** configured for HTTPS
- [ ] **Firewall rules** configured (minimal ports)
- [ ] **User accounts** created with appropriate roles
- [ ] **Audit logging** enabled and configured
- [ ] **Backup strategy** implemented and tested
- [ ] **Monitoring** and alerting configured
- [ ] **Security scanning** completed (no high/critical issues)

### Post-Deployment
- [ ] **Access controls** tested and verified
- [ ] **Audit logs** reviewed for anomalies
- [ ] **Performance monitoring** baseline established
- [ ] **Incident response** procedures tested
- [ ] **Security training** completed for operators
- [ ] **Regular security reviews** scheduled

### Ongoing Security
- [ ] **Regular updates** applied (monthly)
- [ ] **Security scans** automated (weekly)
- [ ] **Audit log reviews** (weekly)
- [ ] **Access reviews** (quarterly)
- [ ] **Incident response testing** (quarterly)
- [ ] **Security awareness training** (annually)

## Best Practices Summary

### 🔐 Credentials
- Use environment variables for secrets
- Never commit credentials to version control
- Rotate API keys regularly
- Use least-privilege access policies

### 🏗️ Infrastructure
- Deploy within secure network perimeter
- Use TLS for all external communications
- Enable audit logging and monitoring
- Implement proper backup and recovery

### 👥 Team Access
- Use role-based access control
- Require strong authentication
- Review access permissions regularly
- Train team on security procedures

### 🔍 Monitoring
- Monitor for failed authentications
- Alert on unusual agent activity
- Review audit logs regularly
- Track resource usage patterns

By following these security guidelines, Station can be safely deployed in production environments while maintaining the security posture your organization requires.