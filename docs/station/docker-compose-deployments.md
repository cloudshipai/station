# Docker Compose Deployments

Production deployment patterns using Docker Compose with Station agents. Deploy AI agent environments as containerized services with automatic configuration, environment isolation, and zero-config setup.

## Overview

Station supports production deployment via Docker Compose with:
- **Zero-Config Deployment** - Automatic bundle installation and MCP server configuration
- **Environment Variables** - Inject secrets and configuration at runtime
- **Volume Persistence** - Agent data and configurations survive container restarts
- **Health Checks** - Automatic service health monitoring
- **Multi-Environment** - Run dev/staging/prod with different configurations

## Quick Start

### Deploy Station with Bundles

```bash
# Step 1: Create bundles directory with agent bundles
mkdir bundles
cp security-scanner.tar.gz bundles/

# Step 2: Create docker-compose.yml (see examples below)

# Step 3: Set environment variables
export OPENAI_API_KEY="sk-..."
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."

# Step 4: Deploy
docker-compose up -d

# Step 5: Access Station
open http://localhost:8585
```

## Zero-Config Deployment

The zero-config pattern automatically installs bundles and configures MCP servers on container startup.

### docker-compose.yml (Zero-Config)

```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-zero-config
    ports:
      - "8585:8585"  # API/UI port
      - "8586:8586"  # MCP port
    environment:
      # AI Provider Configuration (required)
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      # or use other providers:
      # - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      # - GOOGLE_API_KEY=${GOOGLE_API_KEY}

      # AWS Configuration (if using AWS MCP servers)
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - AWS_REGION=${AWS_REGION:-us-east-1}

      # E2B Configuration (if using E2B for code execution)
      - E2B_API_KEY=${E2B_API_KEY}

      # CloudShip Configuration (optional)
      - STN_CLOUDSHIP_ENABLED=${STN_CLOUDSHIP_ENABLED:-false}
      - STN_CLOUDSHIP_ENDPOINT=${STN_CLOUDSHIP_ENDPOINT:-lighthouse.cloudshipai.com:50051}
      - STN_CLOUDSHIP_KEY=${STN_CLOUDSHIP_KEY}
      - STN_CLOUDSHIP_STATION_ID=${STN_CLOUDSHIP_STATION_ID}

      # Station Configuration
      - STATION_ENCRYPTION_KEY=${STATION_ENCRYPTION_KEY:-auto-generated-on-init}
    volumes:
      # Mount bundles directory for zero-config deployment
      - ./bundles:/bundles:ro
      # Persist Station data
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

### Usage

```bash
# Directory structure
.
├── docker-compose.yml
└── bundles/
    ├── security-scanner.tar.gz
    ├── cost-analyzer.tar.gz
    └── deployment-validator.tar.gz

# Deploy with environment variables
OPENAI_API_KEY="sk-..." \
AWS_ACCESS_KEY_ID="AKIA..." \
AWS_SECRET_ACCESS_KEY="..." \
docker-compose up -d

# View logs
docker-compose logs -f station

# Check bundle installation
docker-compose exec station stn agent list
```

## Environment-Specific Deployments

### Development Environment

**docker-compose.dev.yml:**
```yaml
version: '3.8'

services:
  station-dev:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-dev
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - STATION_DEBUG=true
      - STATION_TELEMETRY_ENABLED=false
    volumes:
      # Mount local bundles for development
      - ./bundles:/bundles:ro
      # Mount local config for live editing
      - ./dev-config:/root/.config/station
      # Mount host project directory for filesystem MCP access
      - ${HOME}/projects:/workspace:ro
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o-mini --yes &&
        for bundle in /bundles/*.tar.gz; do
          bundle_name=\$(basename \"\$bundle\" .tar.gz) &&
          stn bundle install \"\$bundle\" \"\$bundle_name\" --set PROJECT_ROOT=/workspace &&
          stn sync \"\$bundle_name\" -i=false
        done &&
        stn serve
      "
    restart: unless-stopped

volumes:
  dev-config:
    driver: local
```

**Usage:**
```bash
# Deploy development environment
docker-compose -f docker-compose.dev.yml up -d

# Develop agents locally, reload bundles
docker-compose -f docker-compose.dev.yml restart station-dev
```

### Staging Environment

**docker-compose.staging.yml:**
```yaml
version: '3.8'

services:
  station-staging:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-staging
    ports:
      - "9585:8585"  # Different port for staging
      - "9586:8586"
    environment:
      - OPENAI_API_KEY=${STAGING_OPENAI_API_KEY}
      - AWS_ACCESS_KEY_ID=${STAGING_AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${STAGING_AWS_SECRET_ACCESS_KEY}
      - AWS_REGION=us-west-2
      - STATION_DEBUG=false
      - STATION_TELEMETRY_ENABLED=true
      - STN_CLOUDSHIP_ENABLED=true
      - STN_CLOUDSHIP_KEY=${STAGING_CLOUDSHIP_KEY}
    volumes:
      - ./staging-bundles:/bundles:ro
      - station-staging-data:/root/.config/station
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o --yes &&
        for bundle in /bundles/*.tar.gz; do
          bundle_name=\$(basename \"\$bundle\" .tar.gz) &&
          stn bundle install \"\$bundle\" \"\$bundle_name-staging\" &&
          stn sync \"\$bundle_name-staging\" -i=false
        done &&
        stn serve
      "
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  station-staging-data:
    driver: local
```

**Usage:**
```bash
# Deploy staging environment
STAGING_OPENAI_API_KEY="sk-staging-..." \
STAGING_AWS_ACCESS_KEY_ID="AKIA-staging..." \
STAGING_CLOUDSHIP_KEY="staging-key" \
docker-compose -f docker-compose.staging.yml up -d
```

### Production Environment

**docker-compose.prod.yml:**
```yaml
version: '3.8'

services:
  station-prod:
    image: ghcr.io/cloudshipai/station:v0.2.8  # Pinned version
    container_name: station-prod
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${PROD_OPENAI_API_KEY}
      - AWS_ACCESS_KEY_ID=${PROD_AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${PROD_AWS_SECRET_ACCESS_KEY}
      - AWS_REGION=us-east-1
      - STATION_DEBUG=false
      - STATION_TELEMETRY_ENABLED=true
      - STN_CLOUDSHIP_ENABLED=true
      - STN_CLOUDSHIP_ENDPOINT=lighthouse.cloudshipai.com:50051
      - STN_CLOUDSHIP_KEY=${PROD_CLOUDSHIP_KEY}
      - STN_CLOUDSHIP_STATION_ID=${PROD_CLOUDSHIP_STATION_ID}
      - STATION_ENCRYPTION_KEY=${PROD_ENCRYPTION_KEY}
    volumes:
      - ./production-bundles:/bundles:ro
      - station-prod-data:/root/.config/station
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o --yes &&
        for bundle in /bundles/*.tar.gz; do
          bundle_name=\$(basename \"\$bundle\" .tar.gz) &&
          stn bundle install \"\$bundle\" \"\$bundle_name-prod\" &&
          stn sync \"\$bundle_name-prod\" -i=false
        done &&
        stn serve
      "
    restart: always
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
  station-prod-data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /mnt/station-data  # Production data on mounted disk

networks:
  default:
    driver: bridge
```

**Usage:**
```bash
# Deploy production with secrets from vault/env
PROD_OPENAI_API_KEY=$(vault kv get -field=key secret/openai) \
PROD_AWS_ACCESS_KEY_ID=$(vault kv get -field=access_key secret/aws) \
PROD_AWS_SECRET_ACCESS_KEY=$(vault kv get -field=secret_key secret/aws) \
PROD_CLOUDSHIP_KEY=$(vault kv get -field=key secret/cloudship) \
PROD_ENCRYPTION_KEY=$(vault kv get -field=encryption_key secret/station) \
docker-compose -f docker-compose.prod.yml up -d
```

## Building Custom Environment Containers

Use `stn build env` to create environment-specific containers with all dependencies pre-packaged.

### Build Environment Container

```bash
# Build container for specific environment
stn build env production --provider openai --model gpt-4o

# Build with Ship security tools
stn build env security-scanner --ship

# Build with CloudShip integration
stn build env finops-agents \
  --provider openai \
  --model gpt-4o-mini \
  --cloudshipai-registration-key "your-key"
```

### Custom Dockerfile Deployment

If you've built a custom environment container, deploy it with docker-compose:

**docker-compose.custom.yml:**
```yaml
version: '3.8'

services:
  station-custom:
    image: station-production:latest  # Your custom-built image
    container_name: station-custom
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
    volumes:
      - station-data:/root/.config/station
    command: stn serve
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  station-data:
    driver: local
```

## Multi-Service Stacks

### FinOps Agent Stack with Monitoring

```yaml
version: '3.8'

services:
  station-finops:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-finops
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - AWS_REGION=us-east-1
    volumes:
      - ./bundles/finops:/bundles:ro
      - station-finops-data:/root/.config/station
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o-mini --yes &&
        stn bundle install /bundles/aws-cost-analyzer.tar.gz finops &&
        stn sync finops -i=false &&
        stn serve
      "
    restart: unless-stopped
    networks:
      - finops-network

  # PostgreSQL for cost analytics
  postgres:
    image: postgres:15
    container_name: finops-postgres
    environment:
      - POSTGRES_DB=finops
      - POSTGRES_USER=finops
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - finops-network

  # Grafana for cost dashboards
  grafana:
    image: grafana/grafana:latest
    container_name: finops-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
    volumes:
      - grafana-data:/var/lib/grafana
    networks:
      - finops-network

volumes:
  station-finops-data:
  postgres-data:
  grafana-data:

networks:
  finops-network:
    driver: bridge
```

**Usage:**
```bash
# Deploy FinOps stack
OPENAI_API_KEY="sk-..." \
AWS_ACCESS_KEY_ID="AKIA..." \
POSTGRES_PASSWORD="secure-pass" \
GRAFANA_PASSWORD="admin-pass" \
docker-compose -f docker-compose.finops.yml up -d

# Run cost analysis agent
docker-compose exec station-finops \
  stn agent run aws-cost-analyzer "Analyze last month's AWS costs"
```

### Security Scanner Stack

```yaml
version: '3.8'

services:
  station-security:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-security
    ports:
      - "8585:8585"
      - "8586:8586"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./bundles/security:/bundles:ro
      # Mount project directories for scanning
      - /home/user/projects:/workspace:ro
      - station-security-data:/root/.config/station
    command: >
      sh -c "
        stn init --provider openai --model gpt-4o-mini --yes &&
        stn bundle install /bundles/security-scanner.tar.gz security &&
        stn sync security -i=false --set PROJECT_ROOT=/workspace &&
        stn serve
      "
    restart: unless-stopped

volumes:
  station-security-data:
```

**Usage:**
```bash
# Deploy security scanner
docker-compose -f docker-compose.security.yml up -d

# Run security scan
docker-compose exec station-security \
  stn agent run infrastructure-security-scanner \
  "Scan /workspace/terraform for security issues"
```

## Environment Variable Injection

Station supports multiple ways to inject configuration and secrets:

### Method 1: .env File

```bash
# .env file
OPENAI_API_KEY=sk-...
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
STATION_ENCRYPTION_KEY=...
```

```bash
# docker-compose.yml automatically loads .env
docker-compose up -d
```

### Method 2: Environment File

```bash
# Create environment-specific .env files
cat > .env.production <<EOF
OPENAI_API_KEY=sk-prod-...
AWS_ACCESS_KEY_ID=AKIA-prod...
AWS_SECRET_ACCESS_KEY=...
EOF

# Load specific env file
docker-compose --env-file .env.production up -d
```

### Method 3: Vault Integration

```bash
# Load secrets from Vault
export OPENAI_API_KEY=$(vault kv get -field=key secret/openai)
export AWS_ACCESS_KEY_ID=$(vault kv get -field=access_key secret/aws)
export AWS_SECRET_ACCESS_KEY=$(vault kv get -field=secret_key secret/aws)

docker-compose up -d
```

### Method 4: Runtime Injection

```bash
# Inject variables at runtime
docker-compose run \
  -e OPENAI_API_KEY="sk-..." \
  -e AWS_REGION="us-west-2" \
  station-prod stn serve
```

## Automatic Configuration

Station automatically configures based on environment variables:

### AWS Credentials

Station detects AWS credentials from:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. IAM instance roles (when running on EC2)
3. ECS task roles (when running on ECS/Fargate)
4. Mounted AWS config files

### Database Connections

For agents using database MCP servers:
```yaml
environment:
  - POSTGRES_CONNECTION_STRING=postgresql://user:pass@postgres:5432/db
  - MONGODB_URI=mongodb://mongo:27017/mydb
```

### MCP Server Configuration

Template variables resolved from environment:
```yaml
# template.json uses {{ .PROJECT_ROOT }}
# Injected via environment variable
environment:
  - PROJECT_ROOT=/workspace
```

## Monitoring and Logging

### Health Checks

All production deployments should include health checks:
```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### Logging Configuration

**Structured logging with rotation:**
```yaml
logging:
  driver: "json-file"
  options:
    max-size: "100m"
    max-file: "10"
    labels: "environment,service"
```

**View logs:**
```bash
# Follow logs
docker-compose logs -f station

# View last 100 lines
docker-compose logs --tail 100 station

# Filter logs
docker-compose logs station | grep ERROR
```

### Monitoring with Prometheus

**docker-compose.monitoring.yml:**
```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    ports:
      - "8585:8585"
      - "9090:9090"  # Metrics endpoint
    environment:
      - STATION_METRICS_ENABLED=true

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'

volumes:
  prometheus-data:
```

**prometheus.yml:**
```yaml
scrape_configs:
  - job_name: 'station'
    static_configs:
      - targets: ['station:9090']
```

## Scaling

### Horizontal Scaling with Multiple Instances

```yaml
version: '3.8'

services:
  station-1:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-1
    ports:
      - "8585:8585"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./bundles:/bundles:ro
      - station-1-data:/root/.config/station

  station-2:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-2
    ports:
      - "8586:8585"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./bundles:/bundles:ro
      - station-2-data:/root/.config/station

volumes:
  station-1-data:
  station-2-data:
```

### Load Balancing with Nginx

```yaml
version: '3.8'

services:
  nginx:
    image: nginx:latest
    ports:
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - station-1
      - station-2

  station-1:
    image: ghcr.io/cloudshipai/station:latest
    expose:
      - "8585"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./bundles:/bundles:ro
      - station-1-data:/root/.config/station

  station-2:
    image: ghcr.io/cloudshipai/station:latest
    expose:
      - "8585"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./bundles:/bundles:ro
      - station-2-data:/root/.config/station

volumes:
  station-1-data:
  station-2-data:
```

**nginx.conf:**
```nginx
http {
  upstream station {
    server station-1:8585;
    server station-2:8585;
  }

  server {
    listen 80;
    location / {
      proxy_pass http://station;
    }
  }
}
```

## Troubleshooting

### Container Won't Start

**Problem:** Container exits immediately after starting

**Solution:**
```bash
# Check logs for errors
docker-compose logs station

# Common issues:
# 1. Missing OPENAI_API_KEY
docker-compose up -e OPENAI_API_KEY="sk-..."

# 2. Invalid bundle format
docker-compose exec station ls -la /bundles

# 3. Permission issues
docker-compose exec station ls -la /root/.config/station
```

### Bundle Installation Fails

**Problem:** Bundles not installing during startup

**Solution:**
```bash
# Verify bundle directory mounted
docker-compose exec station ls -la /bundles

# Check bundle format
tar -tzf bundles/security-scanner.tar.gz

# Manual installation inside container
docker-compose exec station stn bundle install /bundles/security-scanner.tar.gz
docker-compose exec station stn sync security-scanner
```

### MCP Servers Not Connecting

**Problem:** Agents can't access MCP tools

**Solution:**
```bash
# Check environment variables resolved
docker-compose exec station stn mcp list

# Verify template variables
docker-compose exec station cat /root/.config/station/environments/my-env/variables.yml

# Re-sync environment
docker-compose exec station stn sync my-env
```

### Health Check Failing

**Problem:** Health check reports unhealthy

**Solution:**
```bash
# Test health endpoint manually
docker-compose exec station curl -f http://localhost:8585/health

# Check Station server status
docker-compose exec station ps aux | grep stn

# Increase start_period if slow startup
# docker-compose.yml:
healthcheck:
  start_period: 60s  # Increased from 40s
```

## Best Practices

### Production Deployment

1. **Pin Image Versions**: Use specific tags (`v0.2.8`) not `latest`
2. **Volume Backups**: Regular backups of `station-data` volume
3. **Secret Management**: Use vault/secrets manager, not plain .env files
4. **Health Checks**: Always configure health checks for auto-restart
5. **Logging**: Configure log rotation to prevent disk fill
6. **Monitoring**: Integrate with Prometheus/CloudWatch for metrics
7. **Resource Limits**: Set CPU/memory limits for containers

### Security

1. **Never commit secrets**: Add `.env*` to `.gitignore`
2. **Use read-only volumes**: Mount bundles as `:ro`
3. **Minimal permissions**: Don't run as root if possible
4. **Network isolation**: Use custom networks for service isolation
5. **Encryption keys**: Generate unique keys per environment

### Performance

1. **Volume drivers**: Use local driver for better I/O performance
2. **Resource allocation**: Allocate sufficient memory for AI operations
3. **Concurrent limits**: Limit concurrent agent executions
4. **Caching**: Use volume mounts to persist downloaded dependencies

## Next Steps

- [Bundles](./bundles.md) - Create and distribute agent bundles
- [Template Variables](./templates.md) - Configure environment-specific variables
- [Zero-Config Deployments](./zero-config-deployments.md) - IAM role-based deployments
- [Deployment Modes](./deployment-modes.md) - Choose the right deployment strategy
