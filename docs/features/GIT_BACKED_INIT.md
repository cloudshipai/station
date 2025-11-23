# Git-Backed Workspace Initialization

## Overview

Enhance `stn init` command to bootstrap a complete git-backed workspace with GitHub Actions CI/CD workflows, enabling teams to get production-ready agent team repositories in seconds.

## Current Behavior

```bash
# Standard init - local workspace
stn init

# Config-based init - custom workspace location
stn init --config /path/to/config.yaml
```

Creates:
- `station.db` database
- `~/.config/station/` workspace (or custom location)
- Basic configuration

## Proposed Enhancement

```bash
# Git-backed workspace with CI/CD workflows
stn init --git-backed
```

This creates a complete production-ready git repository structure with:
- Environment directories
- GitHub Actions workflows
- Gitignore configuration
- README with instructions
- Example agent configurations

## User Experience

### Interactive Flow

```bash
$ stn init --git-backed

🚀 Station Git-Backed Workspace Setup

This will create a production-ready git repository with:
✓ Environment structure (environments/)
✓ GitHub Actions CI/CD workflows
✓ Deployment guides
✓ Example agent configurations

? Repository name: my-agent-team
? Create initial environment? (Y/n): y
? Environment name: production
? Initialize git repository? (Y/n): y
? Add GitHub Actions workflows? (Y/n): y

Creating git-backed workspace...

✓ Created repository structure
✓ Initialized git repository
✓ Added GitHub Actions workflows (.github/workflows/)
✓ Created production environment skeleton
✓ Generated .gitignore
✓ Created README.md

📁 Repository created at: ./my-agent-team/

Next steps:
1. cd my-agent-team
2. Create your first agent: stn agent create --name "My Agent" --env production
3. Export agents: stn agent export-agents --env production
4. Commit: git add . && git commit -m "feat: initial agent team"
5. Push: git remote add origin <url> && git push -u origin main
6. Trigger build: gh workflow run build-env-image.yml -f environment_name=production -f image_tag=v1.0.0
```

## Repository Structure Created

```
my-agent-team/
├── .github/
│   └── workflows/
│       ├── build-bundle.yml           # Bundle building workflow
│       └── build-env-image.yml        # Container image building workflow
├── environments/
│   └── production/
│       ├── agents/                    # Agent .prompt files (empty initially)
│       ├── template.json              # MCP server configuration
│       └── variables.yml.template     # Template for environment variables
├── .gitignore                         # Ignore station.db, *.tar.gz, etc.
├── config.yaml                        # Station workspace configuration
├── README.md                          # Setup and usage instructions
└── DEPLOYMENT.md                      # Deployment guide
```

## Files Generated

### config.yaml
```yaml
workspace: .
database_path: ./station.db
```

### .gitignore
```
# Station runtime files
station.db
station.db-*

# Build artifacts
bundles/*.tar.gz
*.tar.gz

# Environment-specific
environments/*/variables.yml

# Test artifacts
test-deployments/
test-docker-deploy/
dev-workspace/

# Logs
*.log
debug-*.log
```

### environments/production/template.json
```json
{
  "name": "production",
  "description": "Production agent team environment",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    }
  }
}
```

### environments/production/variables.yml.template
```yaml
# Copy this file to variables.yml and fill in your values
# DO NOT commit variables.yml to git (it's in .gitignore)
PROJECT_ROOT: "/workspace"
```

### README.md
```markdown
# My Agent Team

Production-ready Station agent team with GitOps workflows.

## Quick Start

### 1. Create Your First Agent

```bash
stn agent create \
  --name "My Agent" \
  --description "Agent description" \
  --env production
```

### 2. Export Agent Configuration

```bash
stn agent export-agents \
  --env production \
  --output-directory environments/production/agents/
```

### 3. Commit and Push

```bash
git add environments/production/agents/
git commit -m "feat: add my agent"
git push origin main
```

### 4. Build Container Image

Trigger GitHub Actions workflow to build deployment image:

```bash
gh workflow run build-env-image.yml \
  -f environment_name=production \
  -f image_tag=v1.0.0 \
  -f push_to_registry=true
```

## Workflows

- **build-bundle.yml**: Creates installable agent bundles
- **build-env-image.yml**: Builds deployment container images

## Deployment

Images are published to: `ghcr.io/yourorg/station-demo-production:v1.0.0`

See [DEPLOYMENT.md](DEPLOYMENT.md) for Kubernetes and Docker Compose examples.
```

### DEPLOYMENT.md
(Content from our DevOps workflow guide, customized for this repo)

## Command Implementation

### Location
`cmd/main/init.go` or new `cmd/main/git_backed_init.go`

### Key Functions

```go
type GitBackedInitOptions struct {
    RepoName      string
    EnvName       string
    InitGit       bool
    AddWorkflows  bool
    Interactive   bool
}

func runGitBackedInit(opts GitBackedInitOptions) error {
    // 1. Create directory structure
    // 2. Generate config.yaml
    // 3. Create .gitignore
    // 4. Create initial environment skeleton
    // 5. Copy workflow templates
    // 6. Generate README and DEPLOYMENT docs
    // 7. Initialize git repository (if requested)
    // 8. Create initial commit
    return nil
}

func copyWorkflowTemplates(repoPath string) error {
    // Embed workflows as Go templates
    // Write to .github/workflows/
    return nil
}

func generateEnvironmentSkeleton(envPath, envName string) error {
    // Create template.json
    // Create variables.yml.template
    // Create agents/ directory with .gitkeep
    return nil
}
```

## Embedded Templates

Use Go embed to include workflow templates in the binary:

```go
//go:embed templates/workflows/*.yml
var workflowTemplates embed.FS

//go:embed templates/docs/*.md
var docTemplates embed.FS
```

## CLI Flags

```bash
stn init --git-backed [options]

Options:
  --repo-name string      Repository name (default: current directory name)
  --env-name string       Initial environment name (default: "production")
  --no-git                Skip git initialization
  --no-workflows          Skip GitHub Actions workflows
  --non-interactive       Use defaults, no prompts
```

## Business Value

### For Station Team
- **Instant Setup**: Users get production-ready repos in seconds
- **Best Practices**: Enforces recommended structure
- **Onboarding**: Reduces friction for new teams
- **Documentation**: Self-documenting setup

### For Customers
- **Fast Start**: Clone → init → develop → deploy
- **CI/CD Ready**: Workflows included out of the box
- **GitOps Native**: Version control from day one
- **Production Ready**: No guesswork on structure

## Testing Plan

### Unit Tests
- Directory structure creation
- File generation
- Template rendering
- Git initialization

### Integration Tests
```bash
# Test complete flow
stn init --git-backed --repo-name test-repo --non-interactive
cd test-repo
stn agent create --name "Test Agent" --env production
stn agent export-agents --env production
git add . && git commit -m "test"

# Verify structure
test -f .github/workflows/build-bundle.yml
test -f .github/workflows/build-env-image.yml
test -f config.yaml
test -f README.md
test -d environments/production
```

### Manual Testing
1. Create new repo with `stn init --git-backed`
2. Create agents via MCP or CLI
3. Export agent configurations
4. Commit and push to GitHub
5. Trigger workflow manually
6. Verify image builds and pushes to GHCR
7. Deploy image to Kubernetes
8. Test agent execution in deployed environment

## Future Enhancements

### Phase 1 (MVP)
- [x] Basic repository structure
- [x] Workflow templates
- [x] README generation
- [ ] Interactive prompts
- [ ] Git initialization

### Phase 2
- [ ] Custom workflow templates (user can override)
- [ ] Environment presets (dev/staging/prod)
- [ ] Auto-detect GitHub repo and configure workflows
- [ ] Template marketplace (select from common setups)

### Phase 3
- [ ] Integration with `stn deploy` command
- [ ] Automatic GitHub repo creation via GitHub CLI
- [ ] Secret management setup (configure GitHub secrets)
- [ ] Pre-configured MCP server templates

## Migration Path

Existing users can migrate to git-backed structure:

```bash
# In existing ~/.config/station directory
stn export workspace --output ./my-agent-repo --git-backed

# This creates a new git-backed repo from existing workspace
cd my-agent-repo
git init
git add .
git commit -m "feat: migrate to git-backed workspace"
```

## Related Work

- [DevOps Workflow Guide](../DEVOPS_WORKFLOW.md)
- [Bundle Creation](./BUNDLE_CREATION.md)
- [Environment Management](./ENVIRONMENT_MANAGEMENT.md)

## Implementation Checklist

- [ ] Create embedded workflow templates
- [ ] Create embedded documentation templates
- [ ] Implement `GitBackedInitOptions` struct
- [ ] Implement `runGitBackedInit()` function
- [ ] Add interactive prompts with survey/bubbletea
- [ ] Add CLI flags to init command
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Update main README with `--git-backed` example
- [ ] Create demo video/GIF
- [ ] Update docs

## Example Usage Scenarios

### Scenario 1: New Team Starting Fresh
```bash
stn init --git-backed --repo-name finops-agents
cd finops-agents
stn agent create --name "Cost Analyzer" --env production
# ... create more agents
stn agent export-agents --env production
git add . && git commit -m "feat: finops agent team"
git remote add origin git@github.com:myorg/finops-agents.git
git push -u origin main
gh workflow run build-env-image.yml -f environment_name=production -f image_tag=v1.0.0
```

### Scenario 2: Consulting Firm Delivering to Client
```bash
# Create agent team for client
stn init --git-backed --repo-name client-xyz-security-agents
cd client-xyz-security-agents

# Build agents
stn agent create --name "Compliance Scanner" --env production
stn agent create --name "Threat Detector" --env production
stn agent export-agents --env production

# Deliver to client as private repo
git remote add origin git@github.com:client-xyz/security-agents.git
git push -u origin main

# Client receives:
# - Complete git repository
# - CI/CD workflows ready
# - Just needs to add GitHub secrets (OPENAI_API_KEY)
# - Run workflow to build image
# - Deploy to their infrastructure
```

### Scenario 3: Internal Platform Team
```bash
# Create template repository
stn init --git-backed --repo-name station-template
cd station-template

# Teams clone template
git clone git@github.com:myorg/station-template.git myteam-agents
cd myteam-agents

# Customize and deploy
stn agent create --name "Team Agent" --env production
# ... rest of workflow
```

## Success Metrics

- Time to first deployed agent team: < 10 minutes
- Users who adopt git-backed workflow: > 70%
- Customer satisfaction with setup process: > 4.5/5
- Support tickets related to setup: -50%

---

**Status**: Design Complete  
**Next**: Implementation  
**Owner**: Station Core Team  
**Target Release**: v0.21.0
