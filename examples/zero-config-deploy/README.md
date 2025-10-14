# Station Zero-Config Deployment

Deploy Station with pre-configured agent bundles using Docker Compose. This example demonstrates how to use Station's zero-config deployment feature to quickly spin up agent environments without manual configuration.

## Quick Start

1. **Copy the demo bundle**:
```bash
mkdir bundles
cp /tmp/station-bundle-test/demo-finops-investigations.tar.gz bundles/
```

2. **Configure environment variables**:
```bash
cp .env.example .env
# Edit .env with your API keys
```

3. **Start Station**:
```bash
docker-compose up -d
```

4. **Access Station**:
- UI: http://localhost:8585
- MCP: http://localhost:8586

## How It Works

The docker-compose configuration:
1. Uses the official `ghcr.io/cloudshipai/station:latest` image
2. Mounts `./bundles` directory containing `.tar.gz` bundle files
3. Automatically installs and syncs all bundles on startup
4. Passes API keys and credentials via environment variables (never baked into images)
5. Persists Station data in a Docker volume

## Bundle Structure

Place your Station bundles (`.tar.gz` files) in the `bundles/` directory:

```
examples/zero-config-deploy/
├── docker-compose.yml
├── .env
├── .env.example
├── README.md
└── bundles/
    ├── demo-finops-investigations.tar.gz
    ├── security-scanner-bundle.tar.gz
    └── your-custom-bundle.tar.gz
```

Each bundle will be:
- Installed to an environment with the bundle's name
- Synced to discover MCP tools
- Ready to use via the Station UI or CLI

## Environment Variables

### Required
- `OPENAI_API_KEY`: OpenAI API key for agent execution

### Optional
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`: AWS credentials for AWS MCP servers
- `E2B_API_KEY`: E2B API key for code execution environments
- `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`: Alternative AI providers
- `STN_CLOUDSHIP_*`: CloudShip integration settings

## Using Custom Images

You can also use custom-built Station images:

```yaml
services:
  station:
    image: station-aws:latest  # Your custom-built image
    # ... rest of config
```

## Security Notes

- API keys are passed via environment variables at runtime
- No secrets are baked into Docker images
- Bundles contain only agent configurations and MCP templates
- All credentials are resolved at runtime from environment variables

## Logs and Debugging

View Station logs:
```bash
docker-compose logs -f station
```

Execute commands inside the container:
```bash
docker-compose exec station stn agent list
docker-compose exec station stn agent run <agent-id> "your task"
```

## Cleanup

Stop and remove:
```bash
docker-compose down
```

Remove volumes (deletes all Station data):
```bash
docker-compose down -v
```
