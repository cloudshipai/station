# Station Container Lifecycle: `stn up` and `stn down`

This guide explains how `stn up` and `stn down` work, and how to use them for bundle development.

## Overview

`stn up` starts Station as an isolated Docker container, while `stn down` gracefully stops it. Data persists across restarts unless explicitly deleted.

## How `stn up` Works

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              STN UP - STARTING STATION                                   │
└─────────────────────────────────────────────────────────────────────────────────────────┘

  YOU (Developer)
       │
       │  $ stn up --bundle <bundle-id> --workspace ~/code
       ▼
  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │                                  stn up                                              │
  │                             (cmd/main/up.go)                                         │
  └─────────────────────────────────────────────────────────────────────────────────────┘
       │
       ├──────────────────────────────────────────────────────────────────────────────────┐
       │  1. CHECK DOCKER                                                                 │
       │     ├── Is Docker daemon running?                                                │
       │     └── Is station-server container already running?                             │
       │                                                                                  │
       │  2. PREPARE VOLUMES                                                              │
       │     ├── Create station-config volume (first run)                                 │
       │     ├── Create station-cache volume (build cache)                                │
       │     └── Import host ~/.config/station/config.yaml if exists                      │
       │                                                                                  │
       │  3. BUILD/PULL IMAGE                                                             │
       │     ├── Try: docker pull ghcr.io/cloudshipai/station:latest                      │
       │     └── Fallback: docker build -t station-server:latest . (if Dockerfile exists) │
       │                                                                                  │
       │  4. INSTALL BUNDLE (if --bundle flag)                                            │
       │     ├── Download from CloudShip API (if UUID)                                    │
       │     ├── Download from URL (if http://)                                           │
       │     └── Use local file path                                                      │
       │         │                                                                        │
       │         ▼                                                                        │
       │     ┌─────────────────────────────────────────┐                                  │
       │     │  stn bundle install <bundle> default    │                                  │
       │     │                                         │                                  │
       │     │  Extracts to:                           │                                  │
       │     │  ~/.config/station/environments/default/│                                  │
       │     │    ├── agents/*.prompt                  │                                  │
       │     │    ├── mcp-configs/*.json               │                                  │
       │     │    └── variables.yml                    │                                  │
       │     └─────────────────────────────────────────┘                                  │
       │                                                                                  │
       │  5. START CONTAINER                                                              │
       └──────────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │                          Docker Container: station-server                            │
  │                                                                                      │
  │   Volumes Mounted:                                                                   │
  │   ├── station-config:/home/station/.config/station (persistent data)                │
  │   ├── station-cache:/home/station/.cache (build cache)                              │
  │   ├── ~/code:/workspace (your workspace - read/write)                               │
  │   └── /var/run/docker.sock (Docker-in-Docker for Dagger)                            │
  │                                                                                      │
  │   Environment Variables Passed:                                                      │
  │   ├── OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY                             │
  │   ├── AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY                                      │
  │   ├── STN_CLOUDSHIP_KEY, STN_CLOUDSHIP_ENDPOINT                                     │
  │   └── GITHUB_TOKEN, SLACK_BOT_TOKEN, etc.                                           │
  │                                                                                      │
  │   Command: stn serve --database /home/station/.config/station/station.db            │
  │                      --mcp-port 8586                                                │
  └─────────────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │                          stn serve (cmd/main/server.go)                              │
  │                                                                                      │
  │   STARTUP SEQUENCE:                                                                  │
  │   ─────────────────                                                                  │
  │   1. Load config.yaml                                                                │
  │   2. Initialize SQLite database                                                      │
  │   3. Run database migrations                                                         │
  │   4. Create default environment if none exists                                       │
  │   5. DeclarativeSync: Sync files → database                                         │
  │      │                                                                               │
  │      ├── Scan environments/default/mcp-configs/*.json                               │
  │      │   └── Connect to each MCP server, discover tools                             │
  │      │                                                                               │
  │      └── Scan environments/default/agents/*.prompt                                  │
  │          └── Parse prompts, create agent records                                     │
  │                                                                                      │
  │   6. Initialize Genkit (AI provider: OpenAI/Gemini)                                 │
  │   7. Initialize Lighthouse client (CloudShip connection)                            │
  │   8. Start scheduler service (cron jobs)                                            │
  │   9. Start all servers ──────────────────────────────────────────┐                  │
  └──────────────────────────────────────────────────────────────────│───────────────────┘
                                                                     │
                                                                     ▼
  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │                              RUNNING SERVICES                                        │
  │                                                                                      │
  │   ┌─────────────────────────┐  ┌──────────────────────────────┐  ┌────────────────┐ │
  │   │    MCP Server           │  │    Dynamic Agent MCP         │  │  API/UI Server │ │
  │   │    Port 8586            │  │    Port 8587                 │  │  Port 8585     │ │
  │   │                         │  │                              │  │                │ │
  │   │  • list_tools           │  │  • run_agent                 │  │  • Settings UI │ │
  │   │  • call_tool            │  │  • list_agents               │  │  • Agent list  │ │
  │   │  • ingest_data          │  │  • get_agent                 │  │  • Logs        │ │
  │   │  • create_agent         │  │                              │  │  • Runs        │ │
  │   │  • delete_agent         │  │  Executes agents with        │  │                │ │
  │   │  • list_agents          │  │  tools from MCP servers      │  │  (Dev mode)    │ │
  │   │  • ... 20+ tools        │  │                              │  │                │ │
  │   └─────────────────────────┘  └──────────────────────────────┘  └────────────────┘ │
  │                                                                                      │
  │   URL: http://localhost:8586/mcp   http://localhost:8587/mcp  http://localhost:8585 │
  └─────────────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │  .mcp.json updated automatically:
                                    │  {
                                    │    "mcpServers": {
                                    │      "station": {
                                    │        "type": "http",
                                    │        "url": "http://localhost:8586/mcp"
                                    │      }
                                    │    }
                                    │  }
                                    ▼
  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │   Claude Desktop / Cursor / Any MCP Client                                          │
  │                                                                                      │
  │   Now has access to:                                                                 │
  │   • All tools from connected MCP servers (e.g., GitHub, Slack, AWS...)              │
  │   • All agents defined in your bundle                                               │
  │   • run_agent tool to execute agents                                                │
  └─────────────────────────────────────────────────────────────────────────────────────┘
```

## How `stn down` Works

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              STN DOWN - STOPPING STATION                                 │
└─────────────────────────────────────────────────────────────────────────────────────────┘

  YOU (Developer)
       │
       │  $ stn down [--remove-volume] [--clean-mcp]
       ▼
  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │                                  stn down                                            │
  │                             (cmd/main/down.go)                                       │
  │                                                                                      │
  │   1. docker stop station-server     (graceful SIGTERM, 3s timeout)                  │
  │   2. docker rm station-server       (remove container)                               │
  │                                                                                      │
  │   Optional flags:                                                                    │
  │   ├── --remove-volume: docker volume rm station-config                               │
  │   │                    ⚠️  DELETES ALL: agents, configs, database, bundles           │
  │   │                                                                                  │
  │   ├── --clean-mcp: Remove "station" from .mcp.json                                   │
  │   │                                                                                  │
  │   ├── --remove-image: docker rmi station-server:latest                               │
  │   │                                                                                  │
  │   └── --force: SIGKILL if graceful stop fails                                        │
  │                                                                                      │
  │   ✅ DATA PRESERVED (unless --remove-volume):                                        │
  │   • station-config volume: config.yaml, environments, agents, database              │
  │   • station-cache volume: build caches                                               │
  │   • Your workspace files: unchanged                                                  │
  │                                                                                      │
  │   💡 Run 'stn up' again to restart with same data                                    │
  └─────────────────────────────────────────────────────────────────────────────────────┘
```

## Bundle Development Workflow

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                           DEVELOPING A BUNDLE WITH STATION                               │
└─────────────────────────────────────────────────────────────────────────────────────────┘

  STEP 1: Create your bundle files locally
  ═════════════════════════════════════════

  ~/.config/station/environments/my-bundle/
  ├── agents/
  │   ├── code-reviewer.prompt     # Agent definition with tools
  │   ├── deploy-helper.prompt     # Another agent
  │   └── ...
  │
  ├── mcp-configs/
  │   ├── github.json              # GitHub MCP server config
  │   ├── slack.json               # Slack MCP server config
  │   └── custom-tool.json         # Your custom MCP server
  │
  └── variables.yml                 # Environment variables template
      ┌────────────────────────────────────────┐
      │ variables:                             │
      │   - name: GITHUB_TOKEN                 │
      │     description: "GitHub access token" │
      │     required: true                     │
      │   - name: SLACK_BOT_TOKEN              │
      │     description: "Slack bot token"     │
      │     required: true                     │
      └────────────────────────────────────────┘


  STEP 2: Test locally with stn serve
  ════════════════════════════════════

  $ stn serve --environment my-bundle

  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │  stn serve reads your files directly:                                               │
  │                                                                                      │
  │  1. DeclarativeSync scans environments/my-bundle/                                   │
  │  2. Connects to MCP servers defined in mcp-configs/*.json                           │
  │  3. Loads agents from agents/*.prompt                                               │
  │  4. Exposes everything via MCP on ports 8586/8587                                   │
  │                                                                                      │
  │  Make changes → Restart stn serve → Changes take effect                             │
  └─────────────────────────────────────────────────────────────────────────────────────┘


  STEP 3: Package as a bundle
  ════════════════════════════

  $ stn bundle create my-bundle -o my-bundle.tar.gz

  Creates a tarball containing:
  ┌────────────────────────────────────┐
  │ my-bundle.tar.gz                   │
  │  ├── agents/*.prompt               │
  │  ├── mcp-configs/*.json            │
  │  ├── variables.yml                 │
  │  └── manifest.json (metadata)      │
  └────────────────────────────────────┘


  STEP 4: Test the bundle with stn up
  ════════════════════════════════════

  # Start fresh (removes previous data)
  $ stn down --remove-volume

  # Install and run your bundle in a container
  $ stn up --bundle ./my-bundle.tar.gz

  ┌─────────────────────────────────────────────────────────────────────────────────────┐
  │  This simulates exactly how CloudShip users will run your bundle:                   │
  │                                                                                      │
  │  1. Creates isolated Docker container                                               │
  │  2. Installs bundle into container's default environment                            │
  │  3. Runs DeclarativeSync to load everything                                         │
  │  4. Starts MCP servers and agents                                                   │
  │                                                                                      │
  │  Access your agents via Claude Desktop / Cursor:                                    │
  │  - run_agent("code-reviewer", "Review my PR")                                       │
  │  - run_agent("deploy-helper", "Deploy to staging")                                  │
  └─────────────────────────────────────────────────────────────────────────────────────┘


  STEP 5: Publish to CloudShip
  ════════════════════════════

  $ stn bundle push my-bundle.tar.gz

  Users can then:
  $ stn up --bundle <bundle-id>
```

## Command Reference

### `stn up`

Start Station in a Docker container.

```bash
# Basic usage - start with current directory as workspace
stn up

# Specify workspace directory
stn up --workspace ~/code

# Start with a CloudShip bundle
stn up --bundle e26b414a-f076-4135-927f-810bc1dc892a

# Start with a local bundle file
stn up --bundle ./my-bundle.tar.gz

# Start with AI provider configuration
stn up --provider openai --api-key sk-xxx...

# Enable Genkit Developer UI (port 4000)
stn up --develop

# Rebuild container image before starting
stn up --upgrade

# Pass additional environment variables
stn up --env CUSTOM_VAR=value
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--workspace, -w` | Workspace directory to mount (default: current directory) |
| `--bundle` | CloudShip bundle ID, URL, or local file path to install |
| `--provider` | AI provider: openai, gemini, anthropic, custom |
| `--model` | AI model to use (e.g., gpt-4o-mini, gemini-2.0-flash-exp) |
| `--api-key` | API key for AI provider |
| `--base-url` | Custom base URL for OpenAI-compatible endpoints |
| `--develop` | Enable Genkit Developer UI mode (port 4000) |
| `--environment` | Station environment to use in develop mode |
| `--upgrade` | Rebuild container image before starting |
| `--env` | Additional environment variables to pass through |
| `--detach, -d` | Run container in background (default: true) |
| `--yes, -y` | Use defaults without interactive prompts |
| `--ship` | Bootstrap with ship CLI MCP integration |

### `stn down`

Stop the Station container.

```bash
# Stop server (data preserved in Docker volume)
stn down

# Stop and delete ALL data (config, agents, bundles, database)
stn down --remove-volume

# Stop and remove Station from .mcp.json
stn down --clean-mcp

# Stop and remove Docker image
stn down --remove-image

# Force kill if graceful stop fails
stn down --force
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--remove-volume` | Delete ALL Station data (environments, agents, bundles, config) |
| `--clean-mcp` | Remove Station from .mcp.json |
| `--remove-image` | Remove Docker image after stopping |
| `--force` | Force stop (kill) if graceful stop fails |

### Related Commands

| Command | Description |
|---------|-------------|
| `stn logs` | Show container logs (`-f` to follow) |
| `stn status` | Show container status and port mappings |
| `stn restart` | Restart the container (down + up) |
| `stn serve` | Run Station directly without Docker |
| `stn sync <env>` | Reload agents/MCP configs from files |
| `stn bundle create` | Package environment as a bundle |
| `stn bundle install` | Extract bundle to an environment |

## Exposed Ports

| Port | Service | Description |
|------|---------|-------------|
| 8585 | API/UI Server | Web interface for settings, agent management (dev mode only) |
| 8586 | MCP Server | Main MCP endpoint - tools, agents, data ingestion |
| 8587 | Dynamic Agent MCP | Agent execution - `run_agent`, `list_agents` |
| 4000 | Genkit Developer UI | Only when `--develop` flag is used |

## Data Persistence

Station stores all persistent data in Docker volumes:

| Volume | Contents | Preserved on `stn down`? |
|--------|----------|--------------------------|
| `station-config` | config.yaml, database, environments, agents, bundles | Yes (unless `--remove-volume`) |
| `station-cache` | Build caches, temporary files | Yes |

Your workspace directory is mounted read-write but remains on your host filesystem.

## Typical Workflows

### Fresh Start with New Bundle

```bash
# Remove all previous data
stn down --remove-volume

# Start with new bundle
stn up --bundle <new-bundle-id>
```

### Update Running Station

```bash
# Restart to pick up config changes
stn restart

# Or rebuild with latest image
stn down
stn up --upgrade
```

### Development Iteration

```bash
# Test bundle locally first (no Docker)
stn serve --environment my-bundle

# When ready, test in container
stn bundle create my-bundle -o my-bundle.tar.gz
stn down --remove-volume
stn up --bundle ./my-bundle.tar.gz
```

### Debug Container Issues

```bash
# Check status
stn status

# Follow logs
stn logs -f

# Show last 500 lines
stn logs --tail 500
```

## See Also

- [Bundle System](./bundles.md) - Understanding Station bundles
- [Deployment Modes](./deployment-modes.md) - Different ways to run Station
- [Architecture](./architecture.md) - How Station works internally
