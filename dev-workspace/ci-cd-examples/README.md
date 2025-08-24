# Station CI/CD Integration Examples

This directory contains practical examples of integrating Station with CI/CD pipelines for automated security analysis, SBOM generation, and infrastructure scanning.

## 🏗️ Integration Patterns

### Option 1: Agent-as-a-Service Pattern
**Directory:** `agent-as-service/`  
**Best for:** Simple integration, one-off analysis, small teams

- Uses Docker containers with pre-configured agents
- Direct `stn agent run` commands in CI workflows
- Self-contained with all dependencies included

**Usage:**
```bash
docker run --rm \
  -v ${{ github.workspace }}:/workspace \
  -e OPENAI_API_KEY=${{ secrets.OPENAI_API_KEY }} \
  station-ci:latest \
  agent run terraform-security-agent --input "Analyze for security issues"
```

### Option 2: Station CI Server
**Directory:** `station-ci-server/`  
**Best for:** Persistent analysis, multiple teams, complex workflows

- Long-running Station server with REST API
- Workspace management and job queuing
- Centralized agent management and reporting

**Usage:**
```bash
curl -X POST "http://station-ci:8585/api/v1/agents/ci-terraform-analyzer/execute" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"input": "Analyze workspace for security issues"}'
```

### Option 3: Dagger Integration
**Directory:** `dagger-integration/`  
**Best for:** Complex orchestration, parallel analysis, programmatic control

- Dagger module for programmatic container orchestration
- Parallel analysis execution with consolidated reporting  
- Type-safe pipeline definitions

**Usage:**
```bash
dagger call security-scan --source=. --openai-key=env:OPENAI_API_KEY
```

## 🚀 Quick Start

### 1. Build Station Base Image
```bash
# From Station project root
stn build base
```

### 2. Choose Your Integration Pattern

#### Agent-as-a-Service (Recommended for getting started)
```bash
cd agent-as-service/
export OPENAI_API_KEY="your-key"
export ENCRYPTION_KEY=$(openssl rand -hex 32)

# Test the security analysis
docker run --rm \
  -v $(pwd):/workspace \
  -v $(pwd)/config.yml:/root/.config/station/config.yaml \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -e ENCRYPTION_KEY=$ENCRYPTION_KEY \
  station-base:latest \
  agent run terraform-security-agent \
  --input "Test analysis of current directory"
```

#### Station CI Server
```bash
cd station-ci-server/
export OPENAI_API_KEY="your-key"
export STATION_ENCRYPTION_KEY=$(openssl rand -hex 32)

# Start the server
docker-compose up -d

# Test API
curl http://localhost:8585/api/v1/environments
```

#### Dagger Integration
```bash
cd dagger-integration/
export OPENAI_API_KEY="your-key"
export ENCRYPTION_KEY=$(openssl rand -hex 32)

# Run security scan
dagger call security-scan --source=../../.. \
  --openai-key=env:OPENAI_API_KEY \
  --encryption-key=env:ENCRYPTION_KEY
```

## 🛠️ Agent Configurations

### Pre-built Agents

#### `terraform-security-agent`
- **Tools:** Checkov, Trivy, TFlint
- **Purpose:** Infrastructure security scanning
- **Output:** Security report with severity ratings

#### `sbom-security-agent`  
- **Tools:** Syft, Grype, OSV Scanner
- **Purpose:** Dependency analysis and vulnerability scanning
- **Output:** SBOM files and vulnerability reports

#### `ci-terraform-analyzer`
- **Tools:** Checkov, Infracost, TFlint
- **Purpose:** Comprehensive Terraform analysis
- **Output:** Security, cost, and quality analysis

## 📊 Report Formats

All agents generate structured JSON reports:

```json
{
  "analysis_type": "terraform-security",
  "timestamp": "2025-01-24T17:43:00Z",
  "repository": "company/infrastructure", 
  "commit": "abc123...",
  "findings": {
    "critical": 0,
    "high": 2,
    "medium": 5,
    "low": 12
  },
  "cost_estimate": {
    "monthly": 1250.00,
    "optimizations": ["Use t3.medium instead of t3.large"]
  },
  "recommendations": [
    "Enable encryption for S3 buckets",
    "Add security group rules"
  ]
}
```

## 🔧 Environment Variables

### Required
- `OPENAI_API_KEY` - OpenAI API key for GPT models
- `ENCRYPTION_KEY` - Station encryption key (generate with `openssl rand -hex 32`)

### Optional
- `ANTHROPIC_API_KEY` - Anthropic API key for Claude models
- `GITHUB_TOKEN` - GitHub token for repository access
- `STATION_CI_TOKEN` - API token for Station CI server access

## 🏢 Team Integration Guide

### Small Teams (2-10 developers)
1. Use **Agent-as-a-Service** pattern
2. Add GitHub Actions workflow to your repository
3. Configure secrets in GitHub repository settings
4. Agents run directly in CI containers

### Medium Teams (10-50 developers)
1. Deploy **Station CI Server** on shared infrastructure
2. Configure centralized agent library
3. Use REST API from multiple repositories
4. Implement workspace isolation

### Large Organizations (50+ developers)
1. Use **Dagger Integration** for advanced orchestration
2. Deploy multiple Station instances per team/environment
3. Implement RBAC and audit logging
4. Create custom agent libraries per domain

## 🔒 Security Best Practices

1. **Secrets Management**
   - Use GitHub Secrets or equivalent
   - Rotate API keys regularly
   - Never commit secrets to code

2. **Container Security**
   - Use minimal base images
   - Regular security updates
   - Scan container images

3. **Access Control**
   - Implement authentication for CI server
   - Use least-privilege access tokens
   - Audit API access logs

## 🧪 Testing Your Integration

### Test Agent Execution
```bash
# Test that agents can run
stn agent run security-scanner --input "Test run"

# Verify tools are available  
stn agent run terraform-analyzer --input "Test Terraform tools"
```

### Test CI Pipeline Locally
```bash
# Simulate GitHub Actions locally with act
act -j terraform-analysis \
  --secret OPENAI_API_KEY="$OPENAI_API_KEY" \
  --secret STATION_ENCRYPTION_KEY="$ENCRYPTION_KEY"
```

### Validate Reports
```bash
# Check report format
jq '.findings.critical' reports/security-report.json
jq '.cost_estimate.monthly' reports/terraform-analysis.json
```

## 📚 Advanced Usage

### Custom Agent Development
```json
{
  "name": "custom-security-scanner",
  "description": "Company-specific security analysis",
  "prompt": "You are a security agent for ACME Corp...",
  "max_steps": 15,
  "tools": ["__checkov_scan_directory", "__custom_policy_check"]
}
```

### Multi-Environment Deployment
```yaml
# Deploy different configurations per environment
environments:
  development:
    config: dev-config.yml
    agents: ["basic-security", "code-quality"]
  
  production:  
    config: prod-config.yml
    agents: ["full-security", "compliance", "cost-analysis"]
```

### Integration with Other Tools
```bash
# Export to SARIF for GitHub Security tab
jq '.findings | to_sarif' security-report.json > results.sarif

# Send to Slack on failures
curl -X POST -H 'Content-type: application/json' \
  --data '{"text":"Security scan failed: '"$CRITICAL"' critical issues"}' \
  $SLACK_WEBHOOK_URL
```

## 🐛 Troubleshooting

### Common Issues

1. **Agent execution timeouts**
   - Increase `max_steps` in agent configuration
   - Check resource limits in CI environment

2. **MCP tool connection failures**  
   - Verify all required tools are available in container
   - Check network connectivity to external services

3. **Report generation failures**
   - Ensure output directories exist and are writable
   - Verify JSON schema compliance

### Debug Commands
```bash
# Check agent status
stn agent list

# Test tool availability
stn agent run test-agent --input "List available tools"

# View detailed logs
docker logs station-ci-server --tail 100 -f
```

## 🤝 Contributing

To add new integration patterns:

1. Create new directory under `ci-cd-examples/`
2. Include `config.yml`, `variables.yml`, and agent definitions
3. Add GitHub Actions workflow example
4. Update this README with usage instructions
5. Test with realistic scenarios

## 📖 Additional Resources

- [Station Documentation](../../docs/)
- [Docker Build System](../../docs/DOCKER_BUILD_SYSTEM.md)
- [Agent Development Guide](../../docs/agents/)
- [MCP Tool Reference](../../docs/mcp-tools/)

---

*These examples provide production-ready templates for integrating Station into CI/CD pipelines. Customize the agent configurations and workflows based on your specific security and analysis requirements.*