# Station Docker Deployment Guide

This guide covers deploying Station using Docker containers for development and production use.

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/your-org/station.git
cd station

# Start Station with Docker Compose
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f station
```

### Using Docker Run

```bash
# Pull latest image
docker pull ghcr.io/your-org/station:latest

# Run Station
docker run -d \
  --name station \
  -p 8080:8080 \
  -p 2222:2222 \
  -p 3000:3000 \
  -v station_data:/home/station/data \
  -v station_config:/home/station/.config/station \
  ghcr.io/your-org/station:latest
```

## Configuration

### Environment Variables

Configure Station using environment variables:

```bash
docker run -d \
  --name station \
  -p 8080:8080 \
  -p 2222:2222 \
  -p 3000:3000 \
  -e STATION_LOCAL_MODE=false \
  -e STATION_LOG_LEVEL=info \
  -e STATION_DATABASE_URL=/home/station/data/station.db \
  -e GENKIT_API_KEY=your-api-key \
  -v station_data:/home/station/data \
  ghcr.io/your-org/station:latest
```

### Using Config File

Mount a configuration file:

```bash
# Create config directory
mkdir -p ./config

# Create config file
cat > ./config/config.yaml << 'EOF'
database_url: "/home/station/data/station.db"
ssh_port: 2222
mcp_port: 3000
api_port: 8080
local_mode: false
log_level: "info"
encryption_key: "your-64-character-hex-key"
EOF

# Run with config file
docker run -d \
  --name station \
  -p 8080:8080 \
  -p 2222:2222 \
  -p 3000:3000 \
  -v ./config:/home/station/.config/station:ro \
  -v station_data:/home/station/data \
  ghcr.io/your-org/station:latest
```

## Production Deployment

### Docker Compose for Production

Create `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  station:
    image: ghcr.io/your-org/station:latest
    restart: unless-stopped
    ports:
      - "127.0.0.1:8080:8080"  # Only bind to localhost
      - "0.0.0.0:2222:2222"    # SSH admin access
      - "127.0.0.1:3000:3000"  # MCP server (behind proxy)
    volumes:
      - /opt/station/data:/home/station/data
      - /opt/station/config:/home/station/.config/station:ro
    environment:
      STATION_LOCAL_MODE: "false"
      STATION_LOG_LEVEL: "info"
    healthcheck:
      test: ["CMD", "stn", "--version"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  # Reverse proxy (optional)
  nginx:
    image: nginx:alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - station
    healthcheck:
      test: ["CMD", "nginx", "-s", "reload"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  station_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /opt/station/data
  station_config:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /opt/station/config

networks:
  default:
    name: station-prod
```

### Setup Production Environment

```bash
# Create directories
sudo mkdir -p /opt/station/{data,config,logs}
sudo chown 1001:1001 /opt/station/data  # Station user UID
sudo chmod 755 /opt/station/{data,config}

# Create configuration
sudo tee /opt/station/config/config.yaml << 'EOF'
database_url: "/home/station/data/station.db"
ssh_port: 2222
mcp_port: 3000
api_port: 8080
local_mode: false
log_level: "info"
encryption_key: "generate-with-openssl-rand-hex-32"
EOF

# Set secure permissions
sudo chmod 600 /opt/station/config/config.yaml

# Start services
docker-compose -f docker-compose.prod.yml up -d
```

## Development

### Building Custom Image

```bash
# Build development image
docker build -t station:dev .

# Build with custom Go version
docker build --build-arg GO_VERSION=1.22 -t station:dev .

# Build for different platform
docker buildx build --platform linux/amd64,linux/arm64 -t station:dev .
```

### Development with Hot Reload

Create `docker-compose.dev.yml`:

```yaml
version: '3.8'

services:
  station-dev:
    build: .
    ports:
      - "8080:8080"
      - "2222:2222"
      - "3000:3000"
    volumes:
      - .:/app:ro
      - station_dev_data:/home/station/data
    environment:
      STATION_LOCAL_MODE: "true"
      STATION_LOG_LEVEL: "debug"
    command: ["stn", "serve", "--host", "0.0.0.0", "--debug"]

volumes:
  station_dev_data:
```

## Networking

### Port Configuration

Station uses these ports:

- **8080**: HTTP API (REST endpoints)
- **2222**: SSH Admin Interface
- **3000**: MCP Server (Model Context Protocol)

### Reverse Proxy Setup

Example nginx configuration:

```nginx
upstream station_api {
    server 127.0.0.1:8080;
}

upstream station_mcp {
    server 127.0.0.1:3000;
}

server {
    listen 80;
    server_name station.example.com;

    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name station.example.com;

    ssl_certificate /etc/nginx/ssl/station.crt;
    ssl_certificate_key /etc/nginx/ssl/station.key;

    # API endpoints
    location /api/ {
        proxy_pass http://station_api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # MCP endpoint
    location /mcp {
        proxy_pass http://station_mcp;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support (if needed)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Health check
    location /health {
        proxy_pass http://station_api/health;
    }
}
```

## Data Management

### Backup

```bash
# Backup data volume
docker run --rm -v station_data:/data -v $(pwd):/backup alpine tar czf /backup/station-data-$(date +%Y%m%d-%H%M%S).tar.gz -C /data .

# Backup config
docker run --rm -v station_config:/config -v $(pwd):/backup alpine tar czf /backup/station-config-$(date +%Y%m%d-%H%M%S).tar.gz -C /config .
```

### Restore

```bash
# Restore data
docker run --rm -v station_data:/data -v $(pwd):/backup alpine tar xzf /backup/station-data-backup.tar.gz -C /data

# Restore config
docker run --rm -v station_config:/config -v $(pwd):/backup alpine tar xzf /backup/station-config-backup.tar.gz -C /config
```

### Migrate Data

```bash
# Export from old container
docker run --rm -v old_station_data:/source -v $(pwd):/backup alpine tar czf /backup/migration.tar.gz -C /source .

# Import to new container
docker run --rm -v new_station_data:/dest -v $(pwd):/backup alpine tar xzf /backup/migration.tar.gz -C /dest
```

## Monitoring

### Health Checks

Station provides health check endpoints:

```bash
# Check if container is healthy
docker ps --filter "name=station" --format "table {{.Names}}\\t{{.Status}}"

# Check health endpoint
curl http://localhost:8080/health

# Check specific service health
curl http://localhost:8080/api/v1/health/database
curl http://localhost:8080/api/v1/health/mcp
```

### Logs

```bash
# View logs
docker logs station

# Follow logs
docker logs -f station

# View logs with timestamps
docker logs -t station

# View last 100 lines
docker logs --tail 100 station

# View logs from specific time
docker logs --since "2024-01-01T00:00:00" station
```

### Resource Monitoring

```bash
# Monitor resource usage
docker stats station

# Get detailed container info
docker inspect station

# Check disk usage
docker system df
docker system df -v
```

## Troubleshooting

### Container Won't Start

1. **Check logs:**
   ```bash
   docker logs station
   ```

2. **Check configuration:**
   ```bash
   docker run --rm -v station_config:/config alpine cat /config/config.yaml
   ```

3. **Check permissions:**
   ```bash
   docker run --rm -v station_data:/data alpine ls -la /data
   ```

### Database Issues

```bash
# Access database directly
docker exec -it station sqlite3 /home/station/data/station.db

# Check database file
docker run --rm -v station_data:/data alpine ls -la /data/station.db

# Fix permissions
docker run --rm -v station_data:/data alpine chown 1001:1001 /data/station.db
```

### Network Issues

```bash
# Check port bindings
docker port station

# Test connectivity
docker exec station ping 8.8.8.8

# Check network
docker network inspect bridge
```

### Performance Issues

```bash
# Check resource usage
docker stats --no-stream station

# Check system resources
docker system df
docker system events --filter container=station
```

## Security

### Container Security

```bash
# Run with security options
docker run -d \
  --name station \
  --security-opt no-new-privileges:true \
  --cap-drop ALL \
  --cap-add NET_BIND_SERVICE \
  --read-only \
  --tmpfs /tmp \
  -v station_data:/home/station/data \
  ghcr.io/your-org/station:latest
```

### Secrets Management

Use Docker secrets or external secret management:

```bash
# Using Docker secrets
echo "your-secret-key" | docker secret create station_encryption_key -

# Reference in compose file
services:
  station:
    secrets:
      - station_encryption_key
    environment:
      STATION_ENCRYPTION_KEY_FILE: /run/secrets/station_encryption_key

secrets:
  station_encryption_key:
    external: true
```

### Network Security

```bash
# Create custom network
docker network create --driver bridge station-network

# Run with custom network
docker run -d \
  --name station \
  --network station-network \
  ghcr.io/your-org/station:latest
```

## Multi-Architecture Support

Station supports multiple architectures:

```bash
# Pull for specific architecture
docker pull --platform linux/amd64 ghcr.io/your-org/station:latest
docker pull --platform linux/arm64 ghcr.io/your-org/station:latest

# Build multi-arch images
docker buildx create --name station-builder --use
docker buildx build --platform linux/amd64,linux/arm64 -t station:multi-arch --push .
```