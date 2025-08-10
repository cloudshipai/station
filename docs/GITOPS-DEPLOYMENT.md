# GitOps Deployment Guide

This guide demonstrates how to deploy Station in production with SQLite state persistence using Litestream, enabling true GitOps workflows for AI agent deployment.

## The Challenge: SQLite in GitOps

GitOps deployments typically use ephemeral containers, but Station needs persistent state for:
- Agent configurations and history  
- MCP server connections
- Environment variables and secrets
- Execution logs and metrics

**Station's Solution**: Litestream integration for automatic SQLite replication and restoration.

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Git Repo      │───▶│  Station Pod    │───▶│  S3/GCS/Azure   │
│ Agent Templates │    │  + Litestream   │    │  DB Backups     │
│ Configurations  │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │ Ephemeral SQLite │
                       │ Auto-Restored   │
                       └─────────────────┘
```

## Quick Start

### 1. Set Up Litestream Replica Storage

Choose your preferred cloud storage:

**AWS S3:**
```bash
# Create S3 bucket for database backups
aws s3 mb s3://your-station-backups
aws s3api put-bucket-encryption \
  --bucket your-station-backups \
  --server-side-encryption-configuration '{
    "Rules": [{"ApplyServerSideEncryptionByDefault": {"SSEAlgorithm": "AES256"}}]
  }'
```

**Google Cloud Storage:**
```bash
gsutil mb gs://your-station-backups
gsutil versioning set on gs://your-station-backups
```

**Azure Blob Storage:**
```bash
az storage container create --name station-backups --account-name yourstorageaccount
```

### 2. Deploy with Docker Compose

```bash
# Clone your Station configuration repo
git clone https://github.com/yourorg/station-config
cd station-config

# Set up environment
cp .env.example .env
# Edit .env with your cloud storage credentials

# Deploy
docker-compose -f examples/deployments/docker-compose/docker-compose.production.yml up -d
```

### 3. Deploy with Kubernetes

```bash
# Update secrets in station-deployment.yml
kubectl apply -f examples/deployments/kubernetes/station-deployment.yml

# Verify deployment
kubectl get pods -l app=station
kubectl logs -f deployment/station
```

## Environment Configuration

### Docker Compose Environment Variables

```bash
# .env file
LITESTREAM_S3_BUCKET=your-station-backups
LITESTREAM_S3_REGION=us-east-1
LITESTREAM_S3_ACCESS_KEY_ID=your-access-key
LITESTREAM_S3_SECRET_ACCESS_KEY=your-secret-key

OPENAI_API_KEY=your-openai-key
ANTHROPIC_API_KEY=your-anthropic-key
```

### Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: station-secrets
type: Opaque
stringData:
  s3-access-key-id: "AKIAIOSFODNN7EXAMPLE"
  s3-secret-access-key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  openai-api-key: "sk-..."
  anthropic-api-key: "sk-ant-..."
```

## GitOps Workflow

### Repository Structure

```
your-station-config/
├── agents/                          # Agent templates
│   ├── database-monitor/
│   │   ├── bundle/
│   │   │   ├── manifest.json
│   │   │   ├── agent.json
│   │   │   └── variables.schema.json
│   │   └── variables/
│   │       ├── staging.yml
│   │       └── production.enc
│   └── log-analyzer/
├── environments/                    # Environment configs
│   ├── staging/
│   │   ├── variables.yml
│   │   └── mcp-servers/
│   └── production/
│       ├── variables.enc
│       └── mcp-servers/
├── .github/workflows/
│   └── deploy-agents.yml           # CI/CD pipeline
└── scripts/
    ├── encrypt-secrets.sh
    └── deploy-agent.sh
```

### Deployment Pipeline

The GitOps pipeline automatically:

1. **Validates** all agent templates and environment configs
2. **Deploys to staging** for testing
3. **Runs integration tests** against deployed agents  
4. **Promotes to production** after validation
5. **Verifies database backups** are created

### Manual Agent Deployment

```bash
# Deploy specific agent to production
curl -X POST https://station.company.com/api/v1/agents/templates/install \
  -H "Authorization: Bearer $STATION_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bundle_path": "./agents/database-monitor",
    "environment": "production",
    "variables_file": "environments/production/variables.enc"
  }'
```

## State Persistence Deep Dive

### How Litestream Works

1. **Startup**: Container starts → Litestream restores DB from replica → Station loads restored state
2. **Runtime**: Station operates normally → Litestream continuously replicates changes
3. **Deployment**: New container → Automatic restore → Zero state loss
4. **Backup**: Point-in-time restoration available from cloud storage

### Database Recovery

```bash
# Restore database to specific point in time
litestream restore -config /config/litestream.yml \
  -timestamp 2024-01-15T14:30:00Z \
  /data/station.db

# List available restore points
litestream snapshots -config /config/litestream.yml /data/station.db
```

### Monitoring Replication

```bash
# Check replication status
litestream status -config /config/litestream.yml

# View replication metrics
curl http://localhost:8080/metrics | grep litestream
```

## Security Best Practices

### Secret Management

1. **Environment Variables**: Encrypt using `sops`, `sealed-secrets`, or cloud secret management
2. **Database Encryption**: Use encrypted cloud storage for Litestream replicas
3. **Network Security**: Deploy behind VPN/firewall with TLS termination
4. **Access Control**: Implement RBAC for Station API endpoints

### Example Secret Encryption

```bash
# Using sops with AWS KMS
sops --encrypt --kms arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012 \
  environments/production/variables.yml > environments/production/variables.enc

# Using kubectl with sealed-secrets
echo -n "your-secret" | kubectl create secret generic station-secret \
  --dry-run=client --from-file=key=/dev/stdin -o yaml | kubeseal -o yaml
```

## Troubleshooting

### Database Issues

```bash
# Check database file and Litestream status
kubectl exec -it deployment/station -- ls -la /data/
kubectl exec -it deployment/station -- litestream status -config /config/litestream.yml

# Force database restoration
kubectl exec -it deployment/station -- litestream restore -config /config/litestream.yml /data/station.db
```

### Replication Issues

```bash
# View Litestream logs
kubectl logs deployment/station | grep litestream

# Verify cloud storage connectivity
kubectl exec -it deployment/station -- aws s3 ls s3://your-station-backups/
```

### Performance Monitoring

```bash
# Check Station health
curl https://station.company.com/health

# View metrics
curl https://station.company.com/metrics

# Database size and performance
kubectl exec -it deployment/station -- sqlite3 /data/station.db ".databases"
```

## Production Deployment Checklist

- [ ] Cloud storage bucket created with encryption
- [ ] Litestream credentials configured
- [ ] Station secrets (API keys) encrypted
- [ ] Health checks and monitoring configured
- [ ] CI/CD pipeline tested
- [ ] Database backup and restore tested
- [ ] Network security (TLS, firewall) configured
- [ ] Resource limits and scaling configured
- [ ] Log aggregation and alerts configured

## Cost Optimization

### Storage Costs
- **S3 Standard**: ~$0.023/GB/month for active replicas
- **S3 Glacier**: ~$0.004/GB/month for long-term retention
- **Database Size**: Typical Station deployments: 100MB-1GB

### Compute Costs
- **Minimum**: 512MB RAM, 0.25 CPU cores
- **Recommended**: 2GB RAM, 1 CPU core for production
- **Scaling**: Single replica (SQLite limitation)

This GitOps approach gives you production-ready Station deployments with full state persistence, automatic backups, and zero-downtime deployments.