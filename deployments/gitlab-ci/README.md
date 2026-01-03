# Station + GitLab CI

Run Station agents in GitLab CI pipelines.

## Quick Start

Add to `.gitlab-ci.yml`:

```yaml
analyze:
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Code Reviewer" "Review the merge request changes"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
```

## Complete Example

```yaml
stages:
  - analyze
  - report

code-review:
  stage: analyze
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Code Reviewer" "Review code changes for bugs and best practices"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"

security-scan:
  stage: analyze
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Security Analyst" "Scan for security vulnerabilities"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == "main"

daily-report:
  stage: report
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Report Generator" "Generate daily summary report"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
  rules:
    - if: $CI_PIPELINE_SOURCE == "schedule"
```

## Using Different AI Providers

### Anthropic Claude

```yaml
analyze:
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Code Reviewer" "Review the code"
  variables:
    ANTHROPIC_API_KEY: $ANTHROPIC_API_KEY
    STN_AI_PROVIDER: anthropic
    STN_AI_MODEL: claude-3-5-sonnet-20241022
```

### Google Gemini

```yaml
analyze:
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Code Reviewer" "Review the code"
  variables:
    GOOGLE_API_KEY: $GOOGLE_API_KEY
    STN_AI_PROVIDER: gemini
    STN_AI_MODEL: gemini-2.0-flash-exp
```

## Setup

1. **Add CI/CD Variables** (Settings > CI/CD > Variables):
   - `OPENAI_API_KEY` (or `ANTHROPIC_API_KEY` / `GOOGLE_API_KEY`)

2. **Create your agents** in `environments/default/template.json`

3. **Add pipeline config** to `.gitlab-ci.yml`

4. **Push** - pipeline runs automatically

## Loading Bundles from URL

```yaml
analyze:
  image: ghcr.io/cloudshipai/station:latest
  script:
    - curl -fsSL $BUNDLE_URL -o bundle.tar.gz
    - tar -xzf bundle.tar.gz
    - stn init --config ./config.yaml --yes
    - stn agent run "My Agent" "Run the task"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
    BUNDLE_URL: https://example.com/my-bundle.tar.gz
```
