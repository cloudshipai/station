# Quickest Way to Deploy Station with Docker Compose

## One-Command Deploy (Recommended)

Use our automated deployment script:

```bash
# Download and run
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/scripts/quick-deploy.sh | bash -s -- --mode docker-compose
```

That's it! The script will:
- ✅ Prompt for your OpenAI API key
- ✅ Create docker-compose.yml
- ✅ Create bundles/ directory
- ✅ Start Station with health checks
- ✅ Open browser to Web UI

---

## Manual Deploy (3 Steps)

If you prefer manual setup:

### 1. Create Directory Structure
```bash
mkdir -p station-deploy/bundles
cd station-deploy
```

### 2. Download Files
```bash
# Download docker-compose.yml
curl -o docker-compose.yml https://raw.githubusercontent.com/cloudshipai/station/main/examples/zero-config-deploy/docker-compose.yml

# Download .env.example
curl -o .env.example https://raw.githubusercontent.com/cloudshipai/station/main/examples/zero-config-deploy/.env.example

# Create .env from example
cp .env.example .env
```

### 3. Configure and Start
```bash
# Edit .env with your API key
nano .env  # or vim, code, etc.

# Start Station
docker-compose up -d

# View logs
docker-compose logs -f
```

**Access Station:**
- Web UI: http://localhost:8585
- MCP Server: http://localhost:8586/mcp

---

## Adding Bundles

### Option 1: Download from Registry
```bash
# Example: Download security scanner bundle
curl -o bundles/security-scanner.tar.gz \
  https://registry.station.dev/bundles/security-scanner.tar.gz

# Restart to auto-install
docker-compose restart
```

### Option 2: Install Manually
```bash
# Install bundle into running container
docker-compose exec station \
  stn bundle install /bundles/your-bundle.tar.gz

# Sync environment
docker-compose exec station \
  stn sync your-bundle-name
```

---

## Common Tasks

### List Agents
```bash
docker-compose exec station stn agent list
```

### Run Agent
```bash
docker-compose exec station \
  stn agent run "Agent Name" "your task description"
```

### View Logs
```bash
# Follow logs
docker-compose logs -f station

# Last 100 lines
docker-compose logs --tail 100 station
```

### Stop Station
```bash
docker-compose down
```

### Restart Station
```bash
docker-compose restart
```

---

## Troubleshooting

### Container Won't Start

**Check API key is set:**
```bash
docker-compose exec station env | grep OPENAI_API_KEY
```

**Check logs for errors:**
```bash
docker-compose logs station | grep -i error
```

### Bundles Not Installing

**Verify bundles mounted:**
```bash
docker-compose exec station ls -la /bundles
```

**Check bundle format:**
```bash
tar -tzf bundles/your-bundle.tar.gz
# Should show: manifest.json, template.json, agents/
```

**Manual installation:**
```bash
docker-compose exec station \
  stn bundle install /bundles/your-bundle.tar.gz bundle-name
```

### Health Check Failing

**Test health endpoint:**
```bash
curl http://localhost:8585/health
```

**Check Station process:**
```bash
docker-compose exec station ps aux | grep stn
```

**Increase startup time:**
Edit `docker-compose.yml` healthcheck:
```yaml
healthcheck:
  start_period: 60s  # Increase from 40s
```

---

## Next Steps

- **[Install Bundles](https://registry.station.dev)** - Browse community bundles
- **[Create Agents](../../docs/station/agent-development.md)** - Build custom agents
- **[Production Deployment](../../docs/deployment/PRODUCTION_DEPLOYMENT.md)** - Production best practices

---

## Need Help?

- **Full Documentation**: https://cloudshipai.github.io/station/
- **GitHub Issues**: https://github.com/cloudshipai/station/issues
- **Discord**: https://discord.gg/station-ai
