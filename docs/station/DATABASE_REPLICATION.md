# Database Replication & Persistence

## Overview

Station uses SQLite by default for local development. For production deployments, multi-instance setups, or disaster recovery, Station supports two persistence strategies:

1. **Cloud Database (libsql)** - Distributed SQLite-compatible database
2. **Continuous Backup (Litestream)** - Automatic replication to S3/GCS/Azure

## Local SQLite (Default)

By default, Station uses a local SQLite database file.

**Location:** `~/.config/station/station.db` (or custom workspace)

**Pros:**
- Zero configuration
- Fast performance
- Perfect for local development
- Single-file portability

**Cons:**
- Not shared across machines
- No built-in backup
- Single instance only

**Usage:**
```bash
# Default - database in ~/.config/station/
stn stdio

# Custom location
export STATION_WORKSPACE=/path/to/my/station-config
stn stdio
```

---

## Option 1: Cloud Database (libsql)

Use a libsql-compatible cloud database for multi-deployment persistence and team collaboration.

### What is libsql?

libsql is an open-source SQLite-compatible database protocol that works with cloud-hosted SQLite databases. It provides:
- Multi-region replication
- Concurrent access from multiple instances
- Automatic backups
- Team collaboration with shared state

### Setup

1. **Provision a libsql database** (e.g., with a cloud provider)

2. **Configure Station to use it:**

```bash
# Set database URL
export DATABASE_URL="libsql://your-db.example.com?authToken=your-token"

# Run Station
stn stdio
```

3. **Verify connection:**
```bash
# Station will automatically:
# - Detect the libsql:// URL
# - Use the libsql driver
# - Run migrations
# - Connect to the cloud database
```

### Configuration in config.yaml

Alternatively, set in your configuration file:

```yaml
# ~/.config/station/config.yaml
database_url: "libsql://your-db.example.com?authToken=your-token"
```

### Use Cases

**Multi-Instance Deployments:**
```bash
# Server 1
DATABASE_URL="libsql://prod-db.example.com?authToken=xxx" stn stdio

# Server 2 (same database, shared state)
DATABASE_URL="libsql://prod-db.example.com?authToken=xxx" stn stdio
```

**Team Collaboration:**
```bash
# All team members connect to same database
# Agents, environments, and runs are shared
DATABASE_URL="libsql://team-db.example.com?authToken=xxx" stn stdio
```

**Kubernetes Deployments:**
```yaml
# ConfigMap or Secret
apiVersion: v1
kind: Secret
metadata:
  name: station-db
data:
  database-url: libsql://prod-db.example.com?authToken=xxx

# Deployment
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: station-db
        key: database-url
```

### Performance

- **Connection Time:** ~100ms (first connection)
- **Query Latency:** <10ms within region
- **Concurrent Connections:** 25 max (configurable)
- **Connection Pooling:** Automatic

### Security

- ✅ TLS encryption in transit
- ✅ Per-database authentication tokens
- ✅ Token rotation supported
- ⚠️ **Important:** Store tokens securely (environment variables, secrets management)

**Best Practices:**
```bash
# ❌ Don't hardcode tokens
DATABASE_URL="libsql://db.example.com?authToken=hardcoded-token"

# ✅ Use environment variables
export LIBSQL_TOKEN="your-token"
DATABASE_URL="libsql://db.example.com?authToken=${LIBSQL_TOKEN}"

# ✅ Or secrets management
DATABASE_URL=$(vault kv get -field=database_url secret/station)
```

---

## Option 2: Continuous Backup (Litestream)

Litestream provides continuous replication of SQLite databases to cloud storage (S3, GCS, Azure Blob).

### What is Litestream?

Litestream is a standalone streaming replication tool for SQLite that:
- Continuously backs up database changes to cloud storage
- Restores from backup automatically on startup
- Provides point-in-time recovery
- Minimal performance overhead (<1% CPU)

### Setup

#### 1. Create Litestream Configuration

```yaml
# litestream.yml
dbs:
  - path: /data/station.db
    replicas:
      # S3 (AWS/MinIO/DigitalOcean Spaces)
      - type: s3
        bucket: ${LITESTREAM_S3_BUCKET}
        path: station-db
        region: ${LITESTREAM_S3_REGION:-us-east-1}
        access-key-id: ${LITESTREAM_S3_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_S3_SECRET_ACCESS_KEY}
        sync-interval: 10s    # How often to sync
        retention: 24h        # Keep backups for 24 hours
```

#### 2. Run with Docker

Station's production Docker image includes Litestream integration:

```bash
docker run \
  -v ./litestream.yml:/config/litestream.yml:ro \
  -e LITESTREAM_S3_BUCKET=my-station-backups \
  -e LITESTREAM_S3_ACCESS_KEY_ID=your-access-key \
  -e LITESTREAM_S3_SECRET_ACCESS_KEY=your-secret-key \
  -e LITESTREAM_S3_REGION=us-east-1 \
  ghcr.io/cloudshipai/station:production
```

#### 3. How It Works

**On Startup:**
1. Station production entrypoint checks if `/data/station.db` exists
2. If missing, Litestream restores from S3 backup
3. If no backup exists, creates fresh database
4. Starts Litestream replication in background
5. Starts Station server

**During Operation:**
- Litestream continuously monitors database for changes
- Syncs WAL (write-ahead log) files to S3 every 10 seconds
- Creates periodic snapshots for faster recovery

**On Failure:**
- Next deployment automatically restores from S3
- Zero data loss (up to last sync interval)

### Cloud Storage Options

#### AWS S3
```yaml
- type: s3
  bucket: my-station-backups
  path: station-db
  region: us-east-1
  access-key-id: ${AWS_ACCESS_KEY_ID}
  secret-access-key: ${AWS_SECRET_ACCESS_KEY}
```

#### Google Cloud Storage
```yaml
- type: gcs
  bucket: my-station-backups
  path: station-db
  service-account-json-path: /secrets/gcs-service-account.json
```

#### Azure Blob Storage
```yaml
- type: abs
  bucket: my-station-backups
  path: station-db
  account-name: ${AZURE_STORAGE_ACCOUNT}
  account-key: ${AZURE_STORAGE_KEY}
```

#### Local File (Development/Testing)
```yaml
- type: file
  path: /backup/station-db
  sync-interval: 30s
  retention: 168h  # 7 days
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:production
    volumes:
      - station-data:/data
      - ./litestream.yml:/config/litestream.yml:ro
    environment:
      # Litestream S3 configuration
      - LITESTREAM_S3_BUCKET=my-station-backups
      - LITESTREAM_S3_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - LITESTREAM_S3_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - LITESTREAM_S3_REGION=us-east-1
      # Station configuration
      - DATABASE_PATH=/data/station.db
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    ports:
      - "8585:8585"
      - "2222:2222"
    restart: unless-stopped

volumes:
  station-data:
```

### Kubernetes StatefulSet Example

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: station
spec:
  serviceName: station
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
        image: ghcr.io/cloudshipai/station:production
        env:
        - name: LITESTREAM_S3_BUCKET
          value: station-backups
        - name: LITESTREAM_S3_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: access-key-id
        - name: LITESTREAM_S3_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: secret-access-key
        volumeMounts:
        - name: data
          mountPath: /data
        - name: litestream-config
          mountPath: /config
          readOnly: true
      volumes:
      - name: litestream-config
        configMap:
          name: litestream-config
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

### Manual Restore

If you need to restore to a specific point in time:

```bash
# List available backups
litestream snapshots -config litestream.yml /data/station.db

# Restore to specific timestamp
litestream restore \
  -config litestream.yml \
  -timestamp 2024-11-11T10:00:00Z \
  /data/station.db
```

### Performance

- **Sync Interval:** 1-60 seconds (configurable)
- **CPU Overhead:** <1%
- **Storage:** Incremental WAL files + periodic snapshots
- **Restore Time:** ~1 second for small DBs (<100MB)

---

## Comparison

| Feature | Local SQLite | libsql Cloud | Litestream |
|---------|-------------|--------------|------------|
| **Setup Complexity** | None | Medium | Medium |
| **Multi-Instance** | ❌ No | ✅ Yes | ⚠️ Manual failover |
| **Team Collaboration** | ❌ No | ✅ Yes | ❌ No |
| **Disaster Recovery** | ❌ None | ✅ Built-in | ✅ Excellent |
| **Cost** | Free | Varies | S3 storage only |
| **Performance** | Fastest | Fast (<10ms) | Fastest (local) |
| **Use Case** | Local dev | Production, teams | Production, single instance |

---

## Recommended Configurations

### Local Development
```bash
# Use default local SQLite
stn stdio
```

### Single-Server Production
```bash
# Use Litestream for backup/recovery
docker-compose up  # with litestream.yml
```

### Multi-Server Production
```bash
# Use libsql cloud database
DATABASE_URL="libsql://prod-db.example.com?authToken=xxx" stn stdio
```

### Team Collaboration
```bash
# Shared libsql database
DATABASE_URL="libsql://team-db.example.com?authToken=xxx" stn stdio
```

---

## Troubleshooting

### libsql Connection Issues

**Problem:** `failed to connect to libsql database`

**Solutions:**
1. Verify auth token is correct
2. Check network connectivity to database host
3. Ensure firewall allows outbound HTTPS
4. Verify database URL format: `libsql://host?authToken=token`

### Litestream Restore Failures

**Problem:** Database not restored from backup

**Solutions:**
1. Check S3 credentials are correct
2. Verify bucket exists and is accessible
3. Review Litestream logs: `docker logs station`
4. Manually test restore: `litestream restore -config litestream.yml /data/station.db`

### Performance Issues

**Problem:** Slow queries with cloud database

**Solutions:**
1. Check database region matches deployment region
2. Increase connection pool size (default: 25)
3. Monitor network latency
4. Consider using Litestream for single-instance deployments

---

## Additional Resources

- **libsql Specification:** https://github.com/libsql/libsql
- **Litestream Documentation:** https://litestream.io/
- **Station Configuration:** See `docs/station/CONFIGURATION.md`
- **GitOps Workflow:** See `docs/station/GITOPS_WORKFLOW.md`
