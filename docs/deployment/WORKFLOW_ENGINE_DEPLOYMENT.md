# Workflow Engine Deployment Guide

**Last Updated**: December 24, 2025  
**Station Version**: v0.2.0+ (Workflow Engine V1)

---

## Overview

This guide covers deploying Station with the Workflow Engine enabled. The workflow engine uses NATS JetStream for durable message passing between workflow steps, enabling crash-recovery and distributed execution.

**What This Guide Covers:**
- NATS configuration (embedded vs external)
- Docker Compose deployment with workflow support
- Cloud deployment (ECS, GCP, Fly.io)
- Observability setup (OpenTelemetry)
- Troubleshooting workflow-specific issues

**Prerequisites:**
- Station basics understood (see [QUICK_START.md](./QUICK_START.md))
- Familiarity with Docker/Kubernetes
- Understanding of workflow concepts

---

## NATS Configuration

The workflow engine uses NATS JetStream for step scheduling and event delivery. Station supports two modes:

### Embedded NATS (Default)

**Use when:**
- Single Station instance
- Development/testing
- Simple deployments without HA requirements

**Configuration:**
```bash
# Environment variables (all optional - defaults work out of the box)
STATION_NATS_EMBEDDED=true          # Default: true
STATION_NATS_JETSTREAM_ENABLED=true # Default: true
STATION_NATS_DATA_DIR=/data/nats    # Default: ~/.config/station/nats
```

**Behavior:**
- Station starts an embedded NATS server on startup
- JetStream data stored locally
- No external dependencies required
- Port 4222 not exposed (internal only)

**Pros:**
- Zero configuration required
- Works immediately on `stn serve`
- No additional infrastructure

**Cons:**
- Single instance only
- Data not replicated
- State lost if data directory is ephemeral

### External NATS (Production)

**Use when:**
- Multiple Station replicas for HA
- Need durable workflow state across restarts
- Already have NATS infrastructure
- Production deployments

**Configuration:**
```bash
# Environment variables
STATION_NATS_EMBEDDED=false
STATION_NATS_URL=nats://nats:4222
STATION_NATS_JETSTREAM_ENABLED=true
```

**NATS Server Requirements:**
- NATS Server 2.9+ with JetStream enabled
- Persistent storage configured
- Accessible from Station instances

**Minimal NATS Server Config (`nats-server.conf`):**
```conf
port: 4222
http_port: 8222

jetstream {
  store_dir: "/data/jetstream"
  max_memory_store: 1GB
  max_file_store: 10GB
}

# Logging
debug: false
trace: false
logtime: true
```

---

## Docker Compose Deployment

### Basic Setup (Embedded NATS)

**`docker-compose.yml`:**
```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station
    ports:
      - "8585:8585"   # API/UI
      - "8586:8586"   # MCP
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      # Workflow engine enabled by default with embedded NATS
    volumes:
      - ./bundles:/bundles:ro
      - station-data:/root/.config/station
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o-mini --yes 2>/dev/null || true &&
        for bundle in /bundles/*.tar.gz; do
          [ -f \"$$bundle\" ] && stn bundle install \"$$bundle\" \"$$(basename \"$$bundle\" .tar.gz)\" 2>/dev/null || true
        done &&
        stn sync -i=false 2>/dev/null || true &&
        stn serve
      "
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s

volumes:
  station-data:
```

### Production Setup (External NATS)

**`docker-compose.yml`:**
```yaml
version: '3.8'

services:
  nats:
    image: nats:2.10-alpine
    container_name: nats
    command:
      - "--config"
      - "/etc/nats/nats-server.conf"
    ports:
      - "4222:4222"   # Client connections
      - "8222:8222"   # HTTP monitoring
    volumes:
      - ./nats-server.conf:/etc/nats/nats-server.conf:ro
      - nats-data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8222/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3

  station:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      # External NATS configuration
      - STATION_NATS_EMBEDDED=false
      - STATION_NATS_URL=nats://nats:4222
      - STATION_NATS_JETSTREAM_ENABLED=true
    volumes:
      - ./bundles:/bundles:ro
      - station-data:/root/.config/station
    depends_on:
      nats:
        condition: service_healthy
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o-mini --yes 2>/dev/null || true &&
        for bundle in /bundles/*.tar.gz; do
          [ -f \"$$bundle\" ] && stn bundle install \"$$bundle\" \"$$(basename \"$$bundle\" .tar.gz)\" 2>/dev/null || true
        done &&
        stn sync -i=false 2>/dev/null || true &&
        stn serve
      "
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s

volumes:
  station-data:
  nats-data:
```

**`nats-server.conf`:**
```conf
port: 4222
http_port: 8222

jetstream {
  store_dir: "/data/jetstream"
  max_memory_store: 256MB
  max_file_store: 2GB
}

# Logging
debug: false
trace: false
logtime: true

# Workflow streams will be auto-created by Station:
# - WORKFLOW_TASKS: Step scheduling
# - WORKFLOW_EVENTS: Event log
```

### Development Setup with Observability

**`docker-compose.dev.yml`:**
```yaml
version: '3.8'

services:
  nats:
    image: nats:2.10-alpine
    command: ["--jetstream", "-m", "8222"]
    ports:
      - "4222:4222"
      - "8222:8222"
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8222/healthz"]
      interval: 5s
      timeout: 3s
      retries: 3

  jaeger:
    image: jaegertracing/all-in-one:1.52
    container_name: jaeger
    ports:
      - "16686:16686"   # Jaeger UI
      - "4317:4317"     # OTLP gRPC
      - "4318:4318"     # OTLP HTTP
    environment:
      - COLLECTOR_OTLP_ENABLED=true

  station:
    image: ghcr.io/cloudshipai/station:latest
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      # External NATS
      - STATION_NATS_EMBEDDED=false
      - STATION_NATS_URL=nats://nats:4222
      # OpenTelemetry
      - OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317
      - OTEL_SERVICE_NAME=station
      - OTEL_TRACES_EXPORTER=otlp
      - OTEL_METRICS_EXPORTER=otlp
    volumes:
      - station-data:/root/.config/station
    depends_on:
      nats:
        condition: service_healthy
    command: ["stn", "serve"]

volumes:
  station-data:
```

**Start development environment:**
```bash
docker compose -f docker-compose.dev.yml up -d

# Access:
# - Station UI: http://localhost:8585
# - Jaeger UI: http://localhost:16686
# - NATS Monitor: http://localhost:8222
```

---

## Cloud Deployment

### AWS ECS/Fargate

**Task Definition (station-task.json):**
```json
{
  "family": "station-workflow",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "1024",
  "memory": "2048",
  "executionRoleArn": "arn:aws:iam::ACCOUNT:role/ecsTaskExecutionRole",
  "taskRoleArn": "arn:aws:iam::ACCOUNT:role/stationTaskRole",
  "containerDefinitions": [
    {
      "name": "station",
      "image": "ghcr.io/cloudshipai/station:latest",
      "portMappings": [
        {"containerPort": 8585, "protocol": "tcp"},
        {"containerPort": 8586, "protocol": "tcp"}
      ],
      "environment": [
        {"name": "STATION_NATS_EMBEDDED", "value": "true"},
        {"name": "STATION_NATS_DATA_DIR", "value": "/data/nats"}
      ],
      "secrets": [
        {
          "name": "OPENAI_API_KEY",
          "valueFrom": "arn:aws:secretsmanager:us-east-1:ACCOUNT:secret:station/openai-key"
        }
      ],
      "mountPoints": [
        {
          "sourceVolume": "station-data",
          "containerPath": "/root/.config/station"
        },
        {
          "sourceVolume": "nats-data",
          "containerPath": "/data/nats"
        }
      ],
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:8585/health || exit 1"],
        "interval": 30,
        "timeout": 10,
        "retries": 5,
        "startPeriod": 60
      },
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/station-workflow",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "station"
        }
      }
    }
  ],
  "volumes": [
    {
      "name": "station-data",
      "efsVolumeConfiguration": {
        "fileSystemId": "fs-12345678",
        "rootDirectory": "/station"
      }
    },
    {
      "name": "nats-data",
      "efsVolumeConfiguration": {
        "fileSystemId": "fs-12345678",
        "rootDirectory": "/nats"
      }
    }
  ]
}
```

**For production with external NATS:**
- Use Amazon MQ for NATS (if available) or run NATS on EC2
- Update `STATION_NATS_URL` to point to NATS cluster
- Set `STATION_NATS_EMBEDDED=false`

### Google Cloud Run

**Deployment command:**
```bash
gcloud run deploy station-workflow \
  --image ghcr.io/cloudshipai/station:latest \
  --platform managed \
  --region us-central1 \
  --port 8585 \
  --memory 2Gi \
  --cpu 2 \
  --min-instances 1 \
  --max-instances 10 \
  --set-env-vars "STATION_NATS_EMBEDDED=true" \
  --set-secrets "OPENAI_API_KEY=openai-api-key:latest" \
  --allow-unauthenticated
```

**Note:** Cloud Run is stateless. For durable workflows:
1. Use Cloud Memorystore for Redis-backed state, or
2. Connect to external NATS (GKE-hosted or Synadia Cloud)

### Fly.io

**`fly.toml`:**
```toml
app = "station-workflow"
primary_region = "ord"

[build]
image = "ghcr.io/cloudshipai/station:latest"

[env]
STATION_NATS_EMBEDDED = "true"
STATION_NATS_DATA_DIR = "/data/nats"

[mounts]
source = "station_data"
destination = "/root/.config/station"

[[mounts]]
source = "nats_data"
destination = "/data/nats"

[[services]]
internal_port = 8585
protocol = "tcp"

[[services.ports]]
handlers = ["http"]
port = 80

[[services.ports]]
handlers = ["tls", "http"]
port = 443

[[services.http_checks]]
interval = "30s"
timeout = "10s"
path = "/health"
```

**Deploy:**
```bash
# Create volumes
fly volumes create station_data --size 10 --region ord
fly volumes create nats_data --size 5 --region ord

# Set secrets
fly secrets set OPENAI_API_KEY="sk-..."

# Deploy
fly deploy

# Check status
fly status
fly logs
```

---

## Observability

### OpenTelemetry Configuration

The workflow engine emits OpenTelemetry traces and metrics automatically when configured.

**Environment Variables:**
```bash
# Enable OTLP export
OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4317
OTEL_SERVICE_NAME=station

# Trace settings
OTEL_TRACES_EXPORTER=otlp
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # 10% sampling in production

# Metrics settings
OTEL_METRICS_EXPORTER=otlp
```

### Metrics Exposed

The workflow engine exposes the following metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `station_workflow_runs_total` | Counter | Total workflow runs started |
| `station_workflow_run_duration_seconds` | Histogram | Duration of workflow runs |
| `station_workflow_steps_total` | Counter | Total steps executed |
| `station_workflow_step_duration_seconds` | Histogram | Duration of step execution |
| `station_workflow_runs_active` | UpDownCounter | Currently active runs |
| `station_workflow_failures_total` | Counter | Total failures (runs + steps) |

**Labels:**
- `workflow.name` - Workflow identifier
- `workflow.step_type` - Step type (agent, switch, parallel, etc.)
- `workflow.status` - Run/step status

### Trace Propagation

Traces propagate across NATS message boundaries using W3C trace context headers:

```
Workflow Run Span
├── Step: check_pods (agent)
│   └── Agent Execution
├── Step: analyze (switch)
├── Step: parallel_diagnostics
│   ├── Branch: kubernetes
│   ├── Branch: logs
│   └── Branch: metrics
└── Step: report (agent)
```

**NATS Message Format (with trace):**
```json
{
  "step": { "id": "step-123", "type": "agent", ... },
  "trace_context": {
    "traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
  }
}
```

### Grafana Dashboard

Example Grafana dashboard queries for workflow monitoring:

**Workflow Success Rate:**
```promql
sum(rate(station_workflow_runs_total{status="completed"}[5m])) 
/ 
sum(rate(station_workflow_runs_total[5m])) * 100
```

**Average Step Duration by Type:**
```promql
histogram_quantile(0.95, 
  sum(rate(station_workflow_step_duration_seconds_bucket[5m])) by (le, step_type)
)
```

**Active Workflows:**
```promql
sum(station_workflow_runs_active) by (workflow_name)
```

---

## Troubleshooting

### Workflow Stuck in RUNNING

**Symptoms:**
- Workflow shows "running" but no progress
- Steps not executing

**Diagnosis:**
```bash
# Check NATS connectivity
docker compose exec station curl -s http://nats:8222/healthz

# Check JetStream streams
docker compose exec station nats stream list

# Check consumer status
docker compose exec station nats consumer info WORKFLOW_TASKS station-consumer
```

**Solutions:**
1. **NATS not connected:** Check `STATION_NATS_URL` configuration
2. **Consumer lagging:** Increase consumer `MaxAckPending`
3. **Step timeout:** Check step `timeoutSeconds` settings

### Workflow Not Resuming After Restart

**Symptoms:**
- Workflow was running before restart
- After restart, workflow still shows "running" but no steps execute

**Diagnosis:**
```bash
# Check if run has pending steps
sqlite3 ~/.config/station/station.db '
  SELECT r.id, r.status, s.id, s.status 
  FROM workflow_runs r 
  LEFT JOIN workflow_run_steps s ON r.id = s.run_id 
  WHERE r.status = "running"
'

# Check NATS for pending messages
docker compose exec station nats consumer info WORKFLOW_TASKS station-consumer
```

**Solutions:**
1. **Messages expired:** Increase JetStream `MaxAge` setting
2. **Consumer deleted:** Station recreates on startup, but may miss old messages
3. **Data loss:** If using embedded NATS without persistent volume, data is lost

### Step Failed with Schema Validation Error

**Symptoms:**
- Step shows `SCHEMA_VALIDATION_INPUT` error
- Agent never executed

**Diagnosis:**
```bash
# Check step error in database
sqlite3 ~/.config/station/station.db '
  SELECT id, name, status, error 
  FROM workflow_run_steps 
  WHERE run_id = "your-run-id"
'
```

**Solutions:**
1. Verify previous step output matches expected schema
2. Check agent's `input_schema` in dotprompt
3. Use `inject` state to transform data between steps

### NATS Connection Refused

**Symptoms:**
- `connection refused` errors in logs
- Workflow engine not starting

**Diagnosis:**
```bash
# Test NATS connectivity
nc -zv nats 4222

# Check NATS server logs
docker compose logs nats
```

**Solutions:**
1. **NATS not ready:** Add `depends_on` with health check condition
2. **Wrong URL:** Verify `STATION_NATS_URL` format (`nats://host:port`)
3. **Firewall:** Ensure port 4222 is accessible

---

## Best Practices

### Production Checklist

- [ ] Use external NATS for durability
- [ ] Configure persistent volumes for NATS JetStream data
- [ ] Set up OpenTelemetry for observability
- [ ] Configure health checks with appropriate timeouts
- [ ] Set resource limits (CPU/memory)
- [ ] Enable log rotation
- [ ] Configure backup for SQLite database
- [ ] Set up alerting on `station_workflow_failures_total`

### Performance Tuning

**For high-throughput workflows:**
```bash
# Increase NATS batch size
STATION_NATS_CONSUMER_BATCH_SIZE=100

# Increase concurrent step execution
STATION_WORKFLOW_MAX_CONCURRENT_STEPS=10
```

**For long-running workflows:**
```bash
# Increase step timeout (default: 5 minutes)
STATION_WORKFLOW_STEP_TIMEOUT=30m

# Increase run timeout (default: 24 hours)
STATION_WORKFLOW_RUN_TIMEOUT=72h
```

### High Availability

For HA deployments with multiple Station replicas:

1. **Use external NATS cluster** (3+ nodes recommended)
2. **Use shared SQLite** via Litestream replication or PostgreSQL
3. **Configure consumer groups** for work distribution
4. **Set up load balancer** with health check routing

---

## Next Steps

- [PRODUCTION_DEPLOYMENT.md](./PRODUCTION_DEPLOYMENT.md) - General Station deployment
- [../features/workflow-engine-v1.md](../features/workflow-engine-v1.md) - Workflow Engine PRD
- [../station/workflows.md](../station/workflows.md) - Workflow authoring guide

---

**Last Updated**: December 24, 2025  
**Contributors**: Station Team
