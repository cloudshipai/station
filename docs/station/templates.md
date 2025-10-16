# Template Variables

Secure, environment-specific configuration using Go template variables. Keep secrets safe while sharing configurations across teams.

## Why Template Variables?

Traditional configuration hardcodes sensitive values:
- ❌ API keys exposed in config files
- ❌ Paths and credentials committed to git
- ❌ Can't share configs safely across environments
- ❌ Configuration drift between dev/staging/prod

Template variables solve this:
- ✅ Values resolved at runtime from `variables.yml`
- ✅ Environment-specific configuration (dev/staging/prod)
- ✅ Share `template.json` without exposing secrets
- ✅ Variables prompted during sync if missing

## How Template Variables Work

### The Flow

```
1. template.json contains:        {{ .PROJECT_ROOT }}
2. variables.yml defines:         PROJECT_ROOT: "/workspace"
3. stn sync resolves to:          /workspace
4. MCP servers start with:        npx ... /workspace
```

**Key Point**: Variables are resolved during `stn sync`, not at agent runtime. MCP servers receive resolved values.

## Template Syntax

Station uses Go template syntax: `{{ .VARIABLE_NAME }}`

### In MCP Server Configuration

**template.json:**
```json
{
  "name": "my-environment",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    },
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "{{ .AWS_REGION }}"],
      "env": {
        "AWS_PROFILE": "{{ .AWS_PROFILE }}"
      }
    }
  }
}
```

**variables.yml:**
```yaml
PROJECT_ROOT: "/workspace"
AWS_REGION: "us-east-1"
AWS_PROFILE: "production"
```

**After sync, MCP servers receive:**
```json
{
  "filesystem": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "/workspace"]
  },
  "aws": {
    "command": "mcp-server-aws",
    "args": ["--region", "us-east-1"],
    "env": {
      "AWS_PROFILE": "production"
    }
  }
}
```

## Setting Variables

### Via Web UI (Recommended)

**Step 1: Navigate to Environment**
- Open Station UI: `http://localhost:8585`
- Go to **Environments** → Select environment

**Step 2: Configure Variables**
- Click **Configure Variables**
- Monaco editor opens with `variables.yml`
- Edit YAML:
  ```yaml
  PROJECT_ROOT: "/workspace"
  AWS_REGION: "us-west-2"
  DATABASE_URL: "postgresql://localhost/mydb"
  ```

**Step 3: Save and Sync**
- Click **Save**
- Click **Sync** to apply changes
- MCP servers restart with new values

**UI Benefits:**
- Syntax highlighting and validation
- Real-time YAML error checking
- Visual confirmation of required variables
- One-click sync after changes

### Interactive Sync Prompts

When you run `stn sync` and variables are missing, Station prompts interactively:

**CLI:**
```bash
stn sync my-environment

# If PROJECT_ROOT is undefined:
# ⚠ Missing variable: PROJECT_ROOT
# Description: Root directory for filesystem access
# Enter value: /home/user/projects
```

**UI:**
- Sync button triggers variable detection
- Modal appears with form for missing variables
- Fill in values, click Apply
- Sync completes automatically

### Via Claude/Cursor (MCP)

```
"Update the variables for environment prod-security:
- PROJECT_ROOT: /workspace/production
- SCAN_TIMEOUT: 600"
```

Claude uses:
- `update_environment_file_config` - Updates variables.yml
- Automatic sync after update

### Manual File Edit (Advanced)

```bash
# Edit variables file directly
vim ~/.config/station/environments/my-env/variables.yml

# Sync to apply
stn sync my-env
```

## Common Use Cases

### Multi-Environment Deployment

**Development Environment:**
```yaml
# ~/.config/station/environments/dev/variables.yml
PROJECT_ROOT: "/home/user/dev-projects"
AWS_REGION: "us-east-1"
AWS_PROFILE: "dev"
LOG_LEVEL: "debug"
TIMEOUT: "30"
```

**Production Environment:**
```yaml
# ~/.config/station/environments/prod/variables.yml
PROJECT_ROOT: "/workspace/production"
AWS_REGION: "us-west-2"
AWS_PROFILE: "production"
LOG_LEVEL: "info"
TIMEOUT: "300"
```

**Same `template.json`, different values per environment.**

### Secrets Management

**Never hardcode secrets:**
```json
{
  "env": {
    "DATABASE_PASSWORD": "{{ .DATABASE_PASSWORD }}",
    "API_KEY": "{{ .API_KEY }}"
  }
}
```

**Load from vault or environment:**
```bash
# Set from vault
export DATABASE_PASSWORD=$(vault kv get -field=password secret/db)
export API_KEY=$(vault kv get -field=key secret/api)

# Or in variables.yml
DATABASE_PASSWORD: "from-vault-123"
API_KEY: "sk-secret-key"
```

**Gitignore `variables.yml`:**
```gitignore
# .gitignore
environments/*/variables.yml
*.env
```

### Path Configuration

**Local Development:**
```yaml
PROJECT_ROOT: "/home/user/my-project"
DATA_DIR: "/home/user/data"
CONFIG_DIR: "/home/user/.config"
```

**Docker/Container:**
```yaml
PROJECT_ROOT: "/workspace"
DATA_DIR: "/data"
CONFIG_DIR: "/config"
```

**Production Server:**
```yaml
PROJECT_ROOT: "/opt/application"
DATA_DIR: "/mnt/data"
CONFIG_DIR: "/etc/myapp"
```

## Variable Resolution

### When Variables Are Resolved

Variables are resolved in this order:

1. **During `stn sync`**: Template variables in `template.json` → values from `variables.yml`
2. **MCP Server Start**: Resolved values passed to MCP server processes
3. **Agent Runtime**: Agents receive tools from MCP servers (no variable access)

**Important**: Agents don't see template variables directly. They only interact with MCP tools that were configured with resolved variables.

### Missing Variables

**If variable is undefined:**
- CLI: Prompts interactively for value
- UI: Shows variable form modal
- Sync fails until variable is provided
- Error message indicates which variable is missing

**Example Error:**
```
Error: Variable 'PROJECT_ROOT' is undefined in template.json
Required by: filesystem MCP server
Enter value or update variables.yml
```

### Default Values

**Template with fallback (not currently supported, use variables.yml defaults):**

```yaml
# variables.yml with defaults
PROJECT_ROOT: "/workspace"
TIMEOUT: "300"
DEBUG: "false"
```

All MCP servers receive these defaults unless overridden.

## Environment-Specific Configuration Patterns

### Pattern 1: Cloud Provider Regions

**template.json:**
```json
{
  "mcpServers": {
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "{{ .AWS_REGION }}"]
    }
  }
}
```

**dev/variables.yml:**
```yaml
AWS_REGION: "us-east-1"  # Development in US East
```

**prod/variables.yml:**
```yaml
AWS_REGION: "eu-west-1"  # Production in Europe
```

### Pattern 2: Database Connections

**template.json:**
```json
{
  "mcpServers": {
    "postgres": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres@latest"],
      "env": {
        "POSTGRES_CONNECTION_STRING": "{{ .DATABASE_URL }}"
      }
    }
  }
}
```

**dev/variables.yml:**
```yaml
DATABASE_URL: "postgresql://localhost:5432/dev_db"
```

**prod/variables.yml:**
```yaml
DATABASE_URL: "postgresql://prod-db.internal:5432/production"
```

### Pattern 3: Tool-Specific Configuration

**template.json:**
```json
{
  "mcpServers": {
    "ship": {
      "command": "ship",
      "args": ["mcp", "security", "--stdio"],
      "env": {
        "SCAN_DEPTH": "{{ .SCAN_DEPTH }}",
        "TIMEOUT": "{{ .SCAN_TIMEOUT }}"
      }
    }
  }
}
```

**dev/variables.yml:**
```yaml
SCAN_DEPTH: "shallow"    # Fast scans for dev
SCAN_TIMEOUT: "60"
```

**prod/variables.yml:**
```yaml
SCAN_DEPTH: "deep"       # Comprehensive scans for prod
SCAN_TIMEOUT: "600"
```

## Troubleshooting

### Variable Not Resolved

**Problem**: MCP server shows `{{ .VARIABLE_NAME }}` in logs

**Solution:**
```bash
# 1. Check variables.yml exists
ls ~/.config/station/environments/my-env/variables.yml

# 2. Verify variable is defined
cat ~/.config/station/environments/my-env/variables.yml | grep VARIABLE_NAME

# 3. Re-sync environment
stn sync my-env

# 4. Check sync output for errors
stn sync my-env --verbose
```

**Via UI:**
1. Environments → Select environment
2. Configure Variables → Check variable exists
3. Save → Sync

### Sync Fails with Missing Variable

**Problem**: `Error: Variable 'PROJECT_ROOT' is undefined`

**Solution:**

**Via UI:**
1. Sync button triggers modal
2. Form shows missing variable
3. Enter value
4. Click Apply → Sync completes

**Via CLI:**
```bash
# Interactive prompt appears:
stn sync my-env
# Enter value when prompted

# Or edit variables.yml first:
vim ~/.config/station/environments/my-env/variables.yml
# Add: PROJECT_ROOT: "/workspace"
stn sync my-env
```

### MCP Server Fails to Start

**Problem**: MCP server crashes with invalid path/value

**Solution:**
```bash
# 1. Check resolved template.json
stn mcp list --env my-env

# 2. Verify variable values are valid
cat ~/.config/station/environments/my-env/variables.yml

# 3. Test MCP server manually
npx -y @modelcontextprotocol/server-filesystem@latest /invalid/path
# Should show error

# 4. Fix variable value
# Via UI: Configure Variables → Correct path → Save → Sync
```

### Variables Not Updating

**Problem**: Changed variables but MCP servers use old values

**Solution:**
```bash
# Variables require sync + MCP restart
stn sync my-env

# If MCP servers still have old values, restart Station:
stn down
stn up
```

**Via UI:**
1. Configure Variables → Save
2. **Must click Sync** to apply
3. MCP servers restart automatically
4. If issues persist: Restart Station server

## Security Best Practices

### 1. Never Commit Secrets

```gitignore
# .gitignore
**/variables.yml
**/variables.yaml
*.env
.env.*
secrets/
```

### 2. Use Secret Management

**Load from Vault:**
```bash
# In CI/CD or deployment scripts
vault kv get -field=db_password secret/prod > /tmp/pass
echo "DATABASE_PASSWORD: $(cat /tmp/pass)" > variables.yml
stn sync production
rm /tmp/pass
```

**From Environment Variables:**
```bash
# In container entrypoint
cat > variables.yml <<EOF
DATABASE_URL: ${DATABASE_URL}
API_KEY: ${API_KEY}
AWS_REGION: ${AWS_REGION}
EOF
stn sync
```

### 3. Least Privilege

Only define variables actually needed:

```yaml
# ❌ Over-sharing
AWS_ACCESS_KEY_ID: "..."
AWS_SECRET_ACCESS_KEY: "..."
FULL_ADMIN_TOKEN: "..."

# ✅ Minimal access
AWS_REGION: "us-east-1"
AWS_PROFILE: "read-only-costs"  # Uses IAM role with limited permissions
```

### 4. Separate Environments

```
environments/
├── dev/
│   └── variables.yml          # Dev credentials, loose security
├── staging/
│   └── variables.yml          # Staging credentials, tighter security
└── prod/
    └── variables.yml          # Production credentials, strictest security
```

Different variable files = different credential scope = environment isolation.

## Advanced Patterns

### Conditional MCP Server Configuration

**Problem**: Want different MCP servers in dev vs prod

**Solution**: Use separate environments with different `template.json`:

**dev environment:**
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    }
  }
}
```

**prod environment:**
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    },
    "aws": {
      "command": "mcp-server-aws",
      "args": ["--region", "{{ .AWS_REGION }}"]
    }
  }
}
```

### Dynamic Tool Access

**template.json:**
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .ALLOWED_PATH_1 }}",
        "{{ .ALLOWED_PATH_2 }}"
      ]
    }
  }
}
```

**dev/variables.yml** (broad access):
```yaml
ALLOWED_PATH_1: "/home/user/projects"
ALLOWED_PATH_2: "/home/user/data"
```

**prod/variables.yml** (restricted access):
```yaml
ALLOWED_PATH_1: "/workspace/app"
ALLOWED_PATH_2: "/workspace/config"
```

## Next Steps

- [Bundles](./bundles.md) - Package environments with template configs
- [MCP Tools](./mcp-tools.md) - Configure MCP servers using variables
- [Agent Development](./agent-development.md) - Create agents using configured tools
- [Deployment Modes](./deployment-modes.md) - Deploy with environment-specific variables
