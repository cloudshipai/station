# Station Quick Start Guide

Get Station running in 60 seconds or less. Choose the deployment method that best fits your needs.

## One-Line Install & Run

The absolute fastest way to get started:

```bash
curl -fsSL https://get.station.dev | bash
```

**What this does:**
1. Installs Station CLI to `~/.local/bin`
2. Prompts for AI provider (OpenAI/Anthropic/Gemini/Ollama)
3. Starts Station with Web UI at `http://localhost:8585`
4. Opens browser automatically

**Requirements:** Just `curl` and your AI API key.

---

## Quick Deploy with Docker Compose

Best for production-like environments with bundles:

```bash
# 1. Download quick deploy script
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/scripts/quick-deploy.sh -o quick-deploy.sh
chmod +x quick-deploy.sh

# 2. Set API key
export OPENAI_API_KEY="sk-..."

# 3. Deploy with bundles
mkdir bundles
# Add your .tar.gz bundles to bundles/ directory
./quick-deploy.sh
```

**What you get:**
- Station running in Docker
- Web UI at `http://localhost:8585`
- Auto-installs all bundles from `bundles/` directory
- Health checks and auto-restart
- Persistent data storage

---

## Installation Methods Comparison

| Method | Best For | Time to Run | Bundles | Production Ready |
|--------|----------|-------------|---------|------------------|
| **One-line** | Local development, testing | 30 sec | Manual | ❌ |
| **Quick Deploy** | Docker testing with bundles | 60 sec | Auto | ✅ |
| **Docker Compose** | Production deployment | 2 min | Auto | ✅ |
| **Kubernetes** | Cloud-native deployment | 5 min | Auto | ✅ |
| **Manual Install** | Custom setups | 3 min | Manual | ✅ |

---

## Detailed Quick Start Options

### Option 1: Local Development (Fastest)

**Perfect for:** Trying out Station, agent development, testing

```bash
# Install Station
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash

# Start Station
export OPENAI_API_KEY="sk-..."
stn up --provider openai --model gpt-4o-mini
```

**Access Points:**
- Web UI: `http://localhost:8585`
- MCP Server: `http://localhost:8586/mcp`

**Stop Station:**
```bash
stn down
```

---

### Option 2: Docker Compose (Recommended)

**Perfect for:** Production deployment, persistent agents, team environments

**Step 1: Create project directory**
```bash
mkdir station-deployment && cd station-deployment
```

**Step 2: Download quick-deploy script**
```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/scripts/quick-deploy.sh -o quick-deploy.sh
chmod +x quick-deploy.sh
```

**Step 3: Setup bundles (optional)**
```bash
mkdir bundles
# Add .tar.gz bundle files to bundles/
# Or skip if you'll create agents manually
```

**Step 4: Deploy**
```bash
export OPENAI_API_KEY="sk-..."
./quick-deploy.sh
```

**What gets created:**
- `docker-compose.yml` - Service configuration
- `bundles/` - Agent bundles directory
- `station-data/` - Docker volume for persistence

**Manage deployment:**
```bash
# View logs
docker-compose logs -f

# Stop
docker-compose down

# Restart
docker-compose restart

# Shell access
docker-compose exec station sh
```

---

### Option 3: Docker Compose from Scratch

**Perfect for:** Full control, custom configurations

**Create `docker-compose.yml`:**
```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./bundles:/bundles:ro
      - station-data:/root/.config/station
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o-mini --yes &&
        for bundle in /bundles/*.tar.gz; do
          stn bundle install \"\$bundle\" \"\$(basename \"\$bundle\" .tar.gz)\"
        done &&
        stn serve
      "
    restart: unless-stopped

volumes:
  station-data:
```

**Deploy:**
```bash
export OPENAI_API_KEY="sk-..."
docker-compose up -d
```

---

### Option 4: Kubernetes

**Perfect for:** Cloud deployments, high availability, auto-scaling

**Create `station-deployment.yaml`:**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: station

---
apiVersion: v1
kind: Secret
metadata:
  name: station-secrets
  namespace: station
type: Opaque
stringData:
  openai-api-key: "sk-..."

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station
  namespace: station
spec:
  replicas: 2
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
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        volumeMounts:
        - name: station-data
          mountPath: /root/.config/station
      volumes:
      - name: station-data
        emptyDir: {}

---
apiVersion: v1
kind: Service
metadata:
  name: station
  namespace: station
spec:
  selector:
    app: station
  ports:
  - port: 8585
    targetPort: 8585
  type: LoadBalancer
```

**Deploy:**
```bash
kubectl apply -f station-deployment.yaml
kubectl get service station -n station  # Get LoadBalancer IP
```

---

## Post-Installation: First Steps

### 1. Access Web UI

Open `http://localhost:8585` in your browser.

**What you can do:**
- Browse available MCP tools
- Install bundles from registry
- Create environments
- Manage agents
- View execution history

### 2. Connect Claude Code/Cursor

Station automatically creates `.mcp.json` when you run `stn up`:

```json
{
  "mcpServers": {
    "station": {
      "type": "http",
      "url": "http://localhost:8586/mcp"
    }
  }
}
```

**Restart Claude Code/Cursor** to connect.

### 3. Create Your First Agent

**Via Claude Code/Cursor:**
```
You: "Use Station to create a cost analysis agent that monitors AWS spending"
Claude: [Uses Station MCP tools to create agent]
```

**Via Web UI:**
1. Go to **Environments** → **Create Environment**
2. Add MCP tools (e.g., AWS Cost Explorer)
3. Create agent with dotprompt format
4. Sync environment

**Via CLI:**
```bash
# Create environment
stn env create finops

# Add agent (create .prompt file)
cat > ~/.config/station/environments/finops/agents/cost-analyzer.prompt << 'EOF'
---
metadata:
  name: "AWS Cost Analyzer"
  description: "Analyzes AWS costs and identifies savings"
model: gpt-4o-mini
max_steps: 5
tools:
  - "__get_cost_and_usage"
---

{{role "system"}}
You are a FinOps expert specializing in AWS cost optimization.

{{role "user"}}
{{userInput}}
EOF

# Sync environment
stn sync finops
```

### 4. Run Your Agent

**Via Claude Code/Cursor:**
```
You: "Run the cost analyzer to check last month's AWS spending"
Claude: [Executes agent via Station MCP tools]
```

**Via CLI:**
```bash
stn agent run "AWS Cost Analyzer" "Analyze last month's EC2 costs"
```

**Via Web UI:**
1. Go to **Agents**
2. Select agent
3. Click **Run**
4. Enter task
5. View results

---

## Quick Deployment Recipes

### Recipe: Security Scanner (CICD)

```bash
# 1. Download security scanner bundle
curl -LO https://registry.station.dev/bundles/security-scanner.tar.gz

# 2. Quick deploy
mkdir bundles && mv security-scanner.tar.gz bundles/
export OPENAI_API_KEY="sk-..."
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/scripts/quick-deploy.sh | bash

# 3. Run security scan
docker-compose exec station \
  stn agent run "Infrastructure Security Scanner" \
  "Scan /workspace for terraform security issues"
```

### Recipe: FinOps Cost Analysis

```bash
# 1. Install Station
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash

# 2. Setup AWS credentials
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
export OPENAI_API_KEY="sk-..."

# 3. Start Station
stn up --provider openai

# 4. Install FinOps bundle
stn bundle install https://registry.station.dev/bundles/aws-finops.tar.gz

# 5. Run cost analysis
stn agent run "AWS Cost Analyzer" "Analyze last 30 days of spending"
```

### Recipe: Multi-Environment Setup

```bash
# Deploy dev, staging, prod with same bundles
export OPENAI_API_KEY="sk-..."

# Development
./quick-deploy.sh --mode docker-compose --port 8585

# Staging
./quick-deploy.sh --mode docker-compose --port 9585

# Production
./quick-deploy.sh --mode docker-compose --port 10585
```

---

## Troubleshooting Quick Start

### Station CLI Not Found

```bash
# Add to PATH
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Docker Container Won't Start

```bash
# Check logs
docker-compose logs station

# Common fixes:
# 1. API key not set
export OPENAI_API_KEY="sk-..."

# 2. Port already in use
./quick-deploy.sh --port 9000 --mcp-port 9001

# 3. Permission issues
sudo docker-compose up -d
```

### Bundles Not Installing

```bash
# Verify bundle format
tar -tzf bundles/my-bundle.tar.gz

# Manual installation
docker-compose exec station stn bundle install /bundles/my-bundle.tar.gz

# Check logs for errors
docker-compose logs station | grep bundle
```

### Health Check Failing

```bash
# Increase startup time
# Edit docker-compose.yml healthcheck:
healthcheck:
  start_period: 60s  # Increase from 40s

# Test manually
docker-compose exec station curl http://localhost:8585/health
```

---

## Quick Reference Commands

### Local Development
```bash
# Start
stn up --provider openai

# Stop
stn down

# Status
stn status

# List agents
stn agent list

# Run agent
stn agent run "Agent Name" "task description"
```

### Docker Compose
```bash
# Start
docker-compose up -d

# Stop
docker-compose down

# Logs
docker-compose logs -f

# Restart
docker-compose restart

# Shell
docker-compose exec station sh

# Run agent
docker-compose exec station stn agent run "Agent Name" "task"
```

### Kubernetes
```bash
# Deploy
kubectl apply -f station-deployment.yaml

# Check status
kubectl get pods -n station

# Logs
kubectl logs -n station -l app=station -f

# Port forward
kubectl port-forward -n station svc/station 8585:8585

# Shell
kubectl exec -n station -it deploy/station -- sh
```

---

## Next Steps

Now that Station is running:

1. **[Agent Development](../station/agent-development.md)** - Create custom agents
2. **[MCP Tools](../station/mcp-tools.md)** - Add AWS, Stripe, GitHub integrations
3. **[Bundles](../station/bundles.md)** - Package and distribute agents
4. **[Production Deployment](./PRODUCTION_DEPLOYMENT.md)** - Production best practices
5. **[Examples](../station/examples.md)** - Real-world agent examples

---

## Getting Help

- **Documentation**: https://cloudshipai.github.io/station/
- **GitHub Issues**: https://github.com/cloudshipai/station/issues
- **Discord**: https://discord.gg/station-ai

---

**Happy building with Station! 🚂**
