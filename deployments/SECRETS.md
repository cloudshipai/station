# Secrets Management Across CICD Platforms

This document explains how to configure secrets (API keys, credentials) for Station agents across all supported CICD platforms.

## Required Secrets

### Core Secrets

| Secret | Purpose | Required | Platform Variable Name |
|--------|---------|----------|----------------------|
| OpenAI API Key | AI model access for agent execution | Yes | `OPENAI_API_KEY` |
| CloudShip Registration Key | Telemetry and monitoring (optional) | No | `STN_CLOUDSHIP_KEY` |

### Optional Secrets (for specific agents)

| Secret | Use Case | Agents That Need It |
|--------|----------|-------------------|
| `AWS_ACCESS_KEY_ID` | AWS cost analysis | FinOps agents |
| `AWS_SECRET_ACCESS_KEY` | AWS access | FinOps agents |
| `GRAFANA_API_KEY` | Metrics monitoring | Platform agents |
| `GITHUB_TOKEN` | Repository analysis | Security/compliance agents |
| `KUBE_CONFIG` | Kubernetes access | Platform agents |

## Platform-Specific Setup

### GitHub Actions

**Location:** Repository Settings → Secrets and variables → Actions → New repository secret

**Setup:**
```yaml
# .github/workflows/security.yml
- uses: cloudshipai/station-action@v1
  with:
    agent: infrastructure-security
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
    STN_CLOUDSHIP_KEY: ${{ secrets.CLOUDSHIP_KEY }}  # Optional
```

**Secret Names:**
- `OPENAI_API_KEY` (required)
- `CLOUDSHIP_KEY` (optional - for telemetry)

### GitLab CI

**Location:** Project Settings → CI/CD → Variables

**Setup:**
```yaml
# .gitlab-ci.yml
analyze:
  image: ghcr.io/cloudshipai/station:latest
  script:
    - stn agent run "Code Reviewer" "Review the code"
  variables:
    OPENAI_API_KEY: $OPENAI_API_KEY
    STN_CLOUDSHIP_KEY: $STN_CLOUDSHIP_KEY  # Optional
```

**Variable Names:**
- `OPENAI_API_KEY` (required, protected, masked)
- `STN_CLOUDSHIP_KEY` (optional, protected, masked)

### Jenkins

**Location:** Manage Jenkins → Credentials → System → Global credentials

**Setup:**
```groovy
// Jenkinsfile
pipeline {
  environment {
    OPENAI_API_KEY = credentials('openai-api-key')
    STN_CLOUDSHIP_KEY = credentials('cloudship-key')  // Optional
  }
  stages {
    stage('Security Scan') {
      steps {
        sh 'stn agent run "Code Reviewer" "Review the code"'
      }
    }
  }
}
```

**Credential IDs:**
- `openai-api-key` (Secret text)
- `cloudship-key` (Secret text, optional)

### CircleCI

**Location:** Organization Settings → Contexts → Create Context

**Setup:**
```yaml
# .circleci/config.yml
workflows:
  security:
    jobs:
      - security-scan:
          context: station-security  # Contains secrets
```

**Context:** `station-security`
**Environment Variables:**
- `OPENAI_API_KEY` (required)
- `STN_CLOUDSHIP_KEY` (optional)

### Argo Workflows

**Location:** Kubernetes Secrets

**Setup:**
```bash
# Create secret
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  --from-literal=cloudship-key=$STN_CLOUDSHIP_KEY \
  -n security
```

**Workflow reference:**
```yaml
env:
- name: OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: openai-api-key
- name: STN_CLOUDSHIP_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: cloudship-key
```

### Tekton

**Location:** Kubernetes Secrets

**Setup:**
```bash
# Create secret
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  --from-literal=cloudship-key=$STN_CLOUDSHIP_KEY \
  -n security
```

**Task reference:**
```yaml
env:
- name: OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: openai-api-key
- name: STN_CLOUDSHIP_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: cloudship-key
```

## Adding New Secrets

To add support for a new secret (e.g., `DATADOG_API_KEY` for monitoring agents):

### 1. Update Platform Templates

**GitHub Actions** (`github-actions/action.yml`):
```yaml
inputs:
  datadog_api_key:
    description: 'Datadog API key for monitoring agents'
    required: false

# In Docker command:
if [ -n "${{ inputs.datadog_api_key }}" ]; then
  DOCKER_ENV_VARS="$DOCKER_ENV_VARS -e DATADOG_API_KEY=${{ inputs.datadog_api_key }}"
fi
```

**GitLab CI** (`.gitlab-ci.yml`):
```yaml
variables:
  DATADOG_API_KEY: $DATADOG_API_KEY
```

**Jenkins** (`Jenkinsfile`):
```groovy
environment {
  DATADOG_API_KEY = credentials('datadog-api-key')
}
```

**Kubernetes (Argo/Tekton)**:
```yaml
- name: DATADOG_API_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: datadog-api-key
```

### 2. Document in Platform README

Add to each platform's README.md:
```markdown
### Optional Secrets
- `DATADOG_API_KEY` - Required for monitoring agents
```

### 3. Update This Document

Add row to "Optional Secrets" table above.

## Security Best Practices

### Secret Rotation
1. Update secret in platform-specific secret store
2. Re-run workflows - new secret is picked up automatically
3. No code changes needed

### Secret Scoping
- **GitHub Actions**: Use repository secrets for single repos, organization secrets for multiple repos
- **GitLab CI**: Use project variables for specific projects, group variables for multiple projects
- **Jenkins**: Use credential domains to scope access
- **CircleCI**: Use contexts to group and share secrets
- **Kubernetes**: Use namespaces to isolate secrets

### Masking
All platforms automatically mask secrets in logs. Never log secrets directly.

## Troubleshooting

### "OPENAI_API_KEY not set"
- Verify secret is created in platform's secret store
- Check secret name matches expected name
- For Kubernetes: verify secret exists in correct namespace

### "Permission denied" errors
- **GitHub Actions**: Add required `permissions` to job
- **Jenkins**: Check credential access in job configuration
- **Kubernetes**: Verify ServiceAccount has access to secrets

### CloudShip key not working
- CloudShip key is optional - agents work without it
- If key is set but invalid, check key format in CloudShip dashboard
- Verify environment variable name: `STN_CLOUDSHIP_KEY`

## Future Secrets

As new agent types are added (FinOps, Platform Engineering, Compliance), follow the pattern above:

1. Add input/variable to platform templates
2. Pass as environment variable to Docker container
3. Document in platform READMEs
4. Update this document

Station's architecture makes adding new secrets straightforward across all platforms.
