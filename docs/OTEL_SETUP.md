# OpenTelemetry Integration Guide

Station includes built-in OpenTelemetry (OTEL) distributed tracing that captures complete execution telemetry from agent runs, including:

- **GenKit native spans**: LLM generation, model API calls, dotprompt execution
- **Station custom spans**: Database operations, MCP server lifecycle, agent execution
- **MCP tool spans**: Individual tool calls (AWS Cost Explorer, Stripe, Grafana, etc.)
- **Complete trace hierarchies**: Parent-child relationships showing execution flow

## Quick Start

### Local Development with Jaeger

The fastest way to see traces is using Jaeger locally:

```bash
# Start Jaeger (includes OTLP collector)
make jaeger

# Configure Station to export traces
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Run Station
stn serve

# Run an agent
stn agent run my-agent "Analyze costs"

# View traces
open http://localhost:16686
```

**What you'll see in Jaeger:**
- Service: `station`
- Operations: agent names, `generate`, `openai/gpt-4o-mini`, `mcp.*`, `db.*`
- Complete execution timeline with timing for each operation
- Full input/output payloads in span tags

### Configuration Options

Station supports multiple telemetry providers via the `telemetry:` config section:

**Config File (Recommended)**
```yaml
# ~/.config/station/config.yaml

telemetry:
  enabled: true
  provider: jaeger       # Options: none, jaeger, otlp, cloudship
  endpoint: "http://localhost:4318"
  service_name: station
  environment: development
  sample_rate: 1.0       # 1.0 = sample all, 0.1 = sample 10%
```

**Provider Options:**
- `none` - Disable telemetry export
- `jaeger` - Local Jaeger at http://localhost:4318 (default)
- `otlp` - Custom OTLP endpoint with optional authentication
- `cloudship` - CloudShip managed telemetry (uses registration key)

**Bring Your Own OTLP Backend (Grafana Cloud, Datadog, etc.)**
```yaml
telemetry:
  enabled: true
  provider: otlp
  endpoint: "https://otlp-gateway-prod-us-central-0.grafana.net/otlp"
  headers:
    Authorization: "Basic <base64(user:token)>"
  service_name: station
  environment: production
  sample_rate: 0.1       # Sample 10% in production
```

**Environment Variables (Override config file)**
```bash
# Enable/disable telemetry
export STN_TELEMETRY_ENABLED=true

# Set provider
export STN_TELEMETRY_PROVIDER=otlp

# Set endpoint
export STN_TELEMETRY_ENDPOINT=http://localhost:4318

# Optional overrides
export STN_TELEMETRY_SERVICE_NAME=station-prod
export STN_TELEMETRY_ENVIRONMENT=production
export STN_TELEMETRY_SAMPLE_RATE=0.1
export STN_TELEMETRY_JAEGER_QUERY_URL=http://localhost:16686
```

**Legacy Configuration (still supported)**
```bash
# These still work for backward compatibility
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_SERVICE_NAME=station
```

## Team Integration Examples

### Jaeger (Open Source)

**Setup:**
```bash
# Using Docker
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Using Kubernetes (Jaeger Operator)
kubectl create namespace observability
kubectl create -f https://github.com/jaegertracing/jaeger-operator/releases/latest/download/jaeger-operator.yaml -n observability

# Create Jaeger instance
kubectl apply -f - <<EOF
apiVersion: jaegertracing/v1
kind: Jaeger
metadata:
  name: station-tracing
  namespace: observability
spec:
  strategy: allInOne
  ingress:
    enabled: true
EOF
```

**Station Configuration:**
```bash
# Point to Jaeger OTLP collector
export OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger-collector:4318
```

**Access:**
- UI: `http://jaeger-query:16686`
- Query API: `http://jaeger-query:16686/api/traces?service=station`

---

### Grafana Tempo

**Setup:**
```yaml
# docker-compose.yml
version: '3'
services:
  tempo:
    image: grafana/tempo:latest
    command: [ "-config.file=/etc/tempo.yaml" ]
    volumes:
      - ./tempo.yaml:/etc/tempo.yaml
      - tempo-data:/tmp/tempo
    ports:
      - "4317:4317"  # OTLP gRPC
      - "4318:4318"  # OTLP HTTP

  grafana:
    image: grafana/grafana:latest
    environment:
      - GF_AUTH_ANONYMOUS_ENABLED=true
      - GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
    ports:
      - "3000:3000"
    volumes:
      - grafana-data:/var/lib/grafana

volumes:
  tempo-data:
  grafana-data:
```

**tempo.yaml:**
```yaml
server:
  http_listen_port: 3200

distributor:
  receivers:
    otlp:
      protocols:
        http:
        grpc:

storage:
  trace:
    backend: local
    local:
      path: /tmp/tempo/traces

query_frontend:
  search:
    enabled: true
```

**Station Configuration:**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo:4318
```

**Grafana Setup:**
1. Add Tempo as data source: `http://tempo:3200`
2. Use TraceQL to query: `{service.name="station"}`
3. Create dashboards showing agent execution metrics

---

### Datadog

**Setup:**
```bash
# Install Datadog Agent
DD_API_KEY=<your-key> DD_SITE="datadoghq.com" bash -c "$(curl -L https://s3.amazonaws.com/dd-agent/scripts/install_script_agent7.sh)"

# Enable OTLP ingestion in datadog.yaml
otlp_config:
  receiver:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
```

**Station Configuration:**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Optional: Add Datadog-specific tags
export OTEL_RESOURCE_ATTRIBUTES="env:production,team:infrastructure,service.name:station"
```

**Datadog Console:**
1. Navigate to APM → Traces
2. Filter by service: `station`
3. See agent executions as traces with full span details
4. Create monitors for agent execution latency/errors

---

### Honeycomb

**Setup:**
```bash
# No collector needed - send directly to Honeycomb
export OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io:443
export OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=YOUR_API_KEY"
```

**Station Configuration:**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io:443
export OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=YOUR_API_KEY"
export OTEL_SERVICE_NAME=station
```

**Honeycomb Queries:**
```
# Agent execution latency
AVG(duration_ms) WHERE name = "agent_execution_engine.execute"

# Tool call success rate
COUNT_DISTINCT(trace.trace_id) WHERE genkit.state = "success" / COUNT_DISTINCT(trace.trace_id)

# Expensive operations
HEATMAP(duration_ms) GROUP BY name
```

---

### AWS X-Ray

**Setup:**
```bash
# Install AWS Distro for OpenTelemetry Collector
# https://aws-otel.github.io/docs/getting-started/collector

# Run ADOT Collector with X-Ray exporter
docker run --rm -p 4317:4317 -p 4318:4318 \
  -e AWS_REGION=us-east-1 \
  public.ecr.aws/aws-observability/aws-otel-collector:latest \
  --config=/etc/otel-collector-config.yaml
```

**otel-collector-config.yaml:**
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  awsxray:
    region: us-east-1

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [awsxray]
```

**Station Configuration:**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export AWS_REGION=us-east-1
```

**X-Ray Console:**
1. Navigate to X-Ray → Traces
2. Filter by service: `station`
3. View service map showing agent → MCP → LLM dependencies

---

### New Relic

**Setup:**
```bash
# Direct OTLP export to New Relic
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
export OTEL_EXPORTER_OTLP_HEADERS="api-key=YOUR_LICENSE_KEY"
```

**Station Configuration:**
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
export OTEL_EXPORTER_OTLP_HEADERS="api-key=YOUR_NEW_RELIC_LICENSE_KEY"
export OTEL_SERVICE_NAME=station
```

**New Relic Queries (NRQL):**
```sql
-- Agent execution latency
SELECT average(duration.ms) FROM Span
WHERE service.name = 'station'
AND name LIKE 'agent_execution%'
FACET agent.name TIMESERIES

-- Error rate
SELECT percentage(count(*), WHERE otel.status_code = 'ERROR')
FROM Span WHERE service.name = 'station'
```

---

### Azure Monitor

**Setup:**
```bash
# Install Azure Monitor OpenTelemetry Exporter
# https://learn.microsoft.com/azure/azure-monitor/app/opentelemetry-enable
```

**Station Configuration:**
```bash
export APPLICATIONINSIGHTS_CONNECTION_STRING="InstrumentationKey=YOUR_KEY;IngestionEndpoint=https://YOUR_REGION.in.applicationinsights.azure.com/"
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Run OTEL Collector with Azure Monitor exporter
```

---

## Kubernetes Deployment

**Recommended Architecture:**
```
┌─────────────────┐
│  Station Pod    │
│  ┌───────────┐  │
│  │ stn serve │──┼──┐
│  └───────────┘  │  │ OTLP
└─────────────────┘  │
                     │
                     ▼
            ┌────────────────┐
            │ OTEL Collector │──► Jaeger/Tempo/Datadog
            │   (DaemonSet)  │
            └────────────────┘
```

**Station Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station
spec:
  template:
    spec:
      containers:
      - name: station
        image: your-registry/station:latest
        env:
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://$(HOST_IP):4318"
        - name: HOST_IP
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
        - name: OTEL_SERVICE_NAME
          value: "station"
        - name: OTEL_RESOURCE_ATTRIBUTES
          value: "k8s.pod.name=$(POD_NAME),k8s.namespace.name=$(POD_NAMESPACE)"
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

**OTEL Collector DaemonSet:**
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: otel-collector
spec:
  template:
    spec:
      containers:
      - name: otel-collector
        image: otel/opentelemetry-collector-contrib:latest
        ports:
        - containerPort: 4318
          hostPort: 4318
          protocol: TCP
```

---

## What Gets Traced

### Station Custom Spans

**Database Operations:**
- `db.agent_runs.create` - Creating agent run record
- `db.agent_runs.get_by_id` - Fetching run details
- `db.agent_runs.update_completion` - Updating run with results
- Tags: `agent.name`, `agent.id`, `run.id`

**MCP Infrastructure:**
- `mcp.server.start` - MCP server initialization
- `mcp.client.create_and_discover_tools` - Tool discovery
- Tags: `server.name`, `tool.count`, `environment.id`

**Agent Execution:**
- `agent_execution_engine.execute` - Main agent execution span
- Tags: `agent.name`, `agent.id`, `run.id`, `max_steps`

### GenKit Native Spans

**LLM Operations:**
- `generate` (util) - GenKit's core generation logic
- `openai/gpt-4o-mini` (action) - OpenAI API calls
- `{agent-name}` (executable-prompt) - Root dotprompt span
- Tags: `genkit:input`, `genkit:output`, `genkit:state`, `genkit:path`

**MCP Tool Calls:**
- `__get_cost_and_usage` - AWS Cost Explorer
- `__list_deployments` - GitHub deployments
- `__query_prometheus` - Prometheus queries
- Tags: `genkit:type=action`, `genkit:state=success/error`

### Trace Hierarchy Example

```
aws-cost-spike-analyzer (18.2s)
├─ generate (17ms)
│  ├─ openai/gpt-4o-mini (11ms) - "Analyze this cost data"
│  └─ __get_cost_anomalies (0ms) - Tool call
├─ generate (11ms)
│  ├─ openai/gpt-4o-mini (8ms) - "Compare periods"
│  └─ __get_cost_and_usage_comparisons (0ms)
├─ generate (12ms)
│  └─ __get_cost_drivers (0ms)
├─ db.agent_runs.create (0.1ms)
├─ db.agent_runs.update_completion (0.2ms)
└─ mcp.server.start (x7) - One per MCP server
```

---

## Querying Traces

### Via Jaeger API

```bash
# Get all traces for service
curl "http://localhost:16686/api/traces?service=station&limit=20"

# Find agent executions
curl "http://localhost:16686/api/traces?service=station&operation=aws-cost-spike-analyzer"

# Get services
curl "http://localhost:16686/api/services"
```

### Via TraceQL (Tempo)

```traceql
# Find expensive operations
{ service.name="station" && duration > 10s }

# Find failed executions
{ service.name="station" && genkit.state="error" }

# Tool call success rate
{ service.name="station" && genkit.type="action" }
```

### Via Datadog UI

```
service:station operation_name:agent_execution_engine.execute
service:station @genkit.type:action
service:station @agent.name:aws-cost-spike-analyzer
```

---

## Troubleshooting

### No Traces Appearing

**Check OTEL endpoint:**
```bash
# Verify endpoint is reachable
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{"resourceSpans":[]}'

# Should return 200 OK
```

**Check Station logs:**
```bash
export STN_DEBUG=true
stn serve

# Look for:
# "OTEL telemetry initialized successfully"
```

**Verify configuration:**
```bash
# Check environment variables
echo $STN_TELEMETRY_PROVIDER
echo $STN_TELEMETRY_ENDPOINT

# Or check config file
cat ~/.config/station/config.yaml | grep -A5 telemetry
```

### Partial Traces

**Issue**: Only some spans appear, trace is fragmented

**Cause**: Spans exported to different backends or trace context not propagated

**Solution**: Ensure consistent OTEL configuration across all processes

### High Overhead

**Issue**: OTEL adding significant latency

**Cause**: Default settings export all spans

**Solution**: Configure sampling
```bash
# Sample 10% of traces
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1
```

---

## Advanced Configuration

### Custom Span Attributes

Station automatically adds these attributes to spans:
- `agent.name` - Agent name
- `agent.id` - Agent database ID
- `run.id` - Execution run ID
- `agent.environment` - Environment ID
- `execution.model` - AI model used (e.g., openai/gpt-4o-mini)
- `execution.duration_seconds` - Total execution time
- `execution.success` - Whether execution succeeded
- `execution.steps_taken` - Number of agent steps
- `execution.tools_used` - Number of tool calls
- `task.preview` - First 100 chars of the task
- `genkit.*` - GenKit native attributes

**CloudShip Attributes (when connected):**
- `cloudship.org_id` - Organization ID for multi-tenant filtering
- `cloudship.station_id` - Station UUID
- `cloudship.station_name` - Human-readable station name

These CloudShip attributes are automatically injected when Station authenticates with Lighthouse, enabling you to filter traces by organization in Grafana Cloud or other backends.

### Sampling Strategies

**Always sample (default):**
```bash
export OTEL_TRACES_SAMPLER=always_on
```

**Probabilistic sampling:**
```bash
export OTEL_TRACES_SAMPLER=traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.5  # 50% sampling
```

**Parent-based sampling:**
```bash
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1  # 10% if no parent
```

### Batch Configuration

```bash
# Max batch size (default: 512)
export OTEL_BSP_MAX_EXPORT_BATCH_SIZE=256

# Batch timeout (default: 5s)
export OTEL_BSP_SCHEDULE_DELAY=2000
```

---

## Testing OTEL Integration

```bash
# Run integration tests
make test-otel

# Start Jaeger and run full test
make jaeger
make test-otel-e2e

# Manual verification
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
stn agent run test-agent "Test task"

# Query for traces
curl -s "http://localhost:16686/api/traces?service=station&limit=1" | \
  jq '.data[0].spans[].operationName'
```

---

## Security Considerations

1. **Network Security**: OTLP endpoints should be on private networks or use TLS
2. **Sensitive Data**: Span attributes may contain sensitive info - configure scrubbing
3. **Authentication**: Use headers for authentication when exporting to SaaS platforms
4. **Compliance**: Ensure trace retention policies meet compliance requirements

---

## Resources

- [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/otel/)
- [OTLP Protocol](https://opentelemetry.io/docs/specs/otlp/)
- [GenKit Telemetry](https://firebase.google.com/docs/genkit/observability)
- [Station Architecture](./station/architecture.md)

---

**Questions?** Open an issue at https://github.com/cloudshipai/station/issues
