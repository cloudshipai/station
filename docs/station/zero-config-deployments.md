# Zero-Config Deployments

Deploy Station agents to production with automatic credential discovery and minimal configuration. Station automatically detects cloud credentials, service endpoints, and environment settings, eliminating manual configuration overhead.

## What is Zero-Config Deployment?

Zero-config deployment automatically discovers and configures:
- **Cloud Credentials** - IAM roles, service accounts, instance metadata
- **Environment Variables** - Runtime secrets and configuration
- **Bundle Installation** - Automatic agent deployment from mounted directories
- **MCP Server Connections** - Template variable resolution from environment
- **Service Discovery** - Database connections, API endpoints, cloud services

**No manual configuration files needed**—Station detects and configures everything automatically at runtime.

## Quick Start

### Docker Compose Zero-Config

```bash
# Step 1: Create bundles directory
mkdir bundles
cp security-scanner.tar.gz bundles/

# Step 2: Set environment variables
export OPENAI_API_KEY="sk-..."
export AWS_ACCESS_KEY_ID="AKIA..."     # Optional - uses IAM role if not provided
export AWS_SECRET_ACCESS_KEY="..."

# Step 3: Deploy
docker-compose up -d

# Step 4: Access Station
open http://localhost:8585
```

**What happens automatically:**
1. Station initializes with OpenAI provider
2. Discovers and installs all bundles from `/bundles` directory
3. Syncs environments and discovers MCP tools
4. Detects AWS credentials from environment or IAM role
5. Starts server with all agents ready

## How It Works

### Automatic Discovery Process

```
┌─────────────────────────────────────┐
│  1. Container Starts                │
│     ↓                               │
│  2. Detect Cloud Environment        │
│     • EC2 instance metadata         │
│     • ECS task role                 │
│     • Kubernetes service account    │
│     ↓                               │
│  3. Load Environment Variables      │
│     • OPENAI_API_KEY               │
│     • AWS_* credentials            │
│     • Template variable overrides   │
│     ↓                               │
│  4. Install Bundles                 │
│     • Scan /bundles directory      │
│     • Extract agent configs        │
│     • Create environments          │
│     ↓                               │
│  5. Sync Environments               │
│     • Resolve template variables   │
│     • Connect MCP servers          │
│     • Discover tools               │
│     ↓                               │
│  6. Start Station Server            │
│     • API: http://0.0.0.0:8585    │
│     • MCP: http://0.0.0.0:8586    │
│     • Agents ready for execution   │
└─────────────────────────────────────┘
```

### Credential Discovery Order

Station discovers credentials in this priority order:

**AWS Credentials:**
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. ECS task role credentials (from `AWS_CONTAINER_CREDENTIALS_RELATIVE_URI`)
3. EC2 instance metadata service (IAM instance role)
4. Mounted AWS config files (`~/.aws/credentials`)

**AI Provider Keys:**
1. Environment variables (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.)
2. Mounted config file (`/root/.config/station/config.yaml`)
3. Initialization prompt (first startup)

**Template Variables:**
1. Environment variables matching template variable names
2. `variables.yml` file in environment directory
3. Interactive prompts during sync (if needed)

## Deployment Patterns

### Pattern 1: Docker Compose with IAM Roles (EC2)

Deploy on EC2 instance with IAM instance role—no AWS credentials needed.

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-zero-config
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      # Only AI provider key required
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      # AWS credentials auto-discovered from EC2 instance role
    volumes:
      - ./bundles:/bundles:ro
      - station-data:/root/.config/station
    command: >
      sh -c "
        echo '🚀 Station Zero-Config Deployment Starting...' &&

        if [ ! -f /root/.config/station/config.yaml ]; then
          echo '📦 Initializing Station...' &&
          stn init --provider openai --model gpt-4o-mini --yes
        fi &&

        if [ -d /bundles ] && [ \"\$(ls -A /bundles/*.tar.gz 2>/dev/null)\" ]; then
          echo '📦 Installing bundles from /bundles directory...' &&
          for bundle in /bundles/*.tar.gz; do
            bundle_name=\$(basename \"\$bundle\" .tar.gz) &&
            echo \"  Installing: \$bundle_name\" &&
            stn bundle install \"\$bundle\" \"\$bundle_name\" &&
            echo \"  Syncing: \$bundle_name\" &&
            stn sync \"\$bundle_name\" -i=false || echo \"  ⚠️  Sync had validation errors (non-blocking)\"
          done &&
          echo '✅ All bundles installed and synced!'
        else
          echo '⚠️  No bundles found in /bundles directory'
        fi &&

        echo '🌐 Starting Station server...' &&
        stn serve
      "
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  station-data:
    driver: local
```

**EC2 IAM Role Policy:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ce:GetCostAndUsage",
        "ce:GetCostForecast",
        "ce:GetReservationCoverage",
        "ce:GetSavingsPlansCoverage",
        "s3:ListBucket",
        "s3:GetObject",
        "ec2:DescribeInstances",
        "rds:DescribeDBInstances"
      ],
      "Resource": "*"
    }
  ]
}
```

**Deployment:**
```bash
# On EC2 instance with IAM role attached
git clone https://github.com/your-org/station-deployment
cd station-deployment

# Only set AI provider key
export OPENAI_API_KEY="sk-..."

# Deploy - AWS credentials auto-discovered
docker-compose up -d

# Verify AWS credentials working
docker-compose exec station aws sts get-caller-identity
```

### Pattern 2: ECS/Fargate with Task Roles

Deploy on AWS ECS/Fargate with automatic task role credentials.

**task-definition.json:**
```json
{
  "family": "station-agents",
  "taskRoleArn": "arn:aws:iam::123456789012:role/StationAgentRole",
  "executionRoleArn": "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "1024",
  "memory": "2048",
  "containerDefinitions": [
    {
      "name": "station",
      "image": "ghcr.io/cloudshipai/station:latest",
      "portMappings": [
        {"containerPort": 8585, "protocol": "tcp"},
        {"containerPort": 8586, "protocol": "tcp"}
      ],
      "environment": [
        {"name": "OPENAI_API_KEY", "value": "from-secrets-manager"},
        {"name": "STN_CLOUDSHIP_ENABLED", "value": "true"}
      ],
      "secrets": [
        {
          "name": "OPENAI_API_KEY",
          "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:openai-key"
        }
      ],
      "mountPoints": [
        {
          "sourceVolume": "bundles",
          "containerPath": "/bundles",
          "readOnly": true
        }
      ],
      "command": [
        "sh", "-c",
        "stn init --provider openai --model gpt-4o-mini --yes && for bundle in /bundles/*.tar.gz; do bundle_name=$(basename \"$bundle\" .tar.gz) && stn bundle install \"$bundle\" \"$bundle_name\" && stn sync \"$bundle_name\" -i=false; done && stn serve"
      ],
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:8585/health || exit 1"],
        "interval": 30,
        "timeout": 10,
        "retries": 3,
        "startPeriod": 60
      }
    }
  ],
  "volumes": [
    {
      "name": "bundles",
      "host": {}
    }
  ]
}
```

**Deploy to ECS:**
```bash
# Register task definition
aws ecs register-task-definition --cli-input-json file://task-definition.json

# Create ECS service
aws ecs create-service \
  --cluster production-cluster \
  --service-name station-agents \
  --task-definition station-agents:1 \
  --desired-count 2 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[subnet-abc123],securityGroups=[sg-xyz789]}"
```

### Pattern 3: Kubernetes with Service Accounts

Deploy on Kubernetes with automatic service account credentials.

**deployment.yaml:**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: station

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: station-agent-sa
  namespace: station
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/StationAgentRole

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
apiVersion: v1
kind: ConfigMap
metadata:
  name: station-bundles
  namespace: station
data:
  # Bundles stored as ConfigMap (for small bundles)
  # Or use PersistentVolume for larger bundles

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
      serviceAccountName: station-agent-sa
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
        - name: STN_CLOUDSHIP_ENABLED
          value: "true"
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
          stn init --provider openai --model gpt-4o-mini --yes &&
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
        readinessProbe:
          httpGet:
            path: /health
            port: 8585
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: bundles
        persistentVolumeClaim:
          claimName: station-bundles-pvc
      - name: station-data
        persistentVolumeClaim:
          claimName: station-data-pvc

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
  - name: api
    port: 8585
    targetPort: 8585
  - name: mcp
    port: 8586
    targetPort: 8586
  type: LoadBalancer
```

**Deploy to Kubernetes:**
```bash
# Create namespace and deploy
kubectl apply -f deployment.yaml

# Check pods
kubectl get pods -n station

# View logs
kubectl logs -n station -l app=station -f

# Access Station
kubectl port-forward -n station svc/station 8585:8585
```

### Pattern 4: Multi-Environment with Variable Injection

Deploy multiple environments (dev/staging/prod) with environment-specific template variables.

**docker-compose.multi-env.yml:**
```yaml
version: '3.8'

services:
  station-dev:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-dev
    ports:
      - "8585:8585"
    environment:
      - OPENAI_API_KEY=${DEV_OPENAI_API_KEY}
      - PROJECT_ROOT=/workspace/dev
      - SCAN_DEPTH=shallow
      - DEBUG_MODE=true
    volumes:
      - ./bundles:/bundles:ro
      - ./dev-projects:/workspace/dev:ro
      - station-dev-data:/root/.config/station

  station-staging:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-staging
    ports:
      - "9585:8585"
    environment:
      - OPENAI_API_KEY=${STAGING_OPENAI_API_KEY}
      - PROJECT_ROOT=/workspace/staging
      - SCAN_DEPTH=medium
      - DEBUG_MODE=false
    volumes:
      - ./bundles:/bundles:ro
      - ./staging-projects:/workspace/staging:ro
      - station-staging-data:/root/.config/station

  station-prod:
    image: ghcr.io/cloudshipai/station:v0.2.8  # Pinned version
    container_name: station-prod
    ports:
      - "10585:8585"
    environment:
      - OPENAI_API_KEY=${PROD_OPENAI_API_KEY}
      - PROJECT_ROOT=/workspace/production
      - SCAN_DEPTH=deep
      - DEBUG_MODE=false
      - COMPLIANCE_MODE=strict
    volumes:
      - ./bundles:/bundles:ro
      - ./production-projects:/workspace/production:ro
      - station-prod-data:/root/.config/station
    restart: always

volumes:
  station-dev-data:
  station-staging-data:
  station-prod-data:
```

**Environment-specific .env files:**

**.env.dev:**
```bash
DEV_OPENAI_API_KEY=sk-dev-...
PROJECT_ROOT=/workspace/dev
```

**.env.staging:**
```bash
STAGING_OPENAI_API_KEY=sk-staging-...
PROJECT_ROOT=/workspace/staging
```

**.env.production:**
```bash
PROD_OPENAI_API_KEY=sk-prod-...
PROJECT_ROOT=/workspace/production
```

**Deployment:**
```bash
# Deploy development
docker-compose --env-file .env.dev -f docker-compose.multi-env.yml up -d station-dev

# Deploy staging
docker-compose --env-file .env.staging -f docker-compose.multi-env.yml up -d station-staging

# Deploy production
docker-compose --env-file .env.production -f docker-compose.multi-env.yml up -d station-prod
```

## Environment Variable Reference

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OPENAI_API_KEY` | OpenAI API key for GPT models | `sk-...` |

**OR** alternative AI providers:
- `ANTHROPIC_API_KEY` - Claude models
- `GOOGLE_API_KEY` - Gemini models

### Optional Cloud Credentials

| Variable | Description | Auto-Discovery |
|----------|-------------|----------------|
| `AWS_ACCESS_KEY_ID` | AWS access key | ✅ EC2/ECS roles |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | ✅ EC2/ECS roles |
| `AWS_REGION` | AWS region | ✅ Instance metadata |
| `GOOGLE_APPLICATION_CREDENTIALS` | GCP service account JSON path | ✅ GCE metadata |
| `AZURE_CLIENT_ID` | Azure service principal ID | ✅ Azure managed identity |

### Optional Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `STATION_DEBUG` | Enable debug logging | `false` |
| `STATION_TELEMETRY_ENABLED` | Send telemetry data | `true` |
| `STATION_ENCRYPTION_KEY` | Database encryption key | auto-generated |
| `STN_CLOUDSHIP_ENABLED` | Enable CloudShip integration | `false` |
| `STN_CLOUDSHIP_KEY` | CloudShip API key | - |
| `E2B_API_KEY` | E2B code execution key | - |

### Template Variable Overrides

Any environment variable matching a template variable name will override the value:

**template.json:**
```json
{
  "mcpServers": {
    "filesystem": {
      "args": ["{{ .PROJECT_ROOT }}"]
    }
  }
}
```

**Override with environment variable:**
```bash
export PROJECT_ROOT="/workspace/production"
docker-compose up -d
# Station resolves {{ .PROJECT_ROOT }} to /workspace/production
```

## Configuration Override

While zero-config is designed for automatic setup, you can override specific settings:

### Override 1: Custom Configuration File

Mount custom config file:
```yaml
volumes:
  - ./custom-config.yaml:/root/.config/station/config.yaml:ro
```

### Override 2: Template Variable Files

Mount custom variables.yml:
```yaml
volumes:
  - ./production-variables.yml:/root/.config/station/environments/prod/variables.yml:ro
```

### Override 3: Environment-Specific Bundles

Use different bundles per environment:
```yaml
volumes:
  - ./dev-bundles:/bundles:ro      # Development
  - ./staging-bundles:/bundles:ro  # Staging
  - ./prod-bundles:/bundles:ro     # Production
```

### Override 4: Runtime Configuration

Pass configuration via command arguments:
```yaml
command: >
  sh -c "
    stn init --provider anthropic --model claude-3-sonnet --yes &&
    stn bundle install /bundles/security.tar.gz security \
      --set PROJECT_ROOT=/custom/path \
      --set SCAN_TIMEOUT=600 &&
    stn sync security &&
    stn serve
  "
```

## Secrets Management

### AWS Secrets Manager

**ECS Task Definition with Secrets:**
```json
{
  "secrets": [
    {
      "name": "OPENAI_API_KEY",
      "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:openai-key:openai_api_key::"
    },
    {
      "name": "DATABASE_PASSWORD",
      "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:db-creds:password::"
    }
  ]
}
```

### HashiCorp Vault

**Docker Compose with Vault:**
```yaml
services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    environment:
      - OPENAI_API_KEY=${VAULT_OPENAI_KEY}
      - AWS_ACCESS_KEY_ID=${VAULT_AWS_KEY}
    env_file:
      - vault-secrets.env  # Generated by vault agent
```

**Vault Agent Template:**
```hcl
template {
  source      = "secrets.tpl"
  destination = "vault-secrets.env"
}
```

**secrets.tpl:**
```
{{ with secret "secret/openai" }}
OPENAI_API_KEY={{ .Data.data.api_key }}
{{ end }}
{{ with secret "secret/aws" }}
AWS_ACCESS_KEY_ID={{ .Data.data.access_key }}
AWS_SECRET_ACCESS_KEY={{ .Data.data.secret_key }}
{{ end }}
```

### Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: station-secrets
type: Opaque
stringData:
  openai-api-key: "sk-..."
  aws-access-key: "AKIA..."
  aws-secret-key: "..."
```

Reference in deployment:
```yaml
env:
- name: OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: station-secrets
      key: openai-api-key
```

## Best Practices

### Security

1. **Never hardcode secrets** - Always use environment variables or secret managers
2. **Use IAM roles when possible** - Avoid static AWS credentials
3. **Rotate credentials regularly** - Automated secret rotation with Secrets Manager
4. **Least privilege** - IAM policies with minimal required permissions
5. **Encrypt at rest** - Use encrypted volumes for Station data

### Reliability

1. **Health checks** - Configure liveness and readiness probes
2. **Graceful shutdown** - Allow time for in-progress agent executions to complete
3. **Persistent volumes** - Never lose agent data or configurations
4. **Auto-restart** - Use `restart: unless-stopped` for containers
5. **Resource limits** - Set CPU/memory limits to prevent resource exhaustion

### Performance

1. **Bundle pre-installation** - Use custom images with bundles pre-installed for faster startup
2. **Volume caching** - Persist downloaded MCP dependencies
3. **Multi-stage builds** - Optimize Docker image size
4. **Connection pooling** - Configure database connection pools appropriately
5. **Horizontal scaling** - Run multiple Station instances behind load balancer

### Observability

1. **Structured logging** - Use JSON log format for parsing
2. **Metrics collection** - Export metrics to Prometheus/CloudWatch
3. **Distributed tracing** - Track agent execution across services
4. **Alerting** - Monitor agent failures and performance degradation
5. **Audit logging** - Track all agent executions for compliance

## Troubleshooting

### AWS Credentials Not Auto-Discovered

**Problem:** Station can't access AWS services despite IAM role attached

**Solution:**
```bash
# Verify IAM role attached to EC2 instance
aws ec2 describe-instances --instance-ids i-123456 \
  --query 'Reservations[0].Instances[0].IamInstanceProfile'

# Test credentials inside container
docker-compose exec station aws sts get-caller-identity

# Check EC2 metadata service accessible
docker-compose exec station curl http://169.254.169.254/latest/meta-data/iam/security-credentials/
```

### Bundles Not Installing

**Problem:** Bundles directory mounted but agents not created

**Solution:**
```bash
# Verify bundles mounted correctly
docker-compose exec station ls -la /bundles

# Check bundle format
tar -tzf bundles/security-scanner.tar.gz

# Check startup logs
docker-compose logs station | grep "Installing bundles"

# Manual installation
docker-compose exec station stn bundle install /bundles/security-scanner.tar.gz security
```

### Template Variables Not Resolved

**Problem:** MCP servers show `{{ .VARIABLE_NAME }}` in logs

**Solution:**
```bash
# Check environment variables passed to container
docker-compose exec station env | grep PROJECT_ROOT

# Verify variables.yml created
docker-compose exec station cat /root/.config/station/environments/my-env/variables.yml

# Re-sync environment
docker-compose exec station stn sync my-env
```

### Health Check Failing

**Problem:** Container marked unhealthy

**Solution:**
```bash
# Test health endpoint manually
docker-compose exec station curl -f http://localhost:8585/health

# Increase start_period for slow startup
# docker-compose.yml:
healthcheck:
  start_period: 90s  # Increase from default 40s

# Check Station server process
docker-compose exec station ps aux | grep stn
```

## Production Deployment Checklist

- [ ] IAM roles configured with least-privilege permissions
- [ ] Secrets stored in secret manager (not .env files)
- [ ] Health checks configured with appropriate timeouts
- [ ] Persistent volumes configured for data retention
- [ ] Log aggregation configured (CloudWatch/ELK/Datadog)
- [ ] Monitoring and alerting set up
- [ ] Resource limits (CPU/memory) defined
- [ ] Auto-scaling policies configured (if using ECS/K8s)
- [ ] Backup strategy for Station data volume
- [ ] Disaster recovery plan documented

## Next Steps

- [Docker Compose Deployments](./docker-compose-deployments.md) - Detailed Docker deployment patterns
- [Bundles](./bundles.md) - Create and distribute agent bundles
- [Template Variables](./templates.md) - Configure environment-specific variables
- [Deployment Modes](./deployment-modes.md) - Choose the right deployment strategy
