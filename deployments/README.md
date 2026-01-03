# Station CI/CD Deployments

Run Station agents in any CI/CD platform that supports Docker containers.

## Available Integrations

### GitHub Actions (Recommended)

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'My Agent'
    task: 'Analyze the codebase'
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

See [GitHub Actions README](./github-actions/) for full documentation.

### Other Platforms

| Platform | Status | Documentation |
|----------|--------|---------------|
| [GitLab CI](./gitlab-ci/) | Template | `.gitlab-ci.yml` examples |
| [CircleCI](./circleci/) | Template | `config.yml` examples |
| [Jenkins](./jenkins/) | Template | Jenkinsfile examples |
| [Argo Workflows](./argo-workflows/) | Template | Workflow manifests |
| [Tekton](./tekton/) | Template | Task/Pipeline definitions |
| [Kubernetes](./kubernetes/) | Template | Deployment manifests |

## Quick Start

All platforms use the same Docker image and CLI:

```bash
docker run -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -v $(pwd):/workspace \
  ghcr.io/cloudshipai/station:latest \
  stn agent run "My Agent" "Analyze the code"
```

## Using Your Own Agents

Station runs YOUR agents from YOUR bundles. Define agents in your environment:

```
your-repo/
├── environments/
│   └── default/
│       └── template.json    # Your agent definitions
└── .gitlab-ci.yml           # CI/CD config
```

Example `template.json`:
```json
{
  "agents": [
    {
      "name": "Code Reviewer",
      "description": "Reviews code for bugs and best practices",
      "model": "gpt-4o",
      "prompt": "You are an expert code reviewer..."
    }
  ]
}
```

## Environment Variables

### Required (pick one provider)

| Variable | Provider |
|----------|----------|
| `OPENAI_API_KEY` | OpenAI (GPT-4, GPT-4o) |
| `ANTHROPIC_API_KEY` | Anthropic (Claude) |
| `GOOGLE_API_KEY` | Google (Gemini) |

### Optional

| Variable | Description |
|----------|-------------|
| `STN_AI_PROVIDER` | Override provider: `openai`, `anthropic`, `gemini` |
| `STN_AI_MODEL` | Override model name |
| `STN_AI_BASE_URL` | Custom API endpoint (Azure, Ollama) |
| `STN_CLOUDSHIP_KEY` | CloudShip telemetry key |
| `STN_DEBUG` | Enable debug logging |

## Platform Examples

### GitLab CI

```yaml
agent-task:
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Code Reviewer" "Review the merge request"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
```

### CircleCI

```yaml
jobs:
  analyze:
    docker:
      - image: ghcr.io/cloudshipai/station:latest
    steps:
      - checkout
      - run: stn agent run "Code Reviewer" "Review the code"
```

### Jenkins

```groovy
pipeline {
  agent {
    docker { image 'ghcr.io/cloudshipai/station:latest' }
  }
  stages {
    stage('Analyze') {
      steps {
        sh 'stn agent run "Code Reviewer" "Review the code"'
      }
    }
  }
}
```

## Use Cases

| Use Case | Trigger | Example Agent |
|----------|---------|---------------|
| Code Review | PR/MR | Code Reviewer, Security Analyst |
| Cost Analysis | Daily cron | FinOps Analyzer, Cost Optimizer |
| Compliance | Weekly cron | Compliance Auditor, Policy Checker |
| Monitoring | Hourly | Health Monitor, SLA Tracker |

## Architecture

```
CI/CD Platform
    ↓
Docker: ghcr.io/cloudshipai/station:latest
    ↓
Station CLI: stn agent run "<name>" "<task>"
    ↓
Your Bundle/Environment (agents, tools, prompts)
    ↓
AI Provider (OpenAI/Anthropic/Gemini)
    ↓
Results
```

## Support

- **Documentation**: https://github.com/cloudshipai/station
- **Issues**: https://github.com/cloudshipai/station/issues
- **Discord**: https://discord.gg/cloudshipai
