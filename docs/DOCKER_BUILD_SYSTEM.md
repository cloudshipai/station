# Station Docker Build System

## Overview

Station provides a comprehensive Docker build system using Dagger for programmatic container creation. The system supports two deployment strategies:

1. **Base + Config Injection** (Recommended) - Single base image with runtime config mounting
2. **Environment-Specific Images** - Full environment containers with configs baked in

## Container Sizes

- **Base Container**: ~125MB - Station binary + Ubuntu base + essential packages
- **Environment Container**: ~130MB - Base + environment-specific configs (5MB overhead)

## Build Commands

### Base Image Build
```bash
# Build reusable base image
stn build base

# Output: station-base:latest loaded into Docker daemon
# Image ID: sha256:bb1078264080...
```

### Environment-Specific Build
```bash
# Build environment with configs included
stn build env default
stn build env staging  
stn build env production

# Output: station-{env}:latest loaded into Docker daemon
```

## Deployment Architecture

### Option 1: Base + Config Injection (Recommended)

**Benefits:**
- Single 125MB base image for all environments
- External configuration management
- GitOps-ready deployment
- Environment variables for secrets

**Structure:**
```bash
# 1. Build once
stn build base

# 2. Deploy with runtime config injection
docker run -it \
  -v ./staging/variables.yml:/app/environment/variables.yml \
  -v ./staging/config.yml:/app/environment/config.yml \
  -e OPENAI_API_KEY=$STAGING_OPENAI_KEY \
  -e ANTHROPIC_API_KEY=$STAGING_ANTHROPIC_KEY \
  station-base:latest
```

### Option 2: Environment-Specific Images

**Benefits:**
- Self-contained images with configs baked in
- No external dependencies
- Simpler deployment for isolated environments

**Structure:**
```bash
# Build per environment
stn build env staging    # → station-staging:latest
stn build env production # → station-production:latest

# Deploy directly
docker run -it \
  -e OPENAI_API_KEY=$PROD_KEY \
  station-production:latest
```

## Container Structure

```
station-base:latest
├── /usr/local/bin/stn          # Station binary
├── /app/
│   ├── entrypoint.sh            # Smart config detection script
│   ├── environment/             # Mount point for configs
│   │   ├── variables.yml        # Template variables (mounted)
│   │   └── config.yml          # Environment settings (mounted)
│   └── data/
│       └── station.db          # SQLite database with migrations
├── /lib, /bin, /usr/bin/       # Ubuntu system binaries
└── Environment Variables:
    ├── STATION_CONFIG_ROOT=/app/environment
    └── STATION_DB_PATH=/app/data/station.db
```

## Docker Compose Examples

### Development Environment
```yaml
version: '3.8'
services:
  station-dev:
    image: station-base:latest
    volumes:
      - ./dev/variables.yml:/app/environment/variables.yml
      - ./dev/config.yml:/app/environment/config.yml
      - dev-workspaces:/app/workspaces
    environment:
      - OPENAI_API_KEY=${DEV_OPENAI_KEY}
      - DEBUG=true
    ports:
      - "8585:8585"
volumes:
  dev-workspaces:
```

### Production Environment
```yaml
version: '3.8'
services:
  station-prod:
    image: station-base:latest
    volumes:
      - ./production/variables.yml:/app/environment/variables.yml:ro
      - ./production/config.yml:/app/environment/config.yml:ro
      - prod-workspaces:/app/workspaces
      - prod-logs:/app/logs
    environment:
      - OPENAI_API_KEY=${PROD_OPENAI_KEY}
      - ANTHROPIC_API_KEY=${PROD_ANTHROPIC_KEY}
      - ENVIRONMENT=production
    ports:
      - "8585:8585"
      - "2222:2222"
      - "3000:3000"
    restart: always
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
volumes:
  prod-workspaces:
  prod-logs:
```

### Multi-Environment Stack
```yaml
version: '3.8'
services:
  station-staging:
    image: station-base:latest
    volumes:
      - ./staging/variables.yml:/app/environment/variables.yml:ro
      - ./staging/config.yml:/app/environment/config.yml:ro
    environment:
      - OPENAI_API_KEY=${STAGING_OPENAI_KEY}
      - ENVIRONMENT=staging
    ports:
      - "8586:8585"  # Staging on different port
  
  station-production:
    image: station-base:latest
    volumes:
      - ./production/variables.yml:/app/environment/variables.yml:ro
      - ./production/config.yml:/app/environment/config.yml:ro
    environment:
      - OPENAI_API_KEY=${PROD_OPENAI_KEY}
      - ENVIRONMENT=production
    ports:
      - "8585:8585"  # Production on standard port
```

## Configuration Files

### variables.yml (Environment-Specific)
```yaml
# Template variables for Station dotprompt processing
MEMORY_PATH: /app/data/{{.ENVIRONMENT}}-memory.json
ROOT_PATH: /app/workspaces/{{.ENVIRONMENT}}
API_ENDPOINT: https://{{.ENVIRONMENT}}-api.company.com
DATABASE_URL: postgres://user:pass@{{.ENVIRONMENT}}-db.company.com/station
LOG_LEVEL: "{{ if eq .ENVIRONMENT \"production\" }}info{{ else }}debug{{ end }}"
```

### config.yml (Environment Settings)
```yaml
# Station Configuration for Staging Environment
# This matches the structure from `stn init`

# Core Station settings
admin_username: admin
ai_model: gpt-5
ai_provider: openai
api_port: 8585
database_url: /app/data/station.db
debug: true
encryption_key: "{{ .STAGING_ENCRYPTION_KEY }}"
local_mode: false

# MCP settings
mcp:
  sync:
    confirm: false
    dry_run: false
    environment: "staging"
    interactive: false
    validate: true
    verbose: true

mcp_port: 3000
ssh_host_key_path: /app/ssh_host_key
ssh_port: 2222
telemetry_enabled: true
```

## Environment Variables

### Required
- `OPENAI_API_KEY` - OpenAI API key for Claude/GPT models
- `ANTHROPIC_API_KEY` - Anthropic API key for Claude models

### Optional
- `GITHUB_TOKEN` - For GitHub integration tools
- `ENVIRONMENT` - Environment identifier (dev/staging/production)
- `DEBUG` - Enable debug logging
- `STATION_CONFIG_ROOT` - Override config directory (default: /app/environment)
- `STATION_DB_PATH` - Override database path (default: /app/data/station.db)

## Build Process Details

### Dagger Pipeline
1. **Base Container**: Ubuntu 22.04 + essential packages
2. **Station Binary**: Built from source and installed to `/usr/local/bin/stn`
3. **Database Setup**: SQLite database with full schema migrations
4. **Environment Setup**: Directory structure and environment variables
5. **Entrypoint Script**: Smart config detection and startup logic
6. **Docker Integration**: Export → Load → Tag for immediate availability

### Build Output
```bash
$ stn build base
# Dagger build process...
# Successfully loaded Docker image: station-base:latest
# Image ID: sha256:bb1078264080...
# Run with: docker run -it station-base:latest

$ docker images station-base
REPOSITORY     TAG       IMAGE ID       SIZE
station-base   latest    bb1078264080   125MB
```

## Deployment Patterns

### CI/CD Pipeline
```yaml
# .github/workflows/deploy.yml
- name: Build Station Base Image
  run: stn build base

- name: Deploy to Staging
  run: |
    docker run -d \
      --name station-staging \
      -v ./config/staging/variables.yml:/app/environment/variables.yml \
      -e OPENAI_API_KEY=${{ secrets.STAGING_OPENAI_KEY }} \
      station-base:latest
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station-production
spec:
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
        image: station-base:latest
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        volumeMounts:
        - name: config-volume
          mountPath: /app/environment
        ports:
        - containerPort: 8585
      volumes:
      - name: config-volume
        configMap:
          name: station-config
```

## Technical Implementation

### Dagger Integration
- Uses Dagger for programmatic Docker builds
- Automatic tar export → Docker load → tag process
- Graceful fallback to tar files if Docker daemon unavailable
- Full container structure preservation with proper layering

### Smart Entrypoint
The base image includes an intelligent entrypoint script that:
- Detects mounted configuration files
- Provides helpful feedback about missing configs
- Handles environment variable injection
- Supports both interactive and command execution modes

### Database Management
- SQLite database with full migration support
- Proper schema initialization on container startup
- Database file included in container for immediate functionality
- Supports external database connections via environment variables

## Best Practices

1. **Use Base Images**: Build once with `stn build base`, deploy everywhere
2. **External Configs**: Keep `variables.yml` and `config.yml` outside containers
3. **Secret Management**: Use environment variables for API keys and secrets
4. **Resource Limits**: Set appropriate CPU/memory limits per environment
5. **Health Checks**: Monitor container health with Station's status commands
6. **Persistent Volumes**: Use named volumes for workspaces and logs
7. **Image Tagging**: Tag images with versions for production deployments

## Troubleshooting

### Common Issues

**Build Fails**: Ensure Docker daemon is running and accessible
**Config Not Found**: Verify volume mount paths match container expectations
**Permission Errors**: Check file permissions on mounted config files
**Database Errors**: Ensure SQLite database file permissions are correct

### Debug Commands
```bash
# Check image contents
docker run --rm -it station-base:latest ls -la /app/

# Test config mounting
docker run --rm \
  -v ./test-config.yml:/app/environment/config.yml \
  station-base:latest \
  cat /app/environment/config.yml

# Interactive debugging
docker run --rm -it station-base:latest /bin/bash
```