# Station GitHub Action

Run AI agents from Station bundles in GitHub Actions workflows.

## Quick Start

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Code Reviewer'
    task: 'Review this PR for bugs and security issues'
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

## Inputs

### Required

| Input | Description |
|-------|-------------|
| `agent` | Name of the agent to run (as defined in your bundle/environment) |
| `task` | Task description for the agent to execute |

### Bundle Source

Pick one method to provide your agents:

| Input | Description |
|-------|-------------|
| `environment` | Local environment name (default: `default`). Looks for `./environments/<name>/` in your repo |
| `bundle-url` | URL to download a Station bundle (.tar.gz) |
| `bundle-id` | CloudShip bundle ID to download. Requires `cloudship-api-key` |
| `cloudship-api-key` | CloudShip API key for bundle downloads |

### AI Provider

| Input | Description | Default |
|-------|-------------|---------|
| `provider` | AI provider: `openai`, `anthropic`, `gemini`, `ollama` | `openai` |
| `model` | Model name (e.g., `gpt-4o`, `claude-3-5-sonnet-20241022`, `gemini-2.0-flash-exp`) | Provider default |
| `base-url` | Custom API endpoint (for Azure, Ollama, etc.) | - |

### Execution

| Input | Description | Default |
|-------|-------------|---------|
| `timeout` | Execution timeout in seconds | `300` |
| `max-steps` | Maximum agent steps | `50` |
| `comment-pr` | Post results as PR comment | `false` |
| `fail-on-error` | Fail workflow if agent fails | `true` |
| `version` | Station CLI version | `latest` |
| `debug` | Enable debug logging | `false` |

## API Keys

Pass your API key via the `env:` block (NOT via `with:`):

```yaml
env:
  OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
  # OR
  ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
  # OR
  GOOGLE_API_KEY: ${{ secrets.GOOGLE_API_KEY }}
```

**Why not via `with:`?** Secrets should be passed as environment variables, not action inputs. This is a GitHub Actions security best practice.

## Examples

### OpenAI (Default)

```yaml
name: Code Review
on: [pull_request]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: cloudshipai/station-action@v1
        with:
          agent: 'Code Reviewer'
          task: 'Review this PR for bugs, security issues, and best practices'
          comment-pr: true
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Anthropic Claude

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Security Analyst'
    task: 'Scan for security vulnerabilities'
    provider: anthropic
    model: claude-3-5-sonnet-20241022
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

### Google Gemini

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Documentation Generator'
    task: 'Generate API documentation for changed files'
    provider: gemini
    model: gemini-2.0-flash-exp
  env:
    GOOGLE_API_KEY: ${{ secrets.GOOGLE_API_KEY }}
```

### Azure OpenAI

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Test Generator'
    task: 'Generate unit tests for new code'
    provider: openai
    model: gpt-4o
    base-url: https://YOUR-RESOURCE.openai.azure.com/
  env:
    OPENAI_API_KEY: ${{ secrets.AZURE_OPENAI_KEY }}
```

### Ollama (Self-Hosted)

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Code Assistant'
    task: 'Explain the architecture'
    provider: ollama
    model: llama3.2
    base-url: http://localhost:11434
```

### From Bundle URL

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'FinOps Analyzer'
    task: 'Analyze cloud costs'
    bundle-url: https://releases.example.com/finops-bundle-v1.0.0.tar.gz
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### From CloudShip Registry

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Security Scanner'
    task: 'Run security audit'
    bundle-id: '550e8400-e29b-41d4-a716-446655440000'
    cloudship-api-key: ${{ secrets.CLOUDSHIP_API_KEY }}
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### Scheduled Analysis

```yaml
name: Weekly Security Scan
on:
  schedule:
    - cron: '0 2 * * 1'  # Monday 2 AM

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: cloudshipai/station-action@v1
        with:
          agent: 'Security Auditor'
          task: 'Comprehensive security scan of the entire codebase'
          timeout: 600
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

## Outputs

| Output | Description |
|--------|-------------|
| `result` | Agent execution result (JSON) |
| `exit-code` | Exit code (0 = success) |
| `run-id` | Station run ID |

```yaml
- uses: cloudshipai/station-action@v1
  id: agent
  with:
    agent: 'Analyzer'
    task: 'Analyze code'

- run: echo "Exit code: ${{ steps.agent.outputs.exit-code }}"
```

## Additional Environment Variables

You can pass any Station configuration via environment variables:

```yaml
env:
  OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
  
  # CloudShip telemetry
  STN_CLOUDSHIP_ENABLED: true
  STN_CLOUDSHIP_KEY: ${{ secrets.CLOUDSHIP_KEY }}
  
  # Telemetry/Observability
  STN_TELEMETRY_ENABLED: true
  STN_TELEMETRY_ENDPOINT: https://otel.example.com
  
  # Debug
  STN_DEBUG: true
```

See [Station Configuration](https://github.com/cloudshipai/station#configuration) for all options.

## Repository Structure

For local environments, structure your repo like this:

```
your-repo/
├── environments/
│   ├── default/           # Default environment
│   │   └── template.json
│   └── production/        # Production environment
│       └── template.json
└── .github/
    └── workflows/
        └── station.yml
```

## Troubleshooting

### "No API key found"

Make sure you're passing the API key via `env:`, not `with:`:

```yaml
# WRONG
with:
  openai-api-key: ${{ secrets.OPENAI_API_KEY }}

# CORRECT
env:
  OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

### "Agent not found"

Check that your agent name matches exactly what's in your bundle/environment. Agent names are case-sensitive.

### "Environment directory not found"

Ensure your environment exists at `./environments/<name>/` relative to the repo root.

## Reusable Actions

For more control, use the individual reusable actions:

| Action | Description |
|--------|-------------|
| `cloudshipai/station/.github/actions/setup-station@main` | Just install Station CLI |
| `cloudshipai/station/.github/actions/build-bundle@main` | Create bundles from environments |
| `cloudshipai/station/.github/actions/build-image@main` | Build Docker images |

## License

Apache 2.0 - See [LICENSE](../../LICENSE)
