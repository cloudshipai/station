# Bundles

Package and distribute complete agent environments, making it easy to share agents across teams and deploy to production.

## What are Bundles?

Bundles are portable packages containing:
- **Agents** - The AI agents with their prompts and tool assignments
- **MCP Server Configurations** - Tool integrations (filesystem, AWS, security tools)
- **Template Variables** - Placeholders for environment-specific values
- **Metadata** - Agent descriptions, tags, required tools

Think of bundles as "Docker images for agents" - build once, deploy anywhere with environment-specific configuration.

## Why Bundles?

| Traditional Setup | With Bundles |
|-------------------|--------------|
| Manual agent recreation on each system | Install bundle, customize variables |
| Sharing agents via screenshots/docs | Share single bundle file or URL |
| Configuration drift across environments | Same bundle, different variables per env |
| No version control for agents | Versioned bundles with manifests |

**Use Cases:**
- **Team Distribution**: Share security scanners, cost analyzers across teams
- **Multi-Environment Deployment**: Dev/staging/prod from same bundle
- **Registry Publishing**: Community bundles via official registry
- **Backup & Recovery**: Bundle production agents for disaster recovery

## Creating Bundles

### Via Claude/Cursor (MCP - Recommended)

Use Station's MCP tools to create bundles directly from Claude Desktop or Cursor:

```
"Create a bundle from my security-scanner environment"
```

Claude will use:
- `create_bundle_from_environment` - Creates .tar.gz bundle with manifest
- `export_agents` - Exports agents to dotprompt format
- Bundle automatically includes agents, MCP configs, and metadata

**What gets bundled:**
- All `.prompt` files from `agents/` directory
- `template.json` (MCP server configurations)
- Generated `manifest.json` (bundle metadata, tool lists, variables)
- **Note**: `variables.yml` is **not** included (environment-specific secrets)

### Via CLI (Quick & Dirty)

```bash
# Create bundle from environment
stn bundle create security-scanner

# Output: security-scanner.tar.gz
# Bundle saved to current directory
```

**Verify bundle contents:**
```bash
tar -tzf security-scanner.tar.gz
# Shows:
# manifest.json          (generated metadata)
# template.json          (MCP server config)
# agents/                (agent definitions)
#   terraform-scanner.prompt
#   container-scanner.prompt
```

### Via Web UI

**Step 1: Navigate to Bundles**
- Open Station UI: `http://localhost:8585`
- Go to **Bundles** → **Create Bundle**

**Step 2: Select Environment**
- Choose environment to bundle
- Preview: agents, MCP servers, required tools

**Step 3: Generate Bundle**
- Click **Create Bundle**
- Download `.tar.gz` file
- Manifest generated automatically

**UI Benefits:**
- Visual preview of what's included
- See required variables and dependencies
- One-click bundle creation
- Automatic manifest generation

## Installing Bundles

### Via CLI

```bash
# Install from local file
stn bundle install security-scanner.tar.gz

# Install from URL
stn bundle install https://registry.station.dev/bundles/security-scanner.tar.gz

# Install to custom environment name
stn bundle install security-scanner.tar.gz --environment my-security-tools
```

**What happens during install:**
1. Creates new environment (both database + filesystem)
2. Extracts agents to `environments/<name>/agents/`
3. Extracts `template.json` for MCP server config
4. Creates stub `variables.yml` with required variables
5. Prompts to set required variables (if detected)

### Via Web UI

**Step 1: Upload or Fetch Bundle**
- Go to **Bundles** → **Install Bundle**
- Upload `.tar.gz` file OR provide URL
- Bundle validated and manifest displayed

**Step 2: Configure Variables**
- UI shows required variables from manifest
- Enter environment-specific values:
  - `PROJECT_ROOT: /workspace`
  - `AWS_REGION: us-east-1`
- Variables stored in new environment's `variables.yml`

**Step 3: Install**
- Click **Install Bundle**
- New environment created automatically
- Agents ready to use

**Step 4: Sync (if needed)**
- Go to **Environments** → Select new environment
- Click **Sync** to load agents into database
- MCP servers connect automatically

### Via Claude/Cursor (MCP)

```
"Install the security-scanner bundle from URL https://registry.station.dev/bundles/security-scanner.tar.gz to environment prod-security"
```

Claude uses `install_demo_bundle` or API endpoints to:
- Download/fetch bundle
- Create environment
- Extract files
- Prompt for variables

##Bundle Structure

A typical bundle contains:

```
security-scanner.tar.gz
├── manifest.json          # Generated metadata
├── template.json          # MCP server config
└── agents/               # Agent definitions
    ├── terraform-security-scanner.prompt
    ├── container-security-scanner.prompt
    └── secret-leak-detector.prompt
```

### manifest.json (Auto-Generated)

Generated automatically during bundle creation:

```json
{
  "version": "1.0",
  "bundle": {
    "name": "security-scanner",
    "description": "Station bundle for security-scanner environment",
    "tags": ["security", "terraform", "docker"],
    "created_at": "2025-10-16T10:00:00Z",
    "station_version": "0.2.8"
  },
  "agents": [
    {
      "name": "Terraform Security Scanner",
      "description": "Scans Terraform for misconfigurations",
      "model": "gpt-4o-mini",
      "max_steps": 10,
      "tags": ["terraform", "iac"],
      "tools": ["__read_text_file", "__checkov_scan_directory"]
    }
  ],
  "mcp_servers": [
    {
      "name": "filesystem",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"],
      "tools": ["__read_text_file", "__list_directory"]
    }
  ],
  "required_variables": [
    {
      "name": "PROJECT_ROOT",
      "description": "Required variable: PROJECT_ROOT",
      "type": "string",
      "required": true
    }
  ]
}
```

### template.json (MCP Configuration)

Defines MCP servers and their connections:

```json
{
  "name": "security-scanner",
  "description": "Security scanning agents",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    },
    "ship": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"]
    }
  }
}
```

## Environment-Specific Configuration

Same bundle, different variables per environment.

### Development Environment

```bash
# Install bundle
stn bundle install security-scanner.tar.gz --environment security-dev

# Set dev-specific variables (via UI or edit file)
# ~/.config/station/environments/security-dev/variables.yml
PROJECT_ROOT: "/home/user/dev-projects"
SCAN_TIMEOUT: "60"
DEBUG_MODE: "true"
```

### Production Environment

```bash
# Install same bundle to production
stn bundle install security-scanner.tar.gz --environment security-prod

# Set prod-specific variables
# ~/.config/station/environments/security-prod/variables.yml
PROJECT_ROOT: "/workspace/production-code"
SCAN_TIMEOUT: "600"
DEBUG_MODE: "false"
COMPLIANCE_MODE: "strict"
```

### Updating Variables (UI)

1. Go to **Environments** → Select environment
2. Click **Configure Variables**
3. Edit `variables.yml` in Monaco editor
4. Click **Save** → **Sync** to apply

## Sharing Bundles

### Share via File

```bash
# Create bundle
stn bundle create finops-agents

# Share file directly
scp finops-agents.tar.gz teammate@remote:/tmp/

# Teammate installs
ssh teammate@remote "stn bundle install /tmp/finops-agents.tar.gz"
```

### Share via URL

```bash
# Upload to web server
aws s3 cp security-scanner.tar.gz \
  s3://my-bundles/security-scanner.tar.gz \
  --acl public-read

# Share URL
# Anyone can install: stn bundle install https://my-bundles.s3.amazonaws.com/security-scanner.tar.gz
```

### Share via CloudShip (Enterprise)

```bash
# Upload bundle to CloudShip registry
stn bundle share security-scanner.tar.gz

# CloudShip provides:
# - Private registry
# - Version management
# - Team access control
# - Usage analytics
```

## Registry Integration

Station bundles can be published to the official registry for community discovery.

### Installing from Registry

```bash
# List available bundles
stn registry list

# Search for bundles
stn registry search security

# Install bundle by ID
stn registry install security-scanner

# Install specific version
stn registry install security-scanner@1.2.0
```

### Publishing to Registry (Coming Soon)

Official Station registry will support community bundle submissions. Bundle requirements:
- Complete manifest with descriptions
- Tested agents with example tasks
- Documentation and usage examples
- Open source or community license

## Troubleshooting

### Bundle Installation Fails

**Problem**: `Error: Failed to install bundle`

**Solution:**
```bash
# Verify bundle file exists and is valid tar.gz
tar -tzf security-scanner.tar.gz

# Check for template.json and agents/
tar -tzf security-scanner.tar.gz | grep -E "template\.json|agents/"

# Try with verbose output
stn bundle install security-scanner.tar.gz --verbose
```

### Variables Not Resolved

**Problem**: MCP servers show `{{ .VARIABLE_NAME }}` errors

**Solution:**
```bash
# Check variables file exists
cat ~/.config/station/environments/my-env/variables.yml

# Verify all required variables are set
stn sync my-env

# If using UI: Environments → Configure Variables → Save → Sync
```

### Agent Not Found After Install

**Problem**: `stn agent list` shows no agents

**Solution:**
```bash
# Verify agents extracted
ls ~/.config/station/environments/my-env/agents/

# Sync environment to load agents
stn sync my-env

# Check sync logs for errors
stn sync my-env --verbose
```

### Environment Already Exists

**Problem**: `Environment 'my-env' already exists`

**Solution:**
```bash
# Install to different environment name
stn bundle install bundle.tar.gz --environment my-env-v2

# Or delete existing environment first (WARNING: deletes all agents)
# Via MCP: "Delete environment my-env"
# Via CLI: Use MCP tools (no direct CLI delete for safety)
```

## Best Practices

### Bundle Naming

- Use hyphenated lowercase: `security-scanner`, `finops-cost-analyzer`
- Include purpose: `cicd-terraform-validator`, `prod-monitoring-agents`
- Version in filename if distributing: `security-scanner-v1.2.0.tar.gz`

### What to Bundle

✅ **Include:**
- Agent `.prompt` files
- `template.json` (MCP server config)
- Documentation (README.md)

❌ **Don't Include:**
- `variables.yml` (contains secrets/paths)
- `.env` files (environment-specific)
- API keys or credentials
- Large data files

### Testing Before Distribution

1. Create bundle from working environment
2. Install to new test environment
3. Set required variables
4. Test each agent with sample tasks
5. Verify all MCP tools accessible
6. Document any prerequisites

## Next Steps

- [Agent Development](./agent-development.md) - Create agents for your bundle
- [Template Variables](./templates.md) - Use variables for environment-specific config
- [MCP Tools](./mcp-tools.md) - Configure MCP servers and tools
- [Deployment Modes](./deployment-modes.md) - Deploy bundles to production
