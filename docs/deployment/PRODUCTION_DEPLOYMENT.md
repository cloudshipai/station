# Station Production Deployment Guide

**Last Updated**: November 11, 2025  
**Station Version**: v0.1.0+

---

## Overview

This guide covers everything you need to deploy Station agents to production environments, from single servers to Kubernetes clusters. It focuses on **what's not already documented** in the existing guides while providing a complete deployment checklist.

**What This Guide Covers:**
- ✅ Production-ready bundle creation workflow
- ✅ Server deployment strategies
- ✅ CICD integration patterns
- ✅ Security best practices
- ✅ Troubleshooting common issues

**Related Documentation:**
- [Zero-Config Deployments](../station/zero-config-deployments.md) - IAM role-based Docker deployments
- [Docker Compose Deployments](../station/docker-compose-deployments.md) - Containerized deployment patterns
- [Deployment Modes](../station/deployment-modes.md) - Choosing the right deployment mode
- [CICD Integrations](../deployments/README.md) - Platform-specific CICD guides

---

## Current State: What Works Today

### ✅ **Bundle System**
- `stn bundle create <environment>` creates production-ready `.tar.gz` bundles
- Bundles include: agents, MCP configs, manifest with metadata
- **Excludes** `variables.yml` (secrets not baked into bundles)
- Small size: ~7KB for 10 agents

### ✅ **Zero-Config Docker**
- `examples/zero-config-deploy/` provides working template
- Auto-installs bundles from `/bundles` directory
- Auto-syncs environments on startup
- Supports ENV variable injection for secrets

### ✅ **Docker Image**
- Production image at `ghcr.io/cloudshipai/station:latest`
- Base: Alpine Linux with Litestream for state replication
- Health checks and graceful shutdown built-in

### ✅ **CICD Integrations**
- GitHub Actions composite action (Marketplace ready)
- Templates for GitLab CI, CircleCI, Jenkins, Argo, Tekton
- All use same Docker image: `ghcr.io/cloudshipai/station-security:latest`

---

## Production Deployment Workflow

### Phase 1: Development & Testing

#### 1.1 Create Environment Locally

```bash
# Create and configure environment
stn env create production
cd ~/.config/station/environments/production

# Add MCP servers (or sync from existing)
# Configure agents with prompt files
# Test agents locally

stn agent run <agent-name> "test task" --env production
```

#### 1.2 Test with Faker Tools (if applicable)

If your agents use faker MCP servers (AI-generated tool responses):

```bash
# Faker tools work out of the box - no external dependencies
# Perfect for testing agent logic without real API calls

stn agent run cost-analyzer "Analyze fake AWS cost data" --env production
```

**Faker Benefits:**
- ✅ No AWS/GCP credentials needed for testing
- ✅ Realistic AI-generated responses
- ✅ Fast iteration without API rate limits
- ✅ Smaller Docker images (no Node.js/Python needed)

#### 1.3 Create Bundle

```bash
# Create production bundle
stn bundle create production --output production-agents.tar.gz

# Verify bundle contents
tar -tzf production-agents.tar.gz
# Should show:
# - manifest.json (metadata)
# - template.json OR individual MCP .json files
# - agents/*.prompt files
```

**Bundle Validation Checklist:**
- [ ] All agent `.prompt` files included
- [ ] MCP server configs present
- [ ] Manifest has correct agent metadata
- [ ] No `variables.yml` (secrets) included
- [ ] Bundle size reasonable (<50MB)

---

### Phase 2: Server Deployment

Station supports multiple deployment targets. Choose based on your infrastructure:

#### Option A: Docker Compose (Simplest)

**Use When:**
- Single server or small-scale deployment
- Docker available on target server
- Want simple container management

**Deployment Steps:**

```bash
# 1. On your local machine - copy files to server
scp production-agents.tar.gz server:/opt/station/bundles/
scp examples/zero-config-deploy/docker-compose.yml server:/opt/station/
scp examples/zero-config-deploy/.env.example server:/opt/station/.env

# 2. SSH to server
ssh server

# 3. Edit .env with production secrets
cd /opt/station
vim .env
# Set: OPENAI_API_KEY, AWS_ACCESS_KEY_ID, etc.

# 4. Deploy
docker-compose up -d

# 5. Verify deployment
docker-compose logs -f station
curl http://localhost:8585/health

# 6. Test agent execution
docker-compose exec station stn agent list
docker-compose exec station stn agent run <agent-name> "test task"
```

**Production docker-compose.yml Enhancements:**

```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-prod
    ports:
      - "8585:8585"  # API/UI
      - "8586:8586"  # MCP
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - AWS_REGION=${AWS_REGION:-us-east-1}
    volumes:
      - ./bundles:/bundles:ro           # Read-only bundles
      - station-data:/root/.config/station  # Persistent data
    command: >
      sh -c "
        if [ ! -f /root/.config/station/config.yaml ]; then
          stn init --provider openai --model gpt-4o-mini --yes
        fi &&
        for bundle in /bundles/*.tar.gz; do
          bundle_name=\$(basename \"\$bundle\" .tar.gz) &&
          stn bundle install \"\$bundle\" \"\$bundle_name\" &&
          stn sync \"\$bundle_name\" -i=false
        done &&
        stn serve
      "
    restart: always  # Auto-restart on failure
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"

volumes:
  station-data:
    driver: local
```

**Key Production Changes:**
- `restart: always` - Auto-restart on crash
- Increased health check retries (5 vs 3)
- Longer `start_period` (60s vs 40s) for safer startup
- Log rotation configured (`max-size: 100m`, `max-file: 10`)
- Bundles mounted read-only (`:ro`)

#### Option B: Kubernetes (Scalable)

**Use When:**
- Multi-environment deployments (dev/staging/prod)
- Need horizontal scaling
- Already using Kubernetes

**Prerequisites:**
- Kubernetes cluster available
- `kubectl` configured
- Docker image accessible from cluster

**Deployment Files:**

Create `k8s/` directory with the following manifests:

**1. Namespace (`namespace.yaml`):**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: station-prod
```

**2. ConfigMap for Bundles (`configmap.yaml`):**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: station-bundles
  namespace: station-prod
binaryData:
  production-agents.tar.gz: <base64-encoded-bundle>
```

Generate bundle ConfigMap:
```bash
kubectl create configmap station-bundles \
  --from-file=production-agents.tar.gz \
  --namespace=station-prod \
  --dry-run=client -o yaml > k8s/configmap.yaml
```

**3. Secret for API Keys (`secret.yaml`):**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: station-secrets
  namespace: station-prod
type: Opaque
stringData:
  openai-api-key: "sk-your-key-here"  # Replace in production
  aws-access-key-id: "AKIA..."
  aws-secret-access-key: "..."
```

**Create secret from command (more secure):**
```bash
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key="$OPENAI_API_KEY" \
  --from-literal=aws-access-key-id="$AWS_ACCESS_KEY_ID" \
  --from-literal=aws-secret-access-key="$AWS_SECRET_ACCESS_KEY" \
  --namespace=station-prod
```

**4. Deployment (`deployment.yaml`):**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station
  namespace: station-prod
spec:
  replicas: 2  # For high availability
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
        image: ghcr.io/cloudshipai/station:latest
        ports:
        - containerPort: 8585
          name: api
        - containerPort: 8586
          name: mcp
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: aws-access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: aws-secret-access-key
        volumeMounts:
        - name: bundles
          mountPath: /bundles
          readOnly: true
        - name: station-data
          mountPath: /root/.config/station
        command:
        - sh
        - -c
        - |
          if [ ! -f /root/.config/station/config.yaml ]; then
            stn init --provider openai --model gpt-4o-mini --yes
          fi &&
          for bundle in /bundles/*.tar.gz; do
            bundle_name=$(basename "$bundle" .tar.gz) &&
            stn bundle install "$bundle" "$bundle_name" &&
            stn sync "$bundle_name" -i=false
          done &&
          stn serve
        livenessProbe:
          httpGet:
            path: /health
            port: 8585
          initialDelaySeconds: 60
          periodSeconds: 30
          timeoutSeconds: 10
          failureThreshold: 5
        readinessProbe:
          httpGet:
            path: /health
            port: 8585
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
      volumes:
      - name: bundles
        configMap:
          name: station-bundles
      - name: station-data
        persistentVolumeClaim:
          claimName: station-data-pvc

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: station-data-pvc
  namespace: station-prod
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

**5. Service (`service.yaml`):**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: station
  namespace: station-prod
spec:
  selector:
    app: station
  ports:
  - name: api
    port: 8585
    targetPort: 8585
  - name: mcp
    port: 8586
    targetPort: 8586
  type: LoadBalancer  # or ClusterIP if using Ingress
```

**Deploy to Kubernetes:**
```bash
# Apply all manifests
kubectl apply -f k8s/

# Check deployment status
kubectl get pods -n station-prod
kubectl get svc -n station-prod

# View logs
kubectl logs -n station-prod -l app=station -f

# Test agent execution
kubectl exec -n station-prod deployment/station -- \
  stn agent run <agent-name> "test task"
```

#### Option C: AWS ECS/Fargate (Serverless Containers)

**Use When:**
- AWS-native deployment
- Want serverless container management
- Need auto-scaling

**Full guide available in:** [docs/station/zero-config-deployments.md](../station/zero-config-deployments.md#pattern-2-ecsfargate-with-task-roles)

**Quick Steps:**
1. Create ECS task definition with Station image
2. Attach IAM task role for AWS API access
3. Store secrets in AWS Secrets Manager
4. Deploy ECS service with desired count
5. Use Application Load Balancer for HA

#### Option D: Bare Metal/VM (Traditional Server)

**Use When:**
- No container infrastructure
- Legacy server environment
- Maximum control over deployment

**Installation:**

```bash
# 1. Install Station binary
curl -sSL https://install.station.dev | bash
# Or download from releases:
# https://github.com/cloudshipai/station/releases

# 2. Verify installation
stn version

# 3. Create systemd service
sudo tee /etc/systemd/system/station.service > /dev/null <<EOF
[Unit]
Description=Station Agent Platform
After=network.target

[Service]
Type=simple
User=station
Group=station
WorkingDirectory=/opt/station
Environment="OPENAI_API_KEY=<your-key>"
Environment="PATH=/usr/local/bin:/usr/bin:/bin"
ExecStart=/usr/local/bin/stn serve --port 8585
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# 4. Create station user
sudo useradd -r -s /bin/bash -d /opt/station station
sudo mkdir -p /opt/station/bundles
sudo chown -R station:station /opt/station

# 5. Copy bundle to server
sudo cp production-agents.tar.gz /opt/station/bundles/
sudo chown station:station /opt/station/bundles/production-agents.tar.gz

# 6. Initialize Station (as station user)
sudo -u station bash -c "
  export OPENAI_API_KEY='<your-key>' &&
  stn init --provider openai --model gpt-4o-mini --yes &&
  stn bundle install /opt/station/bundles/production-agents.tar.gz production &&
  stn sync production -i=false
"

# 7. Start service
sudo systemctl daemon-reload
sudo systemctl enable station
sudo systemctl start station

# 8. Check status
sudo systemctl status station
sudo journalctl -u station -f

# 9. Test
curl http://localhost:8585/health
```

**Production Hardening:**
- Run as dedicated user (not root)
- Use systemd for auto-restart
- Store secrets in `/etc/station/secrets` (restricted permissions)
- Enable firewall rules (only expose 8585/8586)
- Set up log rotation: `/etc/logrotate.d/station`

---

### Phase 3: CICD Integration

#### GitHub Actions Deployment

**Use Case**: Deploy bundle to server on every main branch push

**Workflow** (`.github/workflows/deploy-agents.yml`):
```yaml
name: Deploy Station Agents

on:
  push:
    branches: [main]
    paths:
      - 'agents/**'
      - '.github/workflows/deploy-agents.yml'

jobs:
  deploy:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Install Station CLI
      run: |
        curl -sSL https://install.station.dev | bash
        echo "$HOME/.local/bin" >> $GITHUB_PATH
    
    - name: Create Bundle
      run: |
        # Assumes you have agents defined locally in agents/
        stn bundle create production --output production-agents.tar.gz
      env:
        OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
    
    - name: Upload Bundle to Server
      env:
        SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
      run: |
        mkdir -p ~/.ssh
        echo "$SSH_PRIVATE_KEY" > ~/.ssh/id_rsa
        chmod 600 ~/.ssh/id_rsa
        ssh-keyscan -H ${{ secrets.SERVER_HOST }} >> ~/.ssh/known_hosts
        
        # Copy bundle to server
        scp production-agents.tar.gz \
          ${{ secrets.SERVER_USER }}@${{ secrets.SERVER_HOST }}:/opt/station/bundles/
    
    - name: Deploy on Server
      env:
        SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
      run: |
        ssh ${{ secrets.SERVER_USER }}@${{ secrets.SERVER_HOST }} << 'EOF'
          cd /opt/station
          # Backup current bundle
          mv bundles/production-agents.tar.gz bundles/production-agents.tar.gz.bak || true
          mv bundles/production-agents.tar.gz.1 bundles/production-agents.tar.gz
          
          # Restart Station to load new bundle
          docker-compose restart station
          
          # Wait for health check
          sleep 30
          curl -f http://localhost:8585/health || exit 1
          
          echo "✅ Deployment successful!"
        EOF
```

**Required Secrets** (Settings → Secrets):
- `OPENAI_API_KEY` - For bundle creation testing
- `SSH_PRIVATE_KEY` - SSH key for server access
- `SERVER_HOST` - Deployment server hostname
- `SERVER_USER` - SSH username

#### GitLab CI Deployment

**Use Case**: Multi-environment deployment (dev/staging/prod)

**Pipeline** (`.gitlab-ci.yml`):
```yaml
stages:
  - build
  - test
  - deploy

variables:
  STATION_IMAGE: "ghcr.io/cloudshipai/station:latest"

# Build bundle from agents
build-bundle:
  stage: build
  image: ${STATION_IMAGE}
  script:
    - stn bundle create production --output production-agents.tar.gz
  artifacts:
    paths:
      - production-agents.tar.gz
    expire_in: 1 week
  only:
    - main

# Test bundle validity
test-bundle:
  stage: test
  image: ${STATION_IMAGE}
  script:
    - tar -tzf production-agents.tar.gz
    - echo "Bundle contents validated"
  dependencies:
    - build-bundle
  only:
    - main

# Deploy to staging
deploy-staging:
  stage: deploy
  image: ${STATION_IMAGE}
  environment:
    name: staging
    url: https://station-staging.example.com
  script:
    - |
      # Install SSH key
      mkdir -p ~/.ssh
      echo "$STAGING_SSH_KEY" > ~/.ssh/id_rsa
      chmod 600 ~/.ssh/id_rsa
      
      # Deploy bundle
      scp production-agents.tar.gz staging-server:/opt/station/bundles/
      ssh staging-server 'cd /opt/station && docker-compose restart station'
  dependencies:
    - build-bundle
  only:
    - main

# Deploy to production (manual)
deploy-production:
  stage: deploy
  image: ${STATION_IMAGE}
  environment:
    name: production
    url: https://station.example.com
  script:
    - |
      scp production-agents.tar.gz prod-server:/opt/station/bundles/
      ssh prod-server 'cd /opt/station && docker-compose restart station'
  dependencies:
    - build-bundle
  when: manual  # Require manual approval
  only:
    - main
```

**Required CI/CD Variables** (Settings → CI/CD → Variables):
- `STAGING_SSH_KEY` - SSH key for staging server
- `PRODUCTION_SSH_KEY` - SSH key for production server

---

### Phase 4: Multi-Environment Strategy

**Scenario**: Same agents, different configurations for dev/staging/prod

#### Strategy 1: Environment Variables (Recommended)

**Single bundle, environment-specific env vars:**

```bash
# Development
OPENAI_API_KEY=sk-dev-...
PROJECT_ROOT=/workspace/dev
DEBUG_MODE=true
docker-compose up -d

# Production
OPENAI_API_KEY=sk-prod-...
PROJECT_ROOT=/workspace/production
DEBUG_MODE=false
COMPLIANCE_MODE=strict
docker-compose up -d
```

**Benefits:**
- ✅ Single bundle to maintain
- ✅ ENV vars easy to inject in CICD
- ✅ Secrets separate from bundle
- ✅ Same agents, different behavior

#### Strategy 2: Separate Bundles

**Different bundles per environment:**

```bash
# Create environment-specific bundles
stn bundle create dev --output dev-agents.tar.gz
stn bundle create staging --output staging-agents.tar.gz
stn bundle create production --output production-agents.tar.gz

# Deploy to different servers
scp dev-agents.tar.gz dev-server:/opt/station/bundles/
scp staging-agents.tar.gz staging-server:/opt/station/bundles/
scp production-agents.tar.gz prod-server:/opt/station/bundles/
```

**Benefits:**
- ✅ Complete environment isolation
- ✅ Different agents per environment
- ✅ Easy rollback (swap bundle)

**Drawbacks:**
- ❌ Multiple bundles to maintain
- ❌ Drift risk between environments

---

## Security Best Practices

### Secrets Management

#### ✅ DO:
- Use environment variables for API keys
- Store secrets in vault (AWS Secrets Manager, HashiCorp Vault)
- Use IAM roles for cloud credentials (avoid static keys)
- Rotate credentials regularly
- Use different keys per environment

#### ❌ DON'T:
- Hardcode secrets in bundles
- Commit `.env` files to git
- Share credentials across environments
- Use root AWS keys in production

### Network Security

**Firewall Rules:**
```bash
# Allow only necessary ports
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 8585/tcp  # Station API (if public)
sudo ufw deny 8586/tcp   # MCP (internal only)
sudo ufw enable
```

**TLS/SSL:**
```yaml
# Use reverse proxy for TLS termination
services:
  nginx:
    image: nginx:latest
    ports:
      - "443:443"
    volumes:
      - ./ssl/cert.pem:/etc/nginx/cert.pem
      - ./ssl/key.pem:/etc/nginx/key.pem
      - ./nginx.conf:/etc/nginx/nginx.conf
  
  station:
    expose:
      - "8585"  # Not publicly exposed
```

### Access Control

**Docker Socket Protection:**
```yaml
# Never mount Docker socket in production!
# ❌ Bad:
volumes:
  - /var/run/docker.sock:/var/run/docker.sock

# ✅ Good: Run Station without Docker socket access
```

---

## Monitoring & Observability

### Health Checks

**Station Health Endpoint:**
```bash
curl http://localhost:8585/health

# Response:
{
  "status": "healthy",
  "version": "v0.1.0",
  "uptime": 3600,
  "agents": 10
}
```

**Monitoring Script:**
```bash
#!/bin/bash
# /opt/station/healthcheck.sh

HEALTH_URL="http://localhost:8585/health"
MAX_RETRIES=3

for i in $(seq 1 $MAX_RETRIES); do
  if curl -f "$HEALTH_URL" > /dev/null 2>&1; then
    echo "✅ Station healthy"
    exit 0
  fi
  sleep 5
done

echo "❌ Station unhealthy - restarting"
systemctl restart station
exit 1
```

**Add to cron:**
```bash
# Check every 5 minutes
*/5 * * * * /opt/station/healthcheck.sh >> /var/log/station-health.log 2>&1
```

### Logging

**JSON Structured Logging:**
```yaml
logging:
  driver: "json-file"
  options:
    max-size: "100m"
    max-file: "10"
    labels: "environment=production,service=station"
```

**View Logs:**
```bash
# Docker Compose
docker-compose logs -f station

# Systemd
journalctl -u station -f

# Filter for errors
docker-compose logs station | grep ERROR
```

**Log Aggregation (Optional):**
- Integrate with CloudWatch Logs, Datadog, ELK Stack
- Set `STATION_LOG_FORMAT=json` for structured logging

---

## Troubleshooting

### Issue: Container Won't Start

**Symptoms:**
- Container exits immediately
- No logs visible

**Diagnosis:**
```bash
# Check container logs
docker-compose logs station

# Common causes:
# 1. Missing OPENAI_API_KEY
# 2. Invalid bundle format
# 3. Permission issues
```

**Solution:**
```bash
# Verify environment variables
docker-compose config | grep OPENAI_API_KEY

# Test bundle extraction
tar -tzf bundles/production-agents.tar.gz

# Check volume permissions
docker-compose exec station ls -la /bundles
```

### Issue: Agents Not Found After Bundle Install

**Symptoms:**
- Bundle installs but agents don't show up
- `stn agent list` shows no agents

**Diagnosis:**
```bash
# Check if environment was created
docker-compose exec station stn env list

# Check if sync happened
docker-compose exec station ls -la /root/.config/station/environments/
```

**Solution:**
```bash
# Manual sync
docker-compose exec station stn sync production -i=false

# Check for sync errors
docker-compose logs station | grep "sync"
```

### Issue: Template Variables Not Resolved

**Symptoms:**
- MCP servers show `{{ .VARIABLE_NAME }}` in logs
- Agents fail to connect to MCP tools

**Diagnosis:**
```bash
# Check environment variables
docker-compose exec station env | grep PROJECT_ROOT

# Check variables.yml
docker-compose exec station cat /root/.config/station/environments/production/variables.yml
```

**Solution:**
```bash
# Add missing environment variable
# docker-compose.yml:
environment:
  - PROJECT_ROOT=/workspace

# Or manually create variables.yml
docker-compose exec station bash -c '
  echo "PROJECT_ROOT: /workspace" > /root/.config/station/environments/production/variables.yml
'

# Re-sync
docker-compose exec station stn sync production
```

### Issue: Agent Execution Hangs

**Symptoms:**
- Agent starts but never completes
- Run status stuck at "running"

**Diagnosis:**
```bash
# Check active runs
docker-compose exec station stn runs list

# Check specific run
docker-compose exec station stn runs inspect <run-id>

# Check for orphaned processes
docker-compose exec station ps aux | grep stn
```

**Solution:**
```bash
# Recent fix (v0.1.0+): Signal handling for interruptions
# Update to latest version if seeing stuck runs

# Clean up zombie runs (manual fix):
docker-compose exec station sqlite3 /root/.config/station/station.db '
  UPDATE agent_runs 
  SET status="cancelled", 
      completed_at=datetime("now"), 
      final_response="Execution timeout - manually cancelled" 
  WHERE status="running" AND started_at < datetime("now", "-15 minutes");
'
```

---

## Deployment Checklist

### Pre-Deployment
- [ ] Bundle created and validated
- [ ] Secrets configured (ENV vars or vault)
- [ ] Server/cluster prepared
- [ ] Docker/K8s configured
- [ ] Health check endpoints tested
- [ ] Backup strategy defined

### Deployment
- [ ] Bundle uploaded to target
- [ ] Environment variables set
- [ ] Container/service started
- [ ] Health check passing
- [ ] Agents listed successfully
- [ ] Test agent execution works

### Post-Deployment
- [ ] Monitoring configured
- [ ] Logs rotating properly
- [ ] Backup running
- [ ] Access controls verified
- [ ] Performance baseline captured
- [ ] Runbook documented

---

## What's Missing & Future Improvements

### Currently Gaps (Documented in This Guide)
- ✅ Production bundle workflow
- ✅ Multi-environment strategy
- ✅ CICD integration examples
- ✅ Troubleshooting procedures
- ✅ Security hardening

### Future Enhancements Needed

#### 1. **Automated Deployment Command**
```bash
# One-liner deployment (doesn't exist yet)
stn deploy production \
  --server user@host \
  --bundle production-agents.tar.gz \
  --secrets vault://production/station
```

#### 2. **Bundle Validation**
```bash
# Pre-deployment validation (doesn't exist yet)
stn bundle validate production-agents.tar.gz
# Output:
# ✅ All agent .prompt files valid YAML
# ✅ All MCP configs well-formed
# ⚠️  Warning: faker MCP requires OPENAI_API_KEY
# ❌ Error: Missing tool __get_cost_data in manifest
```

#### 3. **Bundle Registry**
```bash
# Central bundle repository (doesn't exist yet)
stn bundle search security
stn bundle install registry://security-scanner@1.2.0
stn bundle publish production-agents.tar.gz --tag latest
```

#### 4. **Enhanced Docker Image Variants**
```bash
# Current: station:latest (minimal Alpine)
# Need:
# - station:node (with Node.js for real MCP servers)
# - station:python (with Python for Python MCP servers)
# - station:full (kitchen sink for all MCP types)
```

#### 5. **Observability Integration**
- Prometheus metrics export
- OpenTelemetry traces
- Structured JSON logging
- Grafana dashboards

---

## Support & Resources

### Documentation
- **Station Docs**: https://docs.cloudshipai.com
- **Zero-Config Guide**: [docs/station/zero-config-deployments.md](../station/zero-config-deployments.md)
- **Docker Guide**: [docs/station/docker-compose-deployments.md](../station/docker-compose-deployments.md)
- **CICD Guide**: [deployments/README.md](../deployments/README.md)

### Examples
- **Zero-Config Template**: `examples/zero-config-deploy/`
- **GitHub Actions**: `deployments/github-actions/`
- **Kubernetes Manifests**: See "Option B" in this guide

### Getting Help
- **GitHub Issues**: https://github.com/cloudshipai/station/issues
- **Discord**: https://discord.gg/cloudshipai
- **Email**: support@cloudshipai.com

---

**Last Updated**: November 11, 2025  
**Contributors**: Station Team, Community  
**License**: MIT
