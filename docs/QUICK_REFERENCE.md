# Station Development Quick Reference

Quick commands and workflows for daily development.

## Daily Workflow

### Morning Routine
```bash
# Check what you're working on
gh issue list --assignee @me

# Pull latest changes
git pull origin main

# Check recent changes
git log --oneline -10
```

### Making Changes
```bash
# Make your changes, then commit with conventional format
git commit -m "feat(bundle): Add version field to bundle metadata"
git commit -m "fix(mcp): Resolve sync timeout for large configs"
git commit -m "docs: Update installation guide"

# Push to main (small changes) or create PR (large features)
git push origin main
```

### Quick Release
```bash
# Automated release (recommended)
./scripts/release.sh patch   # Bug fixes: 0.16.1 -> 0.16.2
./scripts/release.sh minor   # New features: 0.16.1 -> 0.17.0
./scripts/release.sh major   # Breaking changes: 0.16.1 -> 1.0.0
```

## GitHub CLI Quick Commands

### Issues
```bash
# Create issue
gh issue create --title "Add S3 storage backend" --label feature,storage

# List your issues
gh issue list --assignee @me

# List by milestone
gh issue list --milestone v0.17.0

# Close issue
gh issue close 123

# Add comment
gh issue comment 123 --body "Working on this now"
```

### Pull Requests
```bash
# Create PR from current branch
gh pr create --title "Feature: Add bundle versioning" --body "Closes #123"

# Quick PR with autofill
gh pr create --fill

# List PRs
gh pr list

# Check PR status
gh pr checks

# Merge PR
gh pr merge --squash
```

### Releases
```bash
# View latest release
gh release view

# List releases
gh release list --limit 5

# View specific release
gh release view v0.16.1

# Download release asset
gh release download v0.16.1 -p "station_*_linux_amd64.tar.gz"
```

### Workflows
```bash
# List recent workflow runs
gh run list --limit 10

# Watch active run
gh run watch

# View specific run
gh run view 18447349179

# Rerun failed workflow
gh run rerun 18447349179
```

## Commit Message Format

### Structure
```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation
- `refactor:` - Code refactoring
- `test:` - Adding tests
- `chore:` - Maintenance
- `perf:` - Performance
- `ci:` - CI/CD changes

### Examples
```bash
# Feature
git commit -m "feat(bundle): Add S3 storage backend for bundles"

# Bug fix with scope
git commit -m "fix(mcp): Resolve timeout during sync for large configs"

# Documentation
git commit -m "docs: Add troubleshooting guide for common errors"

# Multi-line with body
git commit -m "feat(agent): Add agent memory for context persistence

Agents can now maintain context across multiple runs using a
persistent memory layer backed by SQLite.

Closes #145"
```

## Building & Testing

### Local Development
```bash
# Full rebuild (UI + binary + Docker)
make rebuild-all

# Build binary only
make build

# Build UI only
cd ui && npm run build

# Run tests
make test

# Run specific test
go test ./internal/services -v -run TestAgentExecution
```

### Docker
```bash
# Build Docker image
docker build -t station:local .

# Run container
docker run -p 8585:8585 station:local

# Check logs
docker logs station-server

# Clean up
docker stop station-server && docker rm station-server
```

## Debugging

### Logs
```bash
# Station logs (when running via stn serve)
stn logs

# Follow logs in real-time
stn logs --follow

# Docker logs
docker logs -f station-server

# Debug mode
STN_DEBUG=true stn serve
```

### Common Issues
```bash
# Port already in use
lsof -ti:8585 | xargs kill -9

# Clean up containers
docker stop $(docker ps -q --filter "name=station")
docker rm $(docker ps -aq --filter "name=station")

# Reset database
rm ~/.config/station/station.db
stn init
```

## Release Checklist

### Pre-Release
- [ ] All tests passing: `make test`
- [ ] No uncommitted changes: `git status`
- [ ] On main branch: `git checkout main && git pull`
- [ ] Review commits: `git log $(git describe --tags --abbrev=0)..HEAD`

### Release
```bash
# Automated (recommended)
./scripts/release.sh patch  # or minor, major

# Manual (if needed)
git tag -a v0.17.0 -m "Release v0.17.0"
git push origin v0.17.0
```

### Post-Release
- [ ] GitHub Actions completed: `gh run list --workflow=release.yml --limit 1`
- [ ] Release created: `gh release view v0.17.0`
- [ ] Docker image available: `docker pull ghcr.io/cloudshipai/station:v0.17.0`
- [ ] Close milestone: `gh api "/repos/cloudshipai/station/milestones/{id}" -X PATCH -f state=closed`
- [ ] Update project board

## Version Management

### Current Version
```bash
# Check current version
cat VERSION

# Check latest tag
git describe --tags --abbrev=0

# Check remote tags
git ls-remote --tags origin
```

### Semantic Versioning
- **Major (X.0.0)**: Breaking API changes
- **Minor (0.X.0)**: New features (backward compatible)
- **Patch (0.0.X)**: Bug fixes

### Version Bumping
```bash
# Use release script (handles everything)
./scripts/release.sh patch   # 0.16.1 -> 0.16.2
./scripts/release.sh minor   # 0.16.1 -> 0.17.0
./scripts/release.sh major   # 0.16.1 -> 1.0.0
```

## Project Organization

### Important Files
- `VERSION` - Current version number
- `CHANGELOG.md` - Release history
- `docs/ROADMAP.md` - Feature roadmap
- `docs/DEVELOPMENT_WORKFLOW.md` - Full workflow guide
- `.github/workflows/release.yml` - Release automation
- `scripts/release.sh` - Release script

### Directory Structure
```
station/
├── cmd/main/              # CLI entry point
├── internal/
│   ├── api/              # REST API
│   ├── services/         # Business logic
│   ├── lighthouse/       # CloudShip integration
│   └── config/           # Configuration
├── ui/                   # React frontend
├── scripts/              # Automation scripts
├── docs/                 # Documentation
└── .github/              # GitHub config
```

## Useful Aliases

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
# Station development
alias stn-dev='cd ~/projects/hack/station'
alias stn-build='make rebuild-all'
alias stn-test='make test && echo "✓ All tests passed"'
alias stn-logs='stn logs --follow'

# Git shortcuts
alias gs='git status'
alias gp='git pull'
alias gc='git commit -m'
alias gps='git push origin main'

# GitHub CLI shortcuts
alias ghil='gh issue list --assignee @me'
alias ghic='gh issue create'
alias ghpr='gh pr create --fill'
alias ghrl='gh release list --limit 5'

# Release shortcuts
alias stn-release-patch='./scripts/release.sh patch'
alias stn-release-minor='./scripts/release.sh minor'
```

## Environment Variables

### Development
```bash
export STN_DEBUG=true                    # Enable debug logging
export STN_AI_PROVIDER=openai          # AI provider (openai, ollama, gemini)
export STN_AI_API_KEY=your-key-here    # AI API key
export STN_AI_MODEL=gpt-4o-mini        # AI model
```

### CloudShip Integration
```bash
export STN_CLOUDSHIP_ENABLED=true
export STN_CLOUDSHIP_KEY=your-key-here
export STN_CLOUDSHIP_ENDPOINT=lighthouse.cloudshipai.com:50051
```

## Troubleshooting

### GitHub Actions Not Triggering
```bash
# Verify tag pushed
git ls-remote --tags origin | grep v0.17.0

# Check workflow runs
gh run list --workflow=release.yml --limit 5

# Manually trigger (if workflow supports it)
gh workflow run release.yml
```

### Release Script Fails
```bash
# Check git status
git status

# Ensure on main branch
git checkout main && git pull

# Check for uncommitted changes
git stash
./scripts/release.sh patch
git stash pop
```

### Docker Image Not Updating
```bash
# Clear local images
docker rmi ghcr.io/cloudshipai/station:latest

# Pull fresh image
docker pull ghcr.io/cloudshipai/station:latest

# Verify version
docker run ghcr.io/cloudshipai/station:latest stn --version
```

## Getting Help

- **Documentation**: `docs/` directory
- **Issues**: https://github.com/cloudshipai/station/issues
- **Discussions**: https://github.com/cloudshipai/station/discussions
- **Workflow Guide**: `docs/DEVELOPMENT_WORKFLOW.md`

---
*Quick reference for Station development workflow*
*Last updated: 2025-10-12*
