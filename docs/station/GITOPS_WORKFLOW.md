# GitOps Workflow for Station Agents

## Overview

Station supports version-controlling your agent configurations, MCP server templates, and environment variables in Git. This enables:

- **Team Collaboration** - Share agent configurations across your team
- **Version Control** - Track changes to agent prompts and MCP configs
- **Code Review** - Review agent changes in Pull Requests
- **Environment Management** - Separate production, staging, and development configs
- **Disaster Recovery** - Git as source of truth for all configurations

## Quick Start

### 1. Create Config Repository

```bash
# Create a new directory for your Station configuration
mkdir my-station-config
cd my-station-config

# Initialize Station in this directory
export STATION_WORKSPACE=$(pwd)
stn init

# Initialize Git
git init

# Create .gitignore
cat > .gitignore << 'EOF'
# Database files (use cloud DB or Litestream for persistence)
station.db
station.db-wal
station.db-shm

# Logs
*.log

# OS files
.DS_Store
*~
EOF

# Commit initial structure
git add .
git commit -m "Initial Station configuration"
```

### 2. Directory Structure

After initialization, your repository will have:

```
my-station-config/
├── .gitignore                 # Ignore database and logs
├── README.md                  # Documentation for your team
├── config.yaml                # Station configuration (optional)
├── environments/
│   ├── production/
│   │   ├── agents/
│   │   │   ├── security-scanner.prompt
│   │   │   └── cost-analyzer.prompt
│   │   ├── template.json     # MCP server configurations
│   │   └── variables.yml     # Environment-specific variables
│   ├── staging/
│   │   ├── agents/
│   │   └── template.json
│   └── development/
│       ├── agents/
│       └── template.json
└── vars/                      # Shared variables (optional)
```

### 3. Team Workflow

#### Developer Setup

```bash
# Clone the team repository
git clone git@github.com:your-team/station-config.git
cd station-config

# Run Station with this workspace
export STATION_WORKSPACE=$(pwd)
stn stdio
```

#### Create New Agent

```bash
# Create agent (saves to ./environments/default/agents/)
stn agent create "My New Agent"

# Agent file created at:
# ./environments/default/agents/my-new-agent.prompt

# Commit and push
git add environments/default/agents/my-new-agent.prompt
git commit -m "Add new security scanning agent"
git push
```

#### Update Existing Agent

```bash
# Edit agent prompt file directly
vim environments/production/agents/security-scanner.prompt

# Or use Station CLI to update via API
stn agent update security-scanner --env production

# Sync changes
stn sync production

# Commit changes
git add environments/production/agents/security-scanner.prompt
git commit -m "Update security scanner with new rules"
git push
```

#### Pull Team Updates

```bash
# Pull latest changes
git pull

# Sync environments to load new agents
stn sync production
stn sync staging
```

---

## Configuration Options

### Option 1: Environment Variable (Recommended)

```bash
export STATION_WORKSPACE=/path/to/station-config
stn stdio
```

Add to your shell profile (`.bashrc`, `.zshrc`, etc.):
```bash
# ~/.zshrc
export STATION_WORKSPACE=~/code/my-team/station-config
```

### Option 2: Config File

Create `config.yaml` in your workspace:

```yaml
# ~/code/station-config/config.yaml
workspace: /home/user/code/station-config
database_url: /home/user/code/station-config/station.db

# Optional: Use cloud database for team collaboration
# database_url: "libsql://team-db.example.com?authToken=${STATION_DB_TOKEN}"

# AI Provider
ai_provider: openai
ai_model: gpt-4o-mini
```

Then run Station with config path:
```bash
stn stdio --config ~/code/station-config/config.yaml
```

### Option 3: STATION_CONFIG_DIR

Set the config directory directly:
```bash
export STATION_CONFIG_DIR=/path/to/station-config
stn stdio
```

---

## Example Workflows

### Workflow 1: Pull Request Review

**Developer creates new agent:**
```bash
cd ~/code/station-config
git checkout -b feature/add-cost-optimizer

# Create agent
export STATION_WORKSPACE=$(pwd)
stn agent create "AWS Cost Optimizer" --env production

# Edit agent prompt
vim environments/production/agents/aws-cost-optimizer.prompt

# Test agent locally
stn agent call aws-cost-optimizer "Analyze AWS costs" --env production

# Commit and push
git add environments/production/agents/aws-cost-optimizer.prompt
git commit -m "Add AWS cost optimizer agent"
git push origin feature/add-cost-optimizer
```

**Team reviews PR:**
- Review `.prompt` file changes in GitHub/GitLab
- Check agent logic, tools used, max_steps
- Test agent in staging environment
- Approve and merge to main

**Auto-deploy to production:**
```bash
# In production environment (e.g., via CI/CD)
cd /opt/station-config
git pull origin main
stn sync production  # Reload agents
```

### Workflow 2: Environment Promotion

**Test in staging:**
```bash
cd ~/code/station-config

# Create agent in staging
export STATION_WORKSPACE=$(pwd)
stn agent create "New Feature" --env staging

# Test thoroughly
stn agent call new-feature "Test input" --env staging

# If successful, copy to production
cp environments/staging/agents/new-feature.prompt \
   environments/production/agents/

# Commit
git add environments/production/agents/new-feature.prompt
git commit -m "Promote new-feature agent to production"
git push
```

### Workflow 3: Shared MCP Configuration

**Team-wide MCP servers defined in template.json:**

```json
// environments/production/template.json
{
  "name": "production",
  "description": "Production MCP servers",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    },
    "github": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-github@latest"
      ],
      "env": {
        "GITHUB_TOKEN": "{{ .GITHUB_TOKEN }}"
      }
    }
  }
}
```

**Team shares variables.yml with placeholders:**
```yaml
# environments/production/variables.yml
PROJECT_ROOT: /workspace
# GITHUB_TOKEN is set via environment variable on each deployment
```

All team members use the same MCP configuration, with environment-specific values.

---

## Best Practices

### 1. Separate Environments

Create distinct environments for different deployment stages:

```
environments/
├── development/    # Local development, experimental agents
├── staging/        # Pre-production testing
└── production/     # Live production agents
```

**Benefits:**
- Test agents in staging before production
- Rollback is just a git revert
- Environment-specific variables and MCP configs

### 2. Meaningful Commit Messages

```bash
# ❌ Bad
git commit -m "update agent"

# ✅ Good
git commit -m "Add SQL injection detection to security scanner agent"
git commit -m "Increase cost-analyzer max_steps from 5 to 8 for complex queries"
git commit -m "Fix timeout issue in infrastructure-scanner by reducing tool calls"
```

### 3. Code Review Agent Changes

Treat `.prompt` files like code:
- Review logic and reasoning instructions
- Check tool selection is appropriate
- Verify max_steps is reasonable
- Test agent before merging to main

### 4. Database Separation

**Never commit `station.db` to Git!**

**Options for database:**

**Local Development (each developer):**
```bash
# .gitignore includes station.db
# Each developer has their own local database
export STATION_WORKSPACE=$(pwd)
stn stdio  # Uses ./station.db (gitignored)
```

**Shared Team Database:**
```bash
# Use libsql cloud database for team collaboration
export DATABASE_URL="libsql://team-db.example.com?authToken=${TEAM_DB_TOKEN}"
stn stdio
```

**Production with Litestream:**
```bash
# Continuous backup to S3
# See DATABASE_REPLICATION.md for setup
docker-compose up  # with litestream.yml
```

### 5. Secrets Management

**Never commit secrets!** Use environment variables or secrets management:

```yaml
# ❌ Bad - secrets in variables.yml
GITHUB_TOKEN: ghp_hardcoded_token_12345
API_KEY: sk-real-api-key-67890

# ✅ Good - placeholders only
GITHUB_TOKEN: "{{ .GITHUB_TOKEN }}"
API_KEY: "{{ .API_KEY }}"
```

**Set secrets via environment:**
```bash
export GITHUB_TOKEN="ghp_..."
export API_KEY="sk-..."
stn sync production  # Variables resolved from environment
```

**Or use secrets management:**
```bash
# Load secrets from vault
export $(vault kv get -format=env secret/station)
stn sync production
```

### 6. Documentation

Include a `README.md` in your repository:

```markdown
# Team Station Configuration

## Setup
```bash
git clone git@github.com:team/station-config.git
cd station-config
export STATION_WORKSPACE=$(pwd)
stn stdio
```

## Agents

### Production
- `security-scanner` - Scans for OWASP Top 10 vulnerabilities
- `cost-analyzer` - AWS cost optimization recommendations
- `infrastructure-auditor` - Terraform security and compliance

### Staging
- Test versions of production agents

## Contributing
1. Create feature branch
2. Add/update agents in `environments/*/agents/`
3. Test locally with `stn agent call <agent>`
4. Create PR for team review
5. Merge to main after approval
```

---

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/sync-production.yml
name: Sync Production Agents

on:
  push:
    branches: [main]
    paths:
      - 'environments/production/**'

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Install Station
        run: |
          curl -sSL https://install.station.dev | bash
          echo "$HOME/.local/bin" >> $GITHUB_PATH
      
      - name: Sync Production Environment
        env:
          DATABASE_URL: ${{ secrets.PROD_DATABASE_URL }}
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          STATION_WORKSPACE: ${{ github.workspace }}
        run: |
          stn sync production
```

### GitLab CI

```yaml
# .gitlab-ci.yml
sync-production:
  stage: deploy
  only:
    - main
  changes:
    - environments/production/**
  script:
    - curl -sSL https://install.station.dev | bash
    - export PATH="$HOME/.local/bin:$PATH"
    - export STATION_WORKSPACE=$(pwd)
    - stn sync production
  environment:
    name: production
```

---

## Troubleshooting

### Problem: Agents not loading after git pull

**Solution:** Re-sync the environment
```bash
git pull
stn sync production
```

### Problem: Database conflicts

**Solution:** Don't commit `station.db`! Use cloud database or Litestream.
```bash
# Check .gitignore includes:
station.db
station.db-wal
station.db-shm
```

### Problem: Secret/token not resolved

**Solution:** Ensure environment variable is set before syncing
```bash
# Check variable
echo $GITHUB_TOKEN

# Set if missing
export GITHUB_TOKEN="ghp_..."

# Re-sync
stn sync production
```

### Problem: Wrong workspace being used

**Solution:** Verify STATION_WORKSPACE is set
```bash
# Check current workspace
echo $STATION_WORKSPACE

# Set correctly
export STATION_WORKSPACE=/path/to/your/station-config

# Or use config file
stn stdio --config /path/to/station-config/config.yaml
```

---

## Migration Guide

### Migrating Existing Station to GitOps

**1. Export current configuration:**
```bash
# Your current Station config is likely in:
cd ~/.config/station

# Copy to new Git repository
cp -r ~/.config/station ~/code/station-config
cd ~/code/station-config
```

**2. Initialize Git:**
```bash
git init
echo "station.db*" >> .gitignore
echo "*.log" >> .gitignore
git add .
git commit -m "Initial Station configuration"
```

**3. Test with new workspace:**
```bash
export STATION_WORKSPACE=$(pwd)
stn stdio  # Should load all your existing agents
```

**4. Push to remote:**
```bash
git remote add origin git@github.com:your-team/station-config.git
git push -u origin main
```

---

## Example Repository

See our [example Station configuration repository](https://github.com/cloudshipai/station-config-example) for a complete working example with:
- Multi-environment setup (dev/staging/prod)
- Sample agents for security, finops, and infrastructure
- CI/CD workflows for auto-deployment
- Comprehensive documentation

---

## Additional Resources

- **Database Replication:** See `DATABASE_REPLICATION.md`
- **Configuration Reference:** See `CONFIGURATION.md`
- **Station CLI Reference:** Run `stn --help`
- **Community Examples:** https://github.com/topics/station-agents
