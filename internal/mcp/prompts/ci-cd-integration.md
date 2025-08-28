# CI/CD Agent Integration Guide

This prompt resource provides complete guidance for integrating Station AI agents into CI/CD pipelines with Docker containers, GitHub Actions, and automated workflows.

## Quick Start - Docker CI/CD Integration

### Prerequisites
- Docker with socket access (`/var/run/docker.sock`)
- Station environment with agents and security tools
- OpenAI API key for AI agent execution

### Basic Setup

```bash
# Use the pre-built Station image with DevOps Security environment
docker run \
  -v $(pwd):/workspace:ro \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -e ENCRYPTION_KEY=$STATION_ENCRYPTION_KEY \
  epuerta18/station-default:latest \
  bash -c "
    stn agent run 'Security Scanner' 'Comprehensive security analysis of /workspace'
  "
```

## Production GitHub Actions Workflow

Here's the proven workflow used in the agents-cicd repository:

```yaml
name: Station Security Analysis
on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  security-scan:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Run Station Security Analysis
        run: |
          docker run \
            -v $(pwd):/workspace:ro \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -e OPENAI_API_KEY=${{ secrets.OPENAI_API_KEY }} \
            -e ENCRYPTION_KEY=${{ secrets.STATION_ENCRYPTION_KEY }} \
            epuerta18/station-default:latest \
            bash -c "
              echo '🚀 Starting Station AI Security Analysis...'
              stn agent run 'Security Scanner' 'Analyze /workspace for security vulnerabilities using checkov and security scanning tools. Focus on critical findings and provide actionable recommendations.'
              stn agent run 'Terraform Auditor' 'Analyze any Terraform files in /workspace for best practices and security issues using tflint validation tools.'
              echo '✅ Security analysis complete!'
            "

      - name: Comment PR with Results
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: '🤖 **Station AI Security Analysis Complete**\n\nSecurity agents have analyzed this PR for vulnerabilities and best practices. Check the workflow logs for detailed findings and recommendations.'
            })
```

## Available Pre-Built Agents

The `epuerta18/station-default:latest` container includes these pre-configured agents:

### Security Scanner
- **Description**: Scans repositories for security vulnerabilities using checkov security tools
- **Tools**: `__checkov_scan_directory`, `__checkov_scan_file`, `__checkov_scan_secrets`
- **Use Cases**: Code security analysis, vulnerability detection, secret scanning

### Terraform Auditor  
- **Description**: Analyzes Terraform infrastructure as code using tflint for validation and best practices
- **Tools**: `__tflint_check`, `__tflint_init`, `__tflint_lint`
- **Use Cases**: Terraform validation, IaC best practices, infrastructure security

## Step-by-Step Container Setup

### 1. Build Custom Environment (Optional)

If you need custom agents or tools:

```bash
# Build Station environment with custom agents
stn build env production --provider openai --model gpt-5 --ship

# This creates station-production:latest with:
# ✅ Station CLI pre-installed
# ✅ Ship CLI v0.7.3+ with 80+ security tools  
# ✅ Pre-configured agents with tool assignments
# ✅ MCP server configurations
# ✅ Docker CLI for containerized tools
```

### 2. Test Locally

```bash
# Test the environment locally
docker run \
  -v $(pwd):/workspace:ro \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -e ENCRYPTION_KEY="your-32-character-encryption-key" \
  epuerta18/station-default:latest \
  bash -c "
    stn agent list
    stn tools list
    stn agent run 'Security Scanner' 'Scan current directory for security issues'
  "
```

### 3. CI/CD Integration Options

#### Option A: Direct Agent Execution (Recommended)
```yaml
- name: Security Analysis
  run: |
    docker run \
      -v $(pwd):/workspace:ro \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e OPENAI_API_KEY=${{ secrets.OPENAI_API_KEY }} \
      -e ENCRYPTION_KEY=${{ secrets.STATION_ENCRYPTION_KEY }} \
      epuerta18/station-default:latest \
      stn agent run "Security Scanner" "Analyze /workspace for vulnerabilities"
```

#### Option B: Multiple Agents in Parallel
```yaml
- name: Comprehensive Analysis
  run: |
    docker run \
      -v $(pwd):/workspace:ro \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e OPENAI_API_KEY=${{ secrets.OPENAI_API_KEY }} \
      -e ENCRYPTION_KEY=${{ secrets.STATION_ENCRYPTION_KEY }} \
      epuerta18/station-default:latest \
      bash -c "
        stn agent run 'Security Scanner' 'Security analysis of /workspace' &
        stn agent run 'Terraform Auditor' 'Terraform validation for /workspace' &
        wait
        echo 'All agents completed!'
      "
```

#### Option C: API Integration
```yaml
- name: API-Based Execution
  run: |
    # Start Station server in background
    docker run -d \
      --name station-server \
      -v $(pwd):/workspace:ro \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e OPENAI_API_KEY=${{ secrets.OPENAI_API_KEY }} \
      -e ENCRYPTION_KEY=${{ secrets.STATION_ENCRYPTION_KEY }} \
      -p 8585:8585 \
      epuerta18/station-default:latest \
      stn server
    
    sleep 10
    
    # Execute agents via API
    curl -X POST http://localhost:8585/api/v1/agents/1/queue \
      -H "Content-Type: application/json" \
      -d '{"input": "Analyze /workspace for security issues"}'
```

## Environment Variables

Required environment variables for CI/CD:

```bash
# Required - OpenAI API key for AI agent execution
OPENAI_API_KEY="sk-..."

# Required - Station encryption key (32+ characters)  
ENCRYPTION_KEY="your-station-encryption-key-32-chars"

# Optional - Station configuration
STATION_CONFIG_ROOT="/app/environment"
STATION_DB_PATH="/app/data/station.db"
STATION_DEBUG="false"
```

## Bundle Installation in CI/CD

You can also install bundles dynamically in CI/CD:

```yaml
- name: Install Security Bundle
  run: |
    # Install bundle from GitHub registry
    curl -X POST http://localhost:8585/bundles/install \
      -H "Content-Type: application/json" \
      -d '{
        "bundle_location": "https://github.com/cloudshipai/registry/releases/latest/download/devops-security-bundle.tar.gz",
        "environment_name": "cicd-security",
        "source": "remote"
      }'
    
    # Run agents from installed bundle
    stn agent run "Security Scanner" "Analyze codebase for vulnerabilities"
```

## Real-World Example: agents-cicd Repository

The [agents-cicd](https://github.com/cloudshipai/agents-cicd) repository demonstrates:

1. **Dockerfile for Station environment**
2. **GitHub Actions workflow with Station agents**
3. **PR comment integration with results**
4. **Multi-agent security analysis pipeline**
5. **Docker socket mounting for Ship CLI tools**

Key workflow from agents-cicd:

```yaml
name: Station CI Analysis
on: [push, pull_request]

jobs:
  station-analysis:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Station Security Analysis
        run: |
          docker run \
            -v $(pwd):/workspace:ro \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -e OPENAI_API_KEY=${{ secrets.OPENAI_API_KEY }} \
            -e ENCRYPTION_KEY=${{ secrets.STATION_ENCRYPTION_KEY }} \
            epuerta18/station-default:latest \
            bash -c "
              stn agent run 'Security Scanner' 'Comprehensive security scan of /workspace directory focusing on vulnerabilities, secrets, and misconfigurations'
              stn agent run 'Terraform Auditor' 'Analyze any Terraform files for security and best practices'
            "

      - name: PR Comment
        if: github.event_name == 'pull_request'
        run: echo "Security analysis complete! Check logs for findings."
```

## Troubleshooting

### Common Issues and Solutions

**Agent not found:**
```bash
# Check agents are loaded
docker exec <container> stn agent list
```

**Docker socket permission denied:**
```bash
# Ensure Docker socket is mounted
-v /var/run/docker.sock:/var/run/docker.sock
```

**MCP tools not working:**
```bash
# Verify tools are available
docker exec <container> stn tools list
```

**API key not working:**
```bash
# Check environment variables
docker exec <container> env | grep OPENAI_API_KEY
```

## Advanced Integration: Bundle Creation

Create your own bundles from CI/CD environments:

```bash
# Create bundle from current environment
stn bundle production --output production-security.tar.gz

# Upload to registry (example)
curl -X POST https://api.github.com/repos/myorg/bundles/releases \
  -H "Authorization: token $GITHUB_TOKEN" \
  -d '{"tag_name": "v1.0", "name": "Production Security Bundle"}'
```

## Security Best Practices

- ✅ **Never commit API keys** - Use GitHub Secrets
- ✅ **Use unique encryption keys** - Generate per environment
- ✅ **Mount Docker socket read-only when possible**
- ✅ **Validate all inputs and file paths**  
- ✅ **Enable audit logging for compliance**
- ✅ **Use least-privilege container permissions**

## Performance Optimization

- ✅ **Cache base images** - Build once, reuse everywhere
- ✅ **Run agents in parallel** - Use `&` and `wait` in bash
- ✅ **Set resource limits** - Control CPU/memory usage
- ✅ **Clean up containers** - Remove after execution

This guide provides everything needed to successfully integrate Station AI agents into production CI/CD pipelines with Docker containers and automated workflows.