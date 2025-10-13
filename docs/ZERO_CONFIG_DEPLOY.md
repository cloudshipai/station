# Zero-Configuration Bundle Deployment

This guide demonstrates how to deploy Station bundles with zero manual configuration using environment variables for all secrets and configuration.

## Overview

Station bundles are portable environment packages that include:
- **Agent definitions** (.prompt files)
- **MCP server configurations** (template.json files)
- **NO SECRETS** - variables.yml is always excluded from bundles

All configuration can be injected at runtime via environment variables, enabling true zero-config deployments perfect for Docker, Kubernetes, and CI/CD pipelines.

## Quick Start

### Docker Compose Example

```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudship-io/station:latest
    environment:
      # MCP Template Variables
      - PROJECT_ROOT=/workspace
      - API_KEY=${API_KEY}
      - DATABASE_URL=${DATABASE_URL}
      - GITHUB_TOKEN=${GITHUB_TOKEN}

      # Station AI Configuration
      - OPENAI_API_KEY=${OPENAI_API_KEY}

    volumes:
      - ./workspace:/workspace
      - station-data:/home/station/.config/station

    ports:
      - "8585:8585"

    command: >
      sh -c "
        stn init &&
        stn bundle install https://api.cloudshipai.com/bundles/security-scanner.tar.gz prod &&
        stn sync prod &&
        stn serve
      "

volumes:
  station-data:
```

### Kubernetes Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station
spec:
  replicas: 1
  selector:
    matchLabels:
      app: station
  template:
    metadata:
      labels:
        app: station
    spec:
      containers:
      - name: station
        image: ghcr.io/cloudship-io/station:latest
        ports:
        - containerPort: 8585
        env:
        - name: PROJECT_ROOT
          value: "/workspace"
        - name: API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: api-key
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        command:
        - sh
        - -c
        - |
          stn init && \
          stn bundle install https://api.cloudshipai.com/bundles/security-scanner.tar.gz prod && \
          stn sync prod && \
          stn serve
        volumeMounts:
        - name: workspace
          mountPath: /workspace
        - name: station-data
          mountPath: /home/station/.config/station
      volumes:
      - name: workspace
        persistentVolumeClaim:
          claimName: workspace-pvc
      - name: station-data
        persistentVolumeClaim:
          claimName: station-data-pvc
```

## How It Works

### 1. Bundle Creation (Secrets Never Included)

```bash
stn bundle create production
```

**What gets bundled:**
- ✅ Agent .prompt files
- ✅ MCP server templates (template.json)
- ❌ variables.yml (ALWAYS EXCLUDED)

**Code Reference:** `internal/services/bundle_service.go:54-56`
```go
// Skip variables.yml files
if strings.HasSuffix(file, "variables.yml") || strings.HasSuffix(file, "variables.yaml") {
    return nil
}
```

### 2. Bundle Installation with Database Integration

```bash
stn bundle install https://api.example.com/bundle.tar.gz production
```

**What happens:**
1. Downloads/copies bundle to `~/.config/station/bundles/`
2. Creates environment entry in database (id, name, description, timestamps)
3. Extracts agents and MCP configs to `~/.config/station/environments/production/`
4. Ready for sync with environment variable resolution

**Code Reference:** `internal/services/bundle_service.go:238-274`

### 3. Environment Variable Resolution During Sync

```bash
PROJECT_ROOT=/workspace API_KEY=secret stn sync production
```

**Variable Resolution Order:**
1. Loads environment's `variables.yml` (if exists)
2. **Loads ALL system environment variables**
3. Filters out system vars (PATH, HOME, USER, etc.)
4. Uses variables to render MCP templates
5. Connects MCP servers with rendered configuration

**Code Reference:** `internal/services/template_variable_service.go:75-98`
```go
// 3. Load ALL environment variables as potential template variables
// This enables zero-config deploys where bundles don't include variables.yml
// but all configuration comes from container environment variables
envVarCount := 0
for _, envPair := range os.Environ() {
    parts := strings.SplitN(envPair, "=", 2)
    if len(parts) == 2 {
        key := parts[0]
        value := parts[1]

        // Skip internal/system environment variables
        if tvs.isSystemEnvVar(key) {
            continue
        }

        // Environment variables override variables.yml values
        existingVars[key] = value
        envVarCount++
    }
}
```

## Complete Workflow Example

### Step 1: Create Bundle from Existing Environment

```bash
# In your development Station instance
stn bundle create production --output prod-bundle.tar.gz
```

**Output:**
```
🗂️  Bundling environment: production
📂 Source path: /home/station/.config/station/environments/production
📋 Found:
   🤖 6 agent(s): [terraform-auditor.prompt container-scanner.prompt ...]
   🔧 1 MCP config(s): [template.json]
✅ Bundle created: prod-bundle.tar.gz
📊 Size: 5432 bytes
```

### Step 2: Deploy to Fresh Station Instance

```bash
# Pull latest Station image
docker pull ghcr.io/cloudship-io/station:latest

# Deploy with environment variables
docker run -d \
  --name station-prod \
  -e PROJECT_ROOT=/workspace \
  -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
  -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
  -e OPENAI_API_KEY=${OPENAI_API_KEY} \
  -v $(pwd)/workspace:/workspace \
  -v station-prod-data:/home/station/.config/station \
  -p 8585:8585 \
  ghcr.io/cloudship-io/station:latest \
  sh -c "
    stn init && \
    stn bundle install /bundle.tar.gz production && \
    stn sync production && \
    stn serve
  "
```

### Step 3: Verify Deployment

```bash
# Check environment was created
docker exec station-prod sqlite3 /home/station/.config/station/station.db \
  "SELECT id, name, created_at FROM environments WHERE name='production';"

# Expected output:
# 1|production|2025-10-13 00:31:56

# Check MCP servers synced
docker exec station-prod sqlite3 /home/station/.config/station/station.db \
  "SELECT name, command FROM mcp_servers WHERE environment_id=1;"

# Check tools discovered
docker exec station-prod sqlite3 /home/station/.config/station/station.db \
  "SELECT COUNT(*) FROM mcp_tools WHERE mcp_server_id IN
   (SELECT id FROM mcp_servers WHERE environment_id=1);"
```

## MCP Template Variable Examples

### Filesystem MCP Server

**template.json:**
```json
{
  "name": "production",
  "description": "Production environment",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    }
  }
}
```

**Environment variables:**
```bash
PROJECT_ROOT=/workspace
```

**Rendered configuration:**
```json
{
  "command": "npx",
  "args": [
    "-y",
    "@modelcontextprotocol/server-filesystem@latest",
    "/workspace"
  ]
}
```

### GitHub MCP Server

**template.json:**
```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-github@latest"
      ],
      "env": {
        "GITHUB_TOKEN": "{{ .GITHUB_TOKEN }}"
      }
    }
  }
}
```

**Environment variables:**
```bash
GITHUB_TOKEN=ghp_abc123xyz789
```

### Database MCP Server

**template.json:**
```json
{
  "mcpServers": {
    "postgres": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-postgres@latest",
        "{{ .DATABASE_URL }}"
      ]
    }
  }
}
```

**Environment variables:**
```bash
DATABASE_URL=postgresql://user:pass@db.example.com:5432/production
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Deploy Station Bundle

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4

    - name: Deploy to Production
      env:
        PROJECT_ROOT: /workspace
        API_KEY: ${{ secrets.API_KEY }}
        DATABASE_URL: ${{ secrets.DATABASE_URL }}
        OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
      run: |
        docker run -d \
          --name station-prod \
          -e PROJECT_ROOT="$PROJECT_ROOT" \
          -e API_KEY="$API_KEY" \
          -e DATABASE_URL="$DATABASE_URL" \
          -e OPENAI_API_KEY="$OPENAI_API_KEY" \
          -v $(pwd):/workspace \
          ghcr.io/cloudship-io/station:latest \
          sh -c "
            stn init && \
            stn bundle install https://api.cloudshipai.com/bundles/prod.tar.gz prod && \
            stn sync prod && \
            stn serve
          "

        # Wait for Station to be healthy
        timeout 60 sh -c 'until curl -f http://localhost:8585/health; do sleep 1; done'

        echo "✅ Station deployed successfully"
```

### GitLab CI

```yaml
deploy:
  stage: deploy
  image: docker:latest
  services:
    - docker:dind
  variables:
    PROJECT_ROOT: /workspace
  script:
    - |
      docker run -d \
        --name station-prod \
        -e PROJECT_ROOT="$PROJECT_ROOT" \
        -e API_KEY="$API_KEY" \
        -e DATABASE_URL="$DATABASE_URL" \
        -e OPENAI_API_KEY="$OPENAI_API_KEY" \
        ghcr.io/cloudship-io/station:latest \
        sh -c "
          stn init && \
          stn bundle install $BUNDLE_URL prod && \
          stn sync prod && \
          stn serve
        "
```

## Security Best Practices

### 1. Never Commit Secrets

```bash
# ❌ DON'T: Hardcode secrets in template.json
{
  "env": {
    "API_KEY": "hardcoded-secret-123"
  }
}

# ✅ DO: Use template variables
{
  "env": {
    "API_KEY": "{{ .API_KEY }}"
  }
}
```

### 2. Use Secret Management

**Kubernetes Secrets:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: station-secrets
type: Opaque
data:
  api-key: YWJjMTIzCg==  # base64 encoded
  openai-api-key: c2stdGVzdC1rZXk=
```

**Docker Secrets:**
```bash
echo "abc123" | docker secret create api_key -

docker service create \
  --name station \
  --secret api_key \
  --env API_KEY_FILE=/run/secrets/api_key \
  ghcr.io/cloudship-io/station:latest
```

### 3. Rotate Credentials Regularly

```bash
# Update secret in Kubernetes
kubectl create secret generic station-secrets \
  --from-literal=api-key=new-secret-456 \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart pods to pick up new secret
kubectl rollout restart deployment/station
```

## Troubleshooting

### Bundle Install Fails: Environment Already Exists

**Error:**
```
Error: bundle installation failed: Environment 'default' already exists
```

**Solution - Option 1:** Use a different environment name
```bash
stn bundle install bundle.tar.gz production-v2
```

**Solution - Option 2:** Delete existing environment first
```bash
stn env delete default
stn bundle install bundle.tar.gz default
```

### Sync Fails: Template Rendering Error

**Error:**
```
Template rendering failed: map has no entry for key "PROJECT_ROOT"
```

**Solution:** Ensure environment variable is set before sync
```bash
export PROJECT_ROOT=/workspace
stn sync production
```

Or inline:
```bash
PROJECT_ROOT=/workspace stn sync production
```

### MCP Servers Not Connecting

**Check 1:** Verify environment variables loaded
```bash
stn sync production --verbose
```

Look for:
```
Loaded 6 environment variables for template resolution
```

**Check 2:** Verify rendered configuration
Check the template was rendered with actual values (not `<no value>`).

**Check 3:** Verify MCP server command exists
```bash
npx -y @modelcontextprotocol/server-filesystem@latest /workspace
```

## System Variables Filtered Out

The following environment variables are automatically filtered and NOT used for template resolution:

- `PATH`, `HOME`, `USER`, `SHELL`, `PWD`, `OLDPWD`, `TERM`
- `LANG`, `LC_*`, `TMPDIR`, `TMP`, `TEMP`
- `HOSTNAME`, `HOSTTYPE`, `OSTYPE`
- `SHLVL`, `UID`, `GID`, `LOGNAME`
- `DISPLAY`, `EDITOR`, `PAGER`
- `SSH_*`, `GPG_*`, `XDG_*`
- `DEBIAN_*`, `UBUNTU_*`

**Code Reference:** `internal/services/template_variable_service.go:329-350`

## Testing Zero-Config Deployment

### End-to-End Test

```bash
# 1. Create test bundle
stn bundle create default --output test-bundle.tar.gz

# 2. Start fresh container with env vars
docker run -d \
  --name station-test \
  -e PROJECT_ROOT=/workspace \
  -e TEST_VAR=hello \
  -e OPENAI_API_KEY=sk-test \
  -v $(pwd)/test-bundle.tar.gz:/bundle.tar.gz:ro \
  -p 8686:8585 \
  ghcr.io/cloudship-io/station:latest \
  tail -f /dev/null

# 3. Install and sync
docker exec station-test stn init
docker exec station-test stn bundle install /bundle.tar.gz test-env
docker exec -e PROJECT_ROOT=/workspace station-test stn sync test-env

# 4. Verify database entries
docker exec station-test sqlite3 /home/station/.config/station/station.db \
  "SELECT * FROM environments WHERE name='test-env';"

docker exec station-test sqlite3 /home/station/.config/station/station.db \
  "SELECT COUNT(*) FROM mcp_servers WHERE environment_id=1;"

# 5. Cleanup
docker rm -f station-test
```

## Known Limitations

### Agent Validation Errors (Non-Blocking)

During sync, you may see agent validation errors:
```
❌ VALIDATION ERROR: Agent 'aws-cost-spike-analyzer': failed to extract input schema:
   invalid input schema: invalid JSON Schema: Invalid type. Expected: string/array of strings, given: type
```

**Status:** These are schema validation warnings and do NOT block:
- ✅ MCP server synchronization
- ✅ Tool discovery
- ✅ Environment creation
- ✅ Bundle installation

The agents ARE created in the database and will function correctly. This is a known issue with JSON Schema validation of complex output schemas and will be fixed in a future release.

## Advanced Topics

### Custom Bundle Registry

Host your own bundle registry:

**Nginx Configuration:**
```nginx
server {
    listen 80;
    server_name bundles.example.com;

    location /bundles/ {
        root /var/www;
        add_header Access-Control-Allow-Origin *;
        add_header Content-Type application/gzip;
    }
}
```

**Usage:**
```bash
stn bundle install https://bundles.example.com/bundles/prod.tar.gz production
```

### Multi-Environment Deployment

Deploy multiple environments in one container:

```bash
docker run -d \
  --name station-multi \
  -e DEV_PROJECT_ROOT=/workspace/dev \
  -e PROD_PROJECT_ROOT=/workspace/prod \
  -e DEV_API_KEY=${DEV_API_KEY} \
  -e PROD_API_KEY=${PROD_API_KEY} \
  ghcr.io/cloudship-io/station:latest \
  sh -c "
    stn init && \
    DEV_PROJECT_ROOT=/workspace/dev stn bundle install https://api.example.com/dev.tar.gz dev && \
    PROD_PROJECT_ROOT=/workspace/prod stn bundle install https://api.example.com/prod.tar.gz prod && \
    stn sync dev && \
    stn sync prod && \
    stn serve
  "
```

## Summary

Zero-configuration deployment with Station bundles provides:

✅ **Security**: Secrets never included in bundles
✅ **Portability**: Same bundle works across environments
✅ **Simplicity**: No manual configuration files needed
✅ **Automation**: Perfect for CI/CD pipelines
✅ **Flexibility**: Override any configuration at runtime

For questions or issues, see:
- [Station Documentation](https://docs.station.dev)
- [GitHub Issues](https://github.com/cloudshipai/station/issues)
- [Bundle Registry](https://registry.cloudshipai.com)
