# Station + CircleCI

Run Station agents in CircleCI pipelines.

## Quick Start

Add to `.circleci/config.yml`:

```yaml
version: 2.1

jobs:
  analyze:
    docker:
      - image: ghcr.io/cloudshipai/station:latest
    steps:
      - checkout
      - run:
          name: Run Agent
          command: stn agent run "Code Reviewer" "Review the code"

workflows:
  main:
    jobs:
      - analyze:
          context: station
```

## Complete Example

```yaml
version: 2.1

jobs:
  code-review:
    docker:
      - image: ghcr.io/cloudshipai/station:latest
    steps:
      - checkout
      - run:
          name: Code Review
          command: stn agent run "Code Reviewer" "Review code for bugs and best practices"

  security-scan:
    docker:
      - image: ghcr.io/cloudshipai/station:latest
    steps:
      - checkout
      - run:
          name: Security Scan
          command: stn agent run "Security Analyst" "Scan for security vulnerabilities"

workflows:
  version: 2
  
  pr-checks:
    jobs:
      - code-review:
          context: station
      - security-scan:
          context: station
          requires:
            - code-review

  daily-analysis:
    triggers:
      - schedule:
          cron: "0 9 * * *"
          filters:
            branches:
              only: main
    jobs:
      - code-review:
          context: station
```

## Using Different AI Providers

### Anthropic Claude

Create a context with:
- `ANTHROPIC_API_KEY`
- `STN_AI_PROVIDER=anthropic`
- `STN_AI_MODEL=claude-3-5-sonnet-20241022`

### Google Gemini

Create a context with:
- `GOOGLE_API_KEY`
- `STN_AI_PROVIDER=gemini`
- `STN_AI_MODEL=gemini-2.0-flash-exp`

## Setup

1. **Create Context** (Organization Settings > Contexts):
   - Name: `station`
   - Variables: `OPENAI_API_KEY`

2. **Create your agents** in `environments/default/template.json`

3. **Add config** to `.circleci/config.yml`

4. **Push** - pipeline runs automatically
