# Database Replication Guide

Station provides built-in SQLite database replication using Litestream, enabling production deployments with automatic backup and recovery capabilities.

## Quick Start

### 1. Enable Replication
```bash
# Initialize Station with replication support
stn init --replicate
```

This creates:
- `litestream.yml` - Replication configuration
- `.env.example` - Environment variable template
- Deployment examples for Docker and Kubernetes

### 2. Configure Cloud Storage

Edit the generated `~/.config/station/litestream.yml` file to configure your preferred cloud storage:

#### AWS S3 (Recommended)
```yaml
dbs:
  - path: /data/station.db
    replicas:
      - type: s3
        bucket: ${LITESTREAM_S3_BUCKET}
        path: station-db
        region: ${LITESTREAM_S3_REGION:-us-east-1}
        access-key-id: ${LITESTREAM_S3_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_S3_SECRET_ACCESS_KEY}
        sync-interval: 10s
        retention: 24h
```

**Required Environment Variables:**
```bash
LITESTREAM_S3_BUCKET=my-station-backups
LITESTREAM_S3_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
LITESTREAM_S3_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCY
LITESTREAM_S3_REGION=us-east-1  # Optional, defaults to us-east-1
```

#### Google Cloud Storage
```yaml
dbs:
  - path: /data/station.db
    replicas:
      - type: gcs
        bucket: ${LITESTREAM_GCS_BUCKET}
        path: station-db
        service-account-json-path: /secrets/gcs-service-account.json
        sync-interval: 10s
        retention: 24h
```

**Required Setup:**
1. Create service account with Storage Admin permissions
2. Download service account JSON key
3. Mount as secret in your deployment

#### Azure Blob Storage
```yaml
dbs:
  - path: /data/station.db
    replicas:
      - type: abs
        bucket: ${LITESTREAM_ABS_BUCKET}
        path: station-db
        account-name: ${LITESTREAM_ABS_ACCOUNT_NAME}
        account-key: ${LITESTREAM_ABS_ACCOUNT_KEY}
        sync-interval: 10s
        retention: 24h
```

### 3. Deploy to Production

Choose your deployment method:

#### Docker Compose
```bash
# Copy and customize the production compose file
cp examples/deployments/docker-compose/docker-compose.production.yml ./
# Edit environment variables in .env file
vim .env
# Deploy
docker-compose up -d
```

#### Kubernetes
```bash
# Update secrets in the deployment manifest
vim examples/deployments/kubernetes/station-deployment.yml
# Deploy
kubectl apply -f examples/deployments/kubernetes/station-deployment.yml
```

## How Database Replication Works

### Replication Flow
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Station App   │───▶│   Litestream    │───▶│  Cloud Storage  │
│  (writes data)  │    │  (replicates)   │    │   (S3/GCS/ABS)  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Container Lifecycle
1. **Container Start**: Litestream restores database from cloud replica
2. **Runtime**: Continuous replication every 10 seconds (configurable)
3. **Container Stop**: Graceful shutdown ensures final replication
4. **New Container**: Automatically restores from latest replica

### Data Protection
- **Point-in-Time Recovery**: Restore to any previous timestamp
- **Automatic Snapshots**: Regular full database snapshots
- **WAL Streaming**: Real-time Write-Ahead Log replication
- **Retention Policies**: Configurable cleanup of old backups

## Configuration Reference

### Sync Settings
```yaml
sync-interval: 10s    # How often to sync changes (1s-1h)
retention: 24h        # How long to keep snapshots (1h-8760h)
```

**Recommended Values:**
- **Production**: `sync-interval: 10s, retention: 168h` (7 days)
- **Development**: `sync-interval: 30s, retention: 24h` (1 day)
- **High-Frequency**: `sync-interval: 1s, retention: 24h` (for critical data)

### Multiple Replicas
You can configure multiple backup destinations:

```yaml
dbs:
  - path: /data/station.db
    replicas:
      # Primary backup to S3
      - type: s3
        bucket: ${LITESTREAM_S3_BUCKET}
        path: station-db-primary
        
      # Secondary backup to different region
      - type: s3
        bucket: ${LITESTREAM_S3_BACKUP_BUCKET}
        path: station-db-backup
        region: eu-west-1
        
      # Local backup for development
      - type: file
        path: /backup/station-db-local
```

## Production Deployment Examples

### AWS ECS with S3
```json
{
  "family": "station-production",
  "taskDefinition": {
    "containerDefinitions": [
      {
        "name": "station",
        "image": "station:production",
        "environment": [
          {"name": "LITESTREAM_S3_BUCKET", "value": "station-prod-backups"},
          {"name": "LITESTREAM_S3_REGION", "value": "us-east-1"}
        ],
        "secrets": [
          {"name": "LITESTREAM_S3_ACCESS_KEY_ID", "valueFrom": "arn:aws:ssm:us-east-1:123456789012:parameter/station/s3-access-key"},
          {"name": "LITESTREAM_S3_SECRET_ACCESS_KEY", "valueFrom": "arn:aws:ssm:us-east-1:123456789012:parameter/station/s3-secret-key"}
        ]
      }
    ]
  }
}
```

### Google Cloud Run with GCS
```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: station
spec:
  template:
    metadata:
      annotations:
        run.googleapis.com/execution-environment: gen2
    spec:
      containers:
      - image: station:production
        env:
        - name: LITESTREAM_GCS_BUCKET
          value: "station-prod-backups"
        volumeMounts:
        - name: service-account
          mountPath: /secrets
          readOnly: true
      volumes:
      - name: service-account
        secret:
          secretName: gcs-service-account
```

### Azure Container Instances
```yaml
apiVersion: 2019-12-01
location: eastus
name: station-production
properties:
  containers:
  - name: station
    properties:
      image: station:production
      environmentVariables:
      - name: LITESTREAM_ABS_BUCKET
        value: stationprodbackups
      - name: LITESTREAM_ABS_ACCOUNT_NAME
        secureValue: mystorageaccount
      - name: LITESTREAM_ABS_ACCOUNT_KEY
        secureValue: myaccountkey123
      resources:
        requests:
          cpu: 1
          memoryInGB: 2
```

## Monitoring and Troubleshooting

### Health Checks
Monitor replication status in your application logs:
```bash
docker logs station-container | grep litestream
```

Expected log patterns:
```
level=INFO msg="sync: new generation" db=/data/station.db
level=INFO msg="write snapshot" db=/data/station.db replica=s3
level=INFO msg="write wal segment" db=/data/station.db replica=s3
```

### Manual Database Operations

#### View Replication Status
```bash
# Inside the container
litestream status -config /config/litestream.yml
```

#### Manual Backup
```bash
# Force immediate backup
litestream sync -config /config/litestream.yml
```

#### Point-in-Time Recovery
```bash
# Restore to specific timestamp
litestream restore -config /config/litestream.yml \
  -timestamp 2024-01-15T14:30:00Z \
  /data/station.db
```

#### List Available Snapshots
```bash
litestream snapshots -config /config/litestream.yml /data/station.db
```

### Common Issues

#### "No replica configured" Warning
```
⚠️ No Litestream replica configured. Running in ephemeral mode.
```
**Solution**: Set required environment variables (e.g., `LITESTREAM_S3_BUCKET`)

#### "Access Denied" Errors
```
ERROR: failed to sync: AccessDenied: Access Denied
```
**Solution**: Verify cloud storage credentials and permissions

#### "Database locked" During Startup
```
ERROR: database is locked
```
**Solution**: Ensure previous container stopped gracefully; check for orphaned processes

## Cost Optimization

### Storage Costs
- **S3 Standard**: ~$0.023/GB/month
- **S3 Infrequent Access**: ~$0.0125/GB/month (for long-term retention)
- **GCS Standard**: ~$0.020/GB/month
- **Azure Hot**: ~$0.0184/GB/month

**Typical Station database sizes:**
- Small deployment: 10-100MB
- Medium deployment: 100MB-1GB
- Large deployment: 1-10GB

### Optimization Strategies
```yaml
# Reduce sync frequency for cost savings
sync-interval: 60s        # Instead of 10s

# Shorter retention for lower costs
retention: 72h            # Instead of 168h (7 days)

# Use cheaper storage classes for long-term retention
# (requires custom S3 lifecycle policies)
```

## Security Best Practices

### Credential Management
- **Never hardcode credentials** in configuration files
- **Use IAM roles** when possible (ECS, GKE, AKS)
- **Rotate credentials regularly** (90-day maximum)
- **Use least-privilege permissions** (read/write only to specific bucket)

### Network Security
- **Enable bucket encryption** at rest
- **Use VPC endpoints** to avoid internet traffic
- **Enable access logging** for audit trails
- **Configure bucket policies** to restrict access

### Example IAM Policy (AWS)
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::station-prod-backups",
        "arn:aws:s3:::station-prod-backups/*"
      ]
    }
  ]
}
```

## Migration from Local SQLite

If you have an existing Station deployment without replication:

### 1. Enable Replication
```bash
stn init --replicate  # Creates litestream.yml
```

### 2. Copy Existing Database
```bash
# Copy your existing database to the replication-enabled deployment
cp ~/.config/station/station.db /data/station.db
```

### 3. Start Replication
```bash
# Deploy with replication enabled
docker-compose -f docker-compose.production.yml up -d
```

The existing data will be automatically replicated to your cloud storage.

## Advanced Configuration

### Custom Retention Policies
```yaml
dbs:
  - path: /data/station.db
    replicas:
      - type: s3
        bucket: station-backups
        path: station-db
        # Keep snapshots for different durations
        retention: 720h      # 30 days
        snapshot-interval: 1h # Hourly snapshots
        
        # Advanced: Custom retention schedule
        retention-check-interval: 1h
```

### Monitoring Integration
```yaml
# Add metrics endpoint for monitoring
dbs:
  - path: /data/station.db
    replicas:
      - type: s3
        bucket: station-backups
        path: station-db
        # Enable Prometheus metrics
        monitor-interval: 30s
```

For more advanced configurations, see the [official Litestream documentation](https://litestream.io/).

## Support

- **Issues**: Report problems at [Station GitHub Issues](https://github.com/cloudshipai/station/issues)
- **Community**: Join our [Discord](https://discord.gg/station-ai)
- **Enterprise**: Contact [enterprise@station.ai](mailto:enterprise@station.ai) for deployment assistance