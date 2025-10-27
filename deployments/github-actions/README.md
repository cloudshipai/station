# CloudShip AI Security Scanner - GitHub Action

Add AI-powered security scanning to your CI/CD pipeline in 3 lines.

📚 **Documentation**: [Quick Start](QUICK_START.md) | [Deployment Guide](DEPLOYMENT_GUIDE.md) | [Examples](examples/)

## Quick Start

```yaml
- uses: actions/checkout@v4
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
```

That's it! Your infrastructure will be scanned for security vulnerabilities.

## Features

- 🤖 **6 Specialized AI Agents** - Infrastructure, PR Review, Supply Chain, Deployment Gates, Security Advisor, Metrics
- 🛠️ **97+ Security Tools** - Checkov, TFLint, Semgrep, Syft, Trivy, and many more
- 🧠 **AI-Powered Analysis** - Not just tool output, actual intelligent insights and prioritization
- 📊 **Automatic PR Comments** - Get security findings directly in your pull requests
- 🔧 **Fully Customizable** - Override tasks, agents, and workflows as needed

## Available Agents

### Infrastructure Security Auditor (`infrastructure-security`)
Scans Terraform, Kubernetes, and Docker configurations for misconfigurations, secrets, and compliance violations.

**Use when:**
- You have IaC files (Terraform, K8s manifests, Dockerfiles)
- You want to check for infrastructure misconfigurations
- You need compliance validation (CIS, NIST, etc.)

### PR Security Reviewer (`pr-security`)
Analyzes pull request code changes for security vulnerabilities, focusing on diffs and new code.

**Use when:**
- Reviewing pull requests
- You want automated security code review
- You need to catch secrets, SQL injection, XSS, etc.

### Supply Chain Guardian (`supply-chain`)
Generates SBOMs and scans dependencies for known vulnerabilities and malicious packages.

**Use when:**
- You have package dependencies (npm, pip, go.mod, etc.)
- You need SBOM generation
- You want to track dependency vulnerabilities

### Deployment Security Gate (`deployment-gate`)
Pre-deployment validation with compliance attestation and blocking capabilities.

**Use when:**
- Before deploying to production
- You need security sign-off for deployments
- You want compliance attestation

### Security Improvement Advisor (`security-advisor`)
Proactive security recommendations and modernization suggestions.

**Use when:**
- You want to improve security posture
- You need guidance on security best practices
- You're doing security hardening

### Security Metrics Reporter (`security-metrics`)
Periodic security metrics aggregation with KPI tracking and executive reporting.

**Use when:**
- You need security dashboards
- You want trend analysis
- You're generating compliance reports

## Usage Examples

### Basic Infrastructure Scanning

```yaml
name: Security Scan
on: [pull_request, push]

jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cloudshipai/station-action@v1
        with:
          agent: infrastructure-security
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### PR Security Review

```yaml
name: PR Security Check
on: pull_request

jobs:
  security-review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cloudshipai/station-action@v1
        with:
          agent: pr-security
          comment_pr: true
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Supply Chain + Infrastructure (Multiple Agents)

```yaml
name: Comprehensive Security
on: [pull_request]

jobs:
  infrastructure:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cloudshipai/station-action@v1
        with:
          agent: infrastructure-security
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}

  supply-chain:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cloudshipai/station-action@v1
        with:
          agent: supply-chain
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Custom Task Override

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
    task: "Focus only on AWS security groups and check for overly permissive ingress rules"
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Block Deployment on Critical Issues

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: deployment-gate
    fail_on_critical: true  # Will fail the workflow if critical issues found
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Run All Agents

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: all  # Runs Infrastructure, PR Review, and Supply Chain agents
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### With CloudShip AI Telemetry (Optional)

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
    STN_CLOUDSHIP_KEY: ${{ secrets.CLOUDSHIP_KEY }}  # Optional: for telemetry and monitoring
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `agent` | Agent to run (infrastructure-security, pr-security, supply-chain, deployment-gate, security-advisor, security-metrics, or all) | No | `infrastructure-security` |
| `task` | Custom task description to override default agent behavior | No | Agent-specific default |
| `openai_api_key` | OpenAI API key (can also use OPENAI_API_KEY env var) | No | From env var |
| `cloudship_key` | CloudShip AI registration key for telemetry and monitoring (can also use STN_CLOUDSHIP_KEY env var) | No | From env var |
| `comment_pr` | Post results as PR comment (true/false) | No | `true` |
| `fail_on_critical` | Fail workflow if critical issues found (true/false) | No | `false` |
| `docker_image` | Override default CloudShip security image | No | `ghcr.io/cloudshipai/station-security:latest` |

## Outputs

| Output | Description |
|--------|-------------|
| `exit_code` | Exit code from security scan (0 = clean, non-zero = issues found) |

## Setup

### 1. Add OpenAI API Key

Go to your repository **Settings** → **Secrets and variables** → **Actions** → **New repository secret**

- Name: `OPENAI_API_KEY`
- Value: Your OpenAI API key

### 2. Add Workflow

Create `.github/workflows/security.yml`:

```yaml
name: CloudShip Security
on: [pull_request, push]

jobs:
  security-scan:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write  # Required for PR comments
    steps:
      - uses: actions/checkout@v4
      - uses: cloudshipai/station-action@v1
        with:
          agent: infrastructure-security
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### 3. Push and Test

Commit the workflow file and create a PR to see it in action!

## How It Works

1. **Checkout**: Your code is checked out to the runner
2. **Agent Selection**: Based on your input, the appropriate agent is selected
3. **Docker Execution**: The CloudShip Station container runs with:
   - Your workspace mounted at `/workspace`
   - Docker socket for container scanning (privileged mode)
   - OpenAI API key for AI analysis
4. **Analysis**: The agent:
   - Syncs MCP tools (97+ security tools)
   - Runs relevant security scanners
   - AI analyzes and prioritizes findings
   - Generates actionable recommendations
5. **Results**:
   - PR comment with summary (if enabled)
   - Detailed logs in workflow run
   - Optional workflow failure on critical issues

## What Makes This Different?

### vs Traditional Security Scanners
- ✅ **Multi-tool orchestration** (97 tools vs 1-2 per scanner)
- ✅ **AI-powered analysis** (prioritization and context, not just rule matching)
- ✅ **Single integration** (one action vs 5-10 different scanners)
- ✅ **Natural language customization** (describe what you want, no config files)

### vs GitHub Security Features
- ✅ **More coverage** (IaC, containers, code, dependencies all in one)
- ✅ **Customizable agents** (not locked into predefined rules)
- ✅ **AI analysis** (intelligent insights, not just CVE lists)

### vs GitHub Actions Marketplace
- ✅ **One action for everything** (vs 10+ different actions)
- ✅ **Consistent interface** (same inputs/outputs for all agents)
- ✅ **AI-powered** (actual intelligence, not just tool wrappers)

## Pricing

The action itself is **free and open source**.

You'll need:
- ✅ **OpenAI API key** (pay-as-you-go, typically $0.10-$0.50 per scan)
- ✅ **GitHub Actions minutes** (free tier: 2,000 min/month for private repos)

## Roadmap

- [ ] GitHub App (zero-config installation)
- [ ] Custom bundle support (bring your own agents)
- [ ] GitLab CI adapter
- [ ] CircleCI orb
- [ ] Dagger module
- [ ] Self-hosted option (no API key required)

## Support

- 📖 **Documentation**: https://docs.cloudshipai.com
- 💬 **Discord**: https://discord.gg/cloudshipai
- 🐛 **Issues**: https://github.com/cloudshipai/station-action/issues
- 📧 **Email**: support@cloudshipai.com

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

**Built with ❤️ by the CloudShip AI team**

Add AI-powered security scanning to your pipeline in 3 lines:
```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
```
