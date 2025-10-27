# Station + GitLab CI Integration

Run Station security agents in GitLab CI pipelines.

## Quick Start

Add to your `.gitlab-ci.yml`:

```yaml
security-scan:
  image: ghcr.io/cloudshipai/station-security:latest
  script:
    - stn agent run "Infrastructure Security Auditor" "Scan for security vulnerabilities"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
```

## Complete Example

```yaml
stages:
  - security
  - compliance

infrastructure-security:
  stage: security
  image: ghcr.io/cloudshipai/station-security:latest
  script:
    - stn agent run "Infrastructure Security Auditor" "Scan terraform, kubernetes, and docker configurations for security issues"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
    PROJECT_ROOT: /builds/$CI_PROJECT_PATH
  only:
    - merge_requests
    - main

supply-chain-security:
  stage: security
  image: ghcr.io/cloudshipai/station-security:latest
  script:
    - stn agent run "Supply Chain Guardian" "Generate SBOM and scan dependencies for vulnerabilities"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
  only:
    - merge_requests
    - main

compliance-audit:
  stage: compliance
  image: ghcr.io/cloudshipai/station-security:latest
  script:
    - stn agent run "Deployment Security Gate" "Validate compliance requirements before deployment"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
  only:
    - main
```

## Scheduled Scans

Run agents on a schedule for FinOps and compliance:

```yaml
daily-cost-analysis:
  image: ghcr.io/cloudshipai/station-security:latest
  script:
    - stn agent run "AWS Cost Analyzer" "Analyze AWS costs and identify optimization opportunities"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
    AWS_ACCESS_KEY_ID: $AWS_ACCESS_KEY_ID
    AWS_SECRET_ACCESS_KEY: $AWS_SECRET_ACCESS_KEY
  only:
    - schedules
```

## Setup

1. **Add CI/CD Variables** (Settings → CI/CD → Variables):
   - `OPENAI_API_KEY` - Your OpenAI API key
   - `STN_CLOUDSHIP_KEY` - (Optional) CloudShip telemetry key

2. **Copy template** to your repository as `.gitlab-ci.yml`

3. **Commit and push** - Pipeline runs automatically

## Available Agents

- `Infrastructure Security Auditor` - Terraform, K8s, Docker
- `PR Security Reviewer` - Code security review
- `Supply Chain Guardian` - SBOM + dependencies
- `Deployment Security Gate` - Pre-deployment checks
- `Security Improvement Advisor` - Recommendations
- `Security Metrics Reporter` - KPIs and metrics
