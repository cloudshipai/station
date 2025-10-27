# Station + CircleCI Integration

Run Station security agents in CircleCI pipelines.

## Quick Start

Add to your `.circleci/config.yml`:

```yaml
version: 2.1

jobs:
  security-scan:
    docker:
      - image: ghcr.io/cloudshipai/station-security:latest
    steps:
      - checkout
      - run:
          name: Security Scan
          command: stn agent run "Infrastructure Security Auditor" "Scan for security issues"

workflows:
  security:
    jobs:
      - security-scan:
          context: station-security
```

## Complete Example

```yaml
version: 2.1

jobs:
  infrastructure-security:
    docker:
      - image: ghcr.io/cloudshipai/station-security:latest
    steps:
      - checkout
      - run:
          name: Infrastructure Security Scan
          command: |
            stn agent run "Infrastructure Security Auditor" \
              "Scan terraform, kubernetes, and docker for security vulnerabilities"

  supply-chain-security:
    docker:
      - image: ghcr.io/cloudshipai/station-security:latest
    steps:
      - checkout
      - run:
          name: Supply Chain Scan
          command: |
            stn agent run "Supply Chain Guardian" \
              "Generate SBOM and scan dependencies for vulnerabilities"

  deployment-gate:
    docker:
      - image: ghcr.io/cloudshipai/station-security:latest
    steps:
      - checkout
      - run:
          name: Deployment Security Gate
          command: |
            stn agent run "Deployment Security Gate" \
              "Validate security posture before deployment"

workflows:
  version: 2
  security-pipeline:
    jobs:
      - infrastructure-security:
          context: station-security
          filters:
            branches:
              only:
                - main
                - develop
      - supply-chain-security:
          context: station-security
      - deployment-gate:
          context: station-security
          requires:
            - infrastructure-security
            - supply-chain-security
          filters:
            branches:
              only: main

  daily-analysis:
    triggers:
      - schedule:
          cron: "0 9 * * *"
          filters:
            branches:
              only: main
    jobs:
      - cost-analysis:
          context: station-finops
```

## Setup

1. **Create Context** (Organization Settings → Contexts):
   - Name: `station-security`
   - Add environment variables:
     - `OPENAI_API_KEY`
     - `STN_CLOUDSHIP_KEY` (optional)

2. **Copy config** to `.circleci/config.yml`

3. **Commit and push** - Pipeline runs automatically

## Orb (Future)

Station will provide an official CircleCI Orb:

```yaml
version: 2.1

orbs:
  station: cloudshipai/station@1.0.0

workflows:
  security:
    jobs:
      - station/scan:
          agent: infrastructure-security
          context: station-security
```
