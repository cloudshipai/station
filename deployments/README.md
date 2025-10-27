# Station CICD Deployments

Station agents can run in any CICD platform that supports Docker containers. This directory contains integration guides and templates for various CICD platforms.

## Available Integrations

### ✅ Production Ready

- **[GitHub Actions](./github-actions/)** - Composite Action with 3-line integration
  - `uses: cloudshipai/station-action@v1`
  - Automatic PR comments, multiple agent support
  - Published to GitHub Marketplace

### 🚧 Templates Available

- **[GitLab CI](./gitlab-ci/)** - GitLab CI/CD templates
- **[CircleCI](./circleci/)** - CircleCI Orb configuration
- **[Jenkins](./jenkins/)** - Jenkins Pipeline examples
- **[Argo Workflows](./argo-workflows/)** - Argo Workflow templates
- **[Tekton](./tekton/)** - Tekton Task definitions
- **[Concourse](./concourse/)** - Concourse pipeline templates
- **[Dagger](./dagger/)** - Dagger module integration

## Quick Start

### Using Station Security Image

All integrations use the same Docker image:
```
ghcr.io/cloudshipai/station-security:latest
```

This image contains:
- 6 pre-configured security agents
- 97+ security tools (checkov, trivy, semgrep, gitleaks, etc.)
- Station CLI for agent execution

### Basic Command

```bash
stn agent run "<Agent Name>" "<Task Description>"
```

### Available Agents

1. **Infrastructure Security Auditor** - Terraform, K8s, Docker scanning
2. **PR Security Reviewer** - Code review for security issues
3. **Supply Chain Guardian** - SBOM + dependency scanning
4. **Deployment Security Gate** - Pre-deployment validation
5. **Security Improvement Advisor** - Security recommendations
6. **Security Metrics Reporter** - Security KPIs and reporting

### Required Environment Variables

- `OPENAI_API_KEY` - Required for AI analysis
- `STN_CLOUDSHIP_KEY` - Optional for telemetry/monitoring
- `PROJECT_ROOT` - Path to scan (default: `/workspace`)

## Integration Examples

### GitHub Actions (3 lines)
```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
```

### GitLab CI
```yaml
security-scan:
  image: ghcr.io/cloudshipai/station-security:latest
  script:
    - stn agent run "Infrastructure Security Auditor" "Scan for security issues"
```

### CircleCI
```yaml
jobs:
  security-scan:
    docker:
      - image: ghcr.io/cloudshipai/station-security:latest
    steps:
      - checkout
      - run: stn agent run "Infrastructure Security Auditor" "Scan for issues"
```

### Jenkins
```groovy
pipeline {
  agent {
    docker { image 'ghcr.io/cloudshipai/station-security:latest' }
  }
  stages {
    stage('Security') {
      steps {
        sh 'stn agent run "Infrastructure Security Auditor" "Scan for issues"'
      }
    }
  }
}
```

## Use Cases

### Security Scanning (on push/PR)
Run security agents on every code change to catch vulnerabilities early.

### FinOps Analysis (daily cron)
Schedule cost analysis and resource optimization agents to run daily.

### Compliance Auditing (weekly)
Regular compliance checks for SOC2, ISO27001, CIS benchmarks.

### Platform Monitoring (hourly)
Monitor platform health, SLAs, and operational metrics continuously.

## Architecture

```
CICD Platform
    ↓
Docker Container (ghcr.io/cloudshipai/station-security:latest)
    ↓
Station CLI (stn)
    ↓
Agent Execution (with 97+ tools)
    ↓
AI Analysis (OpenAI/Anthropic/Gemini)
    ↓
Results (PR comments, reports, metrics)
```

## Contributing

Each CICD integration should include:
1. `README.md` - Setup instructions and examples
2. Template files - Ready-to-use configuration
3. Examples - Common use cases (security, finops, compliance)

## Support

- **Documentation**: https://docs.cloudshipai.com
- **Issues**: https://github.com/cloudshipai/station/issues
- **Discord**: https://discord.gg/cloudshipai
