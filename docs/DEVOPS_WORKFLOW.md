# Complete DevOps Workflow for Station Agent Teams

This guide documents the complete end-to-end DevOps lifecycle for building, testing, and deploying Station agent teams from development to production.

## Overview

The Station DevOps workflow enables:
1. **Local Development**: Create and test agents using MCP tools
2. **Version Control**: Commit agent configurations to Git
3. **Automated Builds**: CI/CD builds bundles and container images
4. **Deployment**: Deploy to Docker, Kubernetes, or any container platform
5. **Business Model**: Share private repos with customers, they deploy images to their infrastructure

## Workflow Stages

### Stage 1: Agent Development

#### 1.1 Create Environment (if needed)

```bash
# Create new environment for your agent team
stn env create my-agent-team
cd ~/.config/station/environments/my-agent-team/
```

#### 1.2 Create Agents Using MCP Tools

```bash
# Using opencode-station MCP tools (preferred)
# Create agent via MCP
curl -X POST http://localhost:9686/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "create_agent",
      "arguments": {
        "name": "My Agent",
        "description": "Agent description",
        "prompt": "System prompt for the agent",
        "environment_id": "3",
        "max_steps": 8
      }
    }
  }'

# Or use CLI
stn agent create \
  --name "My Agent" \
  --description "Agent description" \
  --env my-agent-team \
  --max-steps 8
```

#### 1.3 Test Agent Locally

```bash
# Test agent execution
stn agent run "My Agent" \
  "Test task for the agent" \
  --env my-agent-team \
  --tail

# Inspect execution results
stn runs list --agent-id <agent-id>
stn runs inspect <run-id> -v
```

#### 1.4 Export Agent Configuration

```bash
# Export all agents in environment to .prompt files
stn agent export-agents \
  --env my-agent-team \
  --output-directory ~/.config/station/environments/my-agent-team/agents/
```

### Stage 2: Version Control

#### 2.1 Commit Agent Configurations

```bash
cd ~/my-station-repo
git add environments/my-agent-team/agents/*.prompt
git add environments/my-agent-team/template.json
git add environments/my-agent-team/variables.yml
git commit -m "feat: add my-agent-team with 3 agents"
git push origin main
```

### Stage 3: Automated CI/CD Builds

Station provides two GitHub Actions workflows for automated builds:

#### 3.1 Build Agent Bundle (Optional)

**Workflow**: `.github/workflows/build-bundle.yml`

**Manual Trigger**:
```bash
# Trigger via GitHub CLI
gh workflow run build-bundle.yml \
  -f environment_name=my-agent-team \
  -f bundle_version=1.0.0
```

**What it does**:
- Builds a `.tar.gz` bundle from environment
- Creates bundle manifest JSON
- Uploads as GitHub Release
- Generates installation instructions

**Bundle Installation**:
```bash
# Users can install your bundle
stn template install \
  https://github.com/yourorg/station-repo/releases/download/bundle-my-agent-team-1.0.0/my-agent-team-1.0.0.tar.gz
```

#### 3.2 Build Environment Container Image

**Workflow**: `.github/workflows/build-env-image.yml`

**Manual Trigger**:
```bash
# Trigger via GitHub CLI
gh workflow run build-env-image.yml \
  -f environment_name=my-agent-team \
  -f image_tag=v1.0.0 \
  -f push_to_registry=true
```

**What it does**:
- Builds deployment-ready container image using `stn build env`
- Tags image for GitHub Container Registry (GHCR)
- Pushes to `ghcr.io/yourorg/station-env-my-agent-team:v1.0.0`
- Generates deployment instructions (Docker Compose, Kubernetes, Docker)
- Creates artifacts with deployment guides

**Image Registry**:
- Public: `ghcr.io/yourorg/station-env-my-agent-team:v1.0.0`
- Private: Same, with authentication required

### Stage 4: Deployment

#### 4.1 Docker Compose Deployment

```yaml
# docker-compose.yml
services:
  station:
    image: ghcr.io/yourorg/station-env-my-agent-team:v1.0.0
    ports:
      - "9686:9686"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - station-data:/data
      - station-backups:/backups

volumes:
  station-data:
  station-backups:
```

```bash
# Deploy
docker-compose up -d

# Verify
curl http://localhost:9686/health
```

#### 4.2 Kubernetes Deployment

```bash
# Create secrets
kubectl create secret generic station-secrets \
  --from-literal=openai-api-key="${OPENAI_API_KEY}"

# Apply deployment
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station-my-agent-team
spec:
  replicas: 1
  selector:
    matchLabels:
      app: station-my-agent-team
  template:
    metadata:
      labels:
        app: station-my-agent-team
    spec:
      containers:
      - name: station
        image: ghcr.io/yourorg/station-env-my-agent-team:v1.0.0
        ports:
        - containerPort: 9686
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: station-data
---
apiVersion: v1
kind: Service
metadata:
  name: station-my-agent-team
spec:
  selector:
    app: station-my-agent-team
  ports:
  - port: 9686
    targetPort: 9686
  type: LoadBalancer
EOF
```

#### 4.3 Direct Docker Deployment

```bash
# Pull and run
docker pull ghcr.io/yourorg/station-env-my-agent-team:v1.0.0

docker run -d \
  --name station-my-agent-team \
  -p 9686:9686 \
  -e OPENAI_API_KEY=${OPENAI_API_KEY} \
  -v station-data:/data \
  -v station-backups:/backups \
  ghcr.io/yourorg/station-env-my-agent-team:v1.0.0
```

### Stage 5: Production Operations

#### 5.1 Verify Deployment

```bash
# Health check
curl http://your-deployment:9686/health

# MCP endpoint check
curl -X POST http://your-deployment:9686/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

#### 5.2 Call Agents in Production

```bash
# Execute agent via MCP
curl -X POST http://your-deployment:9686/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "call_agent",
      "arguments": {
        "agent_id": "1",
        "task": "Production task for the agent"
      }
    }
  }'
```

#### 5.3 Monitor and Inspect

```bash
# View runs (requires access to Station API)
curl http://your-deployment:9686/api/v1/runs

# Inspect specific run
curl http://your-deployment:9686/api/v1/runs/<run-id>
```

## Business Model: Private Repo + Image Distribution

### Customer Workflow

1. **You (Station Team Builder)**:
   - Create agent teams in private GitHub repo
   - Push commits trigger CI/CD builds
   - Share private repo access with customer OR
   - Share container images via private registry

2. **Customer (Image Consumer)**:
   - Pull container images from your private GHCR
   - Deploy to their Kubernetes cluster
   - Connect to deployed Station via MCP
   - Execute agents without needing source code access

### Private Registry Setup

```bash
# Authenticate to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Pull private image
docker pull ghcr.io/yourorg/station-env-customer-agents:v1.0.0

# Deploy to customer infrastructure
kubectl apply -f customer-deployment.yaml
```

### Access Control

- **Private Repo**: Only your team has access to agent source code
- **Private Registry**: Only authorized users can pull images
- **Customer Deployment**: Customer deploys to their infrastructure
- **Customer Connection**: Customer connects via MCP to deployed Station

## Complete Lifecycle Example

```bash
# 1. LOCAL DEVELOPMENT
stn env create customer-finops
cd ~/.config/station/environments/customer-finops/

# Create 3 agents for FinOps team
stn agent create --name "Cost Analyzer" --env customer-finops
stn agent create --name "Budget Forecaster" --env customer-finops  
stn agent create --name "Optimization Recommender" --env customer-finops

# Test agents
stn agent run "Cost Analyzer" "Analyze AWS costs for November" --env customer-finops

# 2. VERSION CONTROL
cd ~/station-customer-repo
git add environments/customer-finops/
git commit -m "feat: add FinOps agent team for customer"
git push origin main

# 3. CI/CD BUILD (GitHub Actions)
# Automatically triggered on push, or manual:
gh workflow run build-env-image.yml \
  -f environment_name=customer-finops \
  -f image_tag=v1.0.0 \
  -f push_to_registry=true

# 4. CUSTOMER DEPLOYMENT
# Customer receives image: ghcr.io/yourorg/station-env-customer-finops:v1.0.0
kubectl apply -f finops-deployment.yaml

# 5. CUSTOMER USAGE
# Customer connects via MCP and executes agents
curl -X POST http://station.customer.com:9686/mcp \
  -d '{"method":"tools/call","params":{"name":"call_agent","arguments":{"agent_id":"1","task":"Analyze Q4 costs"}}}'
```

## Best Practices

### Development
- Use pre-built environments from station-demo when possible
- Test agents thoroughly before committing
- Export agents to .prompt files for version control
- Keep environment variables in `variables.yml`

### CI/CD
- Tag releases with semantic versioning (v1.0.0, v1.1.0, etc.)
- Build images on every main branch push
- Use workflow_dispatch for manual builds
- Store deployment instructions as artifacts

### Deployment
- Always use persistent volumes for `/data` and `/backups`
- Set resource limits in Kubernetes deployments
- Use secrets for API keys (never hardcode)
- Enable health checks and readiness probes

### Security
- Use private registries for customer-specific agents
- Implement RBAC for GitHub repo access
- Rotate API keys regularly
- Audit agent execution logs

## Troubleshooting

### Build Issues
```bash
# Check workflow logs
gh run list --workflow=build-env-image.yml
gh run view <run-id> --log

# Test build locally
cd ~/station-repo
go build -o bin/stn ./cmd/main
./bin/stn build env my-agent-team --skip-sync --tag test
```

### Deployment Issues
```bash
# Check container logs
docker logs station-my-agent-team

# Check Kubernetes pods
kubectl logs deployment/station-my-agent-team
kubectl describe pod <pod-name>

# Verify MCP connection
curl -v http://localhost:9686/mcp
```

### Agent Execution Issues
```bash
# Check agent configuration
stn agent get <agent-id> --env my-agent-team

# View run logs
stn runs inspect <run-id> -v

# Check MCP server connections
curl http://localhost:9686/api/v1/mcp/servers
```

## Next Steps

- Set up GitHub Actions in your repository
- Create your first agent team
- Test the complete workflow end-to-end
- Deploy to Kubernetes and verify MCP connectivity
- Implement MCP OAuth for dynamic agent access (coming soon)

## Reference

- [Station Documentation](../README.md)
- [Kubernetes Deployment Guide](../deployments/kubernetes/README.md)
- [Bundle Creation Guide](../docs/features/BUNDLE_CREATION.md)
- [GitHub Actions Workflows](../.github/workflows/)
