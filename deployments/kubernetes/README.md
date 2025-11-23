# Station Kubernetes Deployment

Complete Kubernetes deployment manifests for Station AI agent platform.

## Overview

This deployment uses:
- **PersistentVolumeClaim** for database storage (REQUIRED)
- **ConfigMap** for non-sensitive configuration
- **Secret** for API keys and encryption key
- **Deployment** with health checks and resource limits
- **Service** for internal and external access

## Critical Requirements

### 1. Persistent Volume is MANDATORY

Station requires a persistent volume mounted to `/data` for the SQLite database. Without this, the container will crash with:

```
Error: failed to initialize database: failed to create database directory /data: mkdir /data: permission denied
```

### 2. API Keys (Provider-Specific)

Station requires API keys based on your chosen AI provider:

- **OpenAI**: Set `OPENAI_API_KEY` in secret
- **Gemini**: Set `GEMINI_API_KEY` or `GOOGLE_API_KEY` in secret
- **Both**: Can configure multiple providers for different agents

**Note**: As of v0.1.0+ (2025-11-23), GEMINI_API_KEY is only required if using Gemini as your provider. BenchmarkService now uses lazy initialization and won't fail startup if credentials are missing.

### 3. STN_DEV_MODE Required for API Access

Station's HTTP API is disabled by default in production. Set `STN_DEV_MODE=true` to enable the API server on port 8585.

## Quick Start

### 1. Create Namespace

```bash
kubectl apply -f namespace.yaml
```

### 2. Configure Secrets

Edit `secret.yaml` and set your API keys:

```bash
# Generate encryption key
ENCRYPTION_KEY=$(openssl rand -base64 32)

# Edit secret.yaml with your values
vim secret.yaml

# Apply secret
kubectl apply -f secret.yaml
```

### 3. Deploy Station

```bash
# Apply all manifests
kubectl apply -f configmap.yaml
kubectl apply -f pvc.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

### 4. Verify Deployment

```bash
# Check pods
kubectl get pods -n station

# Check health
kubectl exec -n station deployment/station -- curl -f http://localhost:8585/health

# View logs
kubectl logs -n station deployment/station -f
```

## Accessing Station

### Internal Access (within cluster)

```bash
# API endpoint
http://station.station.svc.cluster.local:8585

# MCP endpoint
http://station.station.svc.cluster.local:8586
```

### External Access

#### Option 1: Port Forward (Development)

```bash
kubectl port-forward -n station svc/station 8585:8585 8586:8586
```

Then access:
- API: http://localhost:8585
- MCP: http://localhost:8586

#### Option 2: LoadBalancer (Production)

```bash
kubectl apply -f service.yaml  # Creates station-external service
kubectl get svc -n station station-external
```

#### Option 3: Ingress (Production)

Create an Ingress resource:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: station
  namespace: station
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - station.yourdomain.com
    secretName: station-tls
  rules:
  - host: station.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: station
            port:
              number: 8585
```

## Upgrading Station

### Update Agent Configuration

1. **Modify agent configs** in your environment directory
2. **Rebuild image**:
   ```bash
   stn build env <env-name> --tag v2
   ```
3. **Update deployment** to use new image:
   ```bash
   kubectl set image deployment/station station=station-env-name:v2 -n station
   ```
4. **Verify update**:
   ```bash
   kubectl rollout status deployment/station -n station
   kubectl exec -n station deployment/station -- curl http://localhost:8585/api/v1/agents
   ```

**Key Insight**: Agent records update in-place (same ID), config changes propagate automatically. Database persists across deployments.

## Production Considerations

### Database Backup

Station uses SQLite in `/data/station.db`. Recommend using:

**Option 1: Volume Snapshots**

```bash
# Create snapshot (cloud provider specific)
kubectl create volumesnapshot station-backup \
  --source-pvc=station-data \
  --snapshot-class=csi-snapshot-class \
  -n station
```

**Option 2: Litestream Sidecar**

Add Litestream sidecar to deployment for continuous replication to S3:

```yaml
containers:
- name: litestream
  image: litestream/litestream:latest
  args:
    - replicate
    - /data/station.db
    - s3://your-bucket/station.db
  env:
  - name: AWS_ACCESS_KEY_ID
    valueFrom:
      secretKeyRef:
        name: aws-credentials
        key: access-key-id
  - name: AWS_SECRET_ACCESS_KEY
    valueFrom:
      secretKeyRef:
        name: aws-credentials
        key: secret-access-key
  volumeMounts:
  - name: data
    mountPath: /data
```

### Secret Management

**Production Recommendations**:

1. **External Secrets Operator**:
   ```yaml
   apiVersion: external-secrets.io/v1beta1
   kind: ExternalSecret
   metadata:
     name: station-secrets
     namespace: station
   spec:
     secretStoreRef:
       name: aws-secrets-manager
       kind: SecretStore
     target:
       name: station-secrets
     data:
     - secretKey: OPENAI_API_KEY
       remoteRef:
         key: station/openai-api-key
   ```

2. **Sealed Secrets**: Encrypt secrets for GitOps
3. **Vault**: Inject secrets via Vault Agent

### Scaling Limitations

**CRITICAL**: Station uses SQLite which only supports single-instance deployments.

- `replicas: 1` (REQUIRED)
- `strategy: Recreate` (required for PVC ReadWriteOnce)
- Cannot horizontally scale
- For high availability, use database replication (Litestream) and fast recovery

### Resource Sizing

**Development**:
```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "1Gi"
    cpu: "500m"
```

**Production**:
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "250m"
  limits:
    memory: "4Gi"
    cpu: "2000m"
```

Storage: Start with 10Gi, monitor growth.

### Monitoring

**Health Checks**: Already configured in deployment.yaml

**Metrics**: Expose Prometheus metrics:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8585"
  prometheus.io/path: "/metrics"
```

**Logs**: Centralize via Fluentd/Loki:

```bash
kubectl logs -n station deployment/station | grep -E "ERROR|WARN"
```

## Troubleshooting

### Container Crashes on Startup

```bash
# Check logs
kubectl logs -n station deployment/station

# Common issues:
# 1. Missing GEMINI_API_KEY → Add to secret.yaml
# 2. Missing PVC → kubectl get pvc -n station
# 3. Missing secrets → kubectl get secret -n station
```

### Database Permission Denied

```bash
# Verify PVC is mounted
kubectl describe pod -n station $(kubectl get pod -n station -l app=station -o name)

# Check volume mount
# Should see: /data from data (rw)
```

### API Not Accessible

```bash
# Verify STN_DEV_MODE is set
kubectl get deployment -n station station -o yaml | grep STN_DEV_MODE

# Check health endpoint
kubectl exec -n station deployment/station -- curl http://localhost:8585/health

# Verify service
kubectl get svc -n station
```

### Agent Not Updating After Deployment

```bash
# Verify new image is running
kubectl get pod -n station -o yaml | grep image:

# Check agent config
kubectl exec -n station deployment/station -- curl http://localhost:8585/api/v1/agents

# Force sync (if needed)
kubectl exec -n station deployment/station -- stn sync
```

## k3s Specific Notes

k3s uses local-path-provisioner by default:

```yaml
# PVC will automatically use local-path
spec:
  storageClassName: local-path
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

For external access with k3s:

```bash
# Use NodePort instead of LoadBalancer
kubectl patch svc station-external -n station -p '{"spec":{"type":"NodePort"}}'

# Get NodePort
kubectl get svc -n station station-external

# Access via node IP:NodePort
curl http://<node-ip>:<nodeport>/health
```

## Files in This Directory

- `namespace.yaml` - Creates station namespace
- `secret.yaml` - API keys and encryption key (edit before applying!)
- `configmap.yaml` - Non-sensitive configuration
- `pvc.yaml` - Persistent volume claim for database
- `deployment.yaml` - Station deployment with health checks
- `service.yaml` - ClusterIP and LoadBalancer services
- `README.md` - This file

## Next Steps

1. **Deploy to k3s**: Test complete deployment workflow
2. **Test upgrades**: Verify v1→v2 upgrade preserves database
3. **Add Litestream**: Implement continuous backup to S3
4. **Fix GEMINI_API_KEY bug**: Make BenchmarkService optional
5. **Add Ingress**: Production-ready external access with TLS
