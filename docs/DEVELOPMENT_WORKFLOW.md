# Station Development Workflow

This document outlines the complete development workflow for Station, from ideation to release.

## Overview

Station uses a **trunk-based development** model with automated releases and comprehensive tracking:

- **Main Branch**: Production-ready code, always deployable
- **Feature Development**: Short-lived feature branches or direct commits to main
- **Release Automation**: GitHub Actions handles building, testing, and releasing
- **Issue Tracking**: GitHub Issues with labels for organization
- **Roadmap**: GitHub Projects for planning and visibility

## Workflow Stages

### 1. Planning & Roadmap

**Location**: [GitHub Projects](https://github.com/cloudshipai/station/projects)

**Process**:
1. Create issues for new features/bugs in GitHub
2. Add to project board with status (Backlog, Planned, In Progress, Done)
3. Assign priority labels (P0-Critical, P1-High, P2-Medium, P3-Low)
4. Tag with release milestone

**Commands**:
```bash
# Create a feature issue
stn dev issue create "Add bundle versioning support" --label feature --priority P1 --milestone v0.17.0

# Create a bug issue
stn dev issue create "Fix MCP sync timeout" --label bug --priority P0

# List issues for current milestone
stn dev issue list --milestone current
```

### 2. Development

**Branch Strategy**:
- **Small changes**: Commit directly to `main`
- **Large features**: Create feature branch (`feature/bundle-versioning`)
- **Bug fixes**: Create fix branch (`fix/mcp-sync-timeout`)

**Development Cycle**:
```bash
# 1. Pull latest
git pull origin main

# 2. Create feature branch (optional for large features)
git checkout -b feature/bundle-versioning

# 3. Make changes with conventional commits
git commit -m "feat: Add version field to bundle metadata"
git commit -m "test: Add bundle version validation tests"
git commit -m "docs: Update bundle creation guide with versioning"

# 4. Push and create PR (or push to main for small changes)
git push origin feature/bundle-versioning
gh pr create --title "Add bundle versioning support" --body "Closes #123"
```

**Conventional Commits** (for automated changelog):
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Adding tests
- `chore:` - Maintenance tasks
- `perf:` - Performance improvements
- `ci:` - CI/CD changes

### 3. Testing

**Automated Testing**:
- CI runs on every push to `main` or PR
- Tests must pass before merge
- Docker image builds validated

**Manual Testing**:
```bash
# Build and test locally
make rebuild-all

# Run tests
go test ./...

# Test UI
cd ui && npm test

# Integration test
./scripts/test-integration.sh
```

### 4. Code Review (Optional)

For large features or breaking changes:
```bash
# Create PR
gh pr create --title "Feature: Add bundle versioning" --body "$(cat <<EOF
## Description
Adds version field to bundle metadata for better tracking.

## Changes
- Added version field to Bundle struct
- Updated bundle creation CLI
- Added version validation

## Testing
- Unit tests added
- Manual testing on 5 bundles

Closes #123
EOF
)"

# Review and merge
gh pr review --approve
gh pr merge --squash
```

### 5. Changelog & Release

**Automated Process** (on version tag push):

```bash
# 1. Update version (automated by release script)
./scripts/release.sh patch  # or minor, major

# This script will:
# - Bump version in VERSION file
# - Generate changelog from commits since last tag
# - Create git tag
# - Push tag (triggers GitHub Actions)
```

**GitHub Actions automatically**:
- Builds binaries for all platforms
- Creates Docker images (multi-arch)
- Generates changelog from commits
- Creates GitHub release
- Updates `latest` tag

**Manual Release** (if needed):
```bash
# Tag version manually
git tag -a v0.17.0 -m "Release v0.17.0"
git push origin v0.17.0

# GitHub Actions handles the rest
```

### 6. Post-Release

**Automated**:
- GitHub release created with changelog
- Docker images published to GHCR
- Installation script updated
- Discord webhook notification (if configured)

**Manual**:
- Update roadmap project (move items to "Done")
- Close milestone
- Announce on social media/blog

## Automation Tools

### CLI Tool: `stn dev`

Station includes a development CLI for workflow automation:

```bash
# Issue management
stn dev issue create "Feature description" --label feature --priority P1
stn dev issue list --milestone v0.17.0
stn dev issue close 123

# Changelog generation
stn dev changelog generate --from v0.16.0 --to HEAD
stn dev changelog preview

# Release management
stn dev release prepare --version 0.17.0  # Creates changelog, updates version
stn dev release publish --version 0.17.0  # Creates tag and pushes

# Roadmap management
stn dev roadmap show
stn dev roadmap add "Q1 2025: Multi-cloud support"
```

### GitHub CLI Integration

```bash
# Quick issue creation
gh issue create --title "Add S3 storage backend" \
  --label feature,storage --milestone v0.18.0

# List my assigned issues
gh issue list --assignee @me

# Create PR from current branch
gh pr create --fill

# Check CI status
gh pr checks

# View release
gh release view v0.16.1
```

## File Organization

```
station/
├── docs/
│   ├── DEVELOPMENT_WORKFLOW.md      # This file
│   ├── ROADMAP.md                   # Feature roadmap
│   ├── CHANGELOG.md                 # Auto-generated changelog
│   ├── CONTRIBUTING.md              # Contribution guidelines
│   └── releases/
│       ├── v0.16.0.md              # Release notes
│       └── v0.17.0.md
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                  # Test on PR/push
│   │   ├── release.yml             # Build/release on tag
│   │   └── changelog.yml           # Auto-generate changelog
│   ├── ISSUE_TEMPLATE/
│   │   ├── feature.md
│   │   ├── bug.md
│   │   └── enhancement.md
│   └── PULL_REQUEST_TEMPLATE.md
├── scripts/
│   ├── release.sh                  # Release automation
│   ├── changelog.sh                # Changelog generation
│   └── test-integration.sh         # Integration tests
└── VERSION                         # Current version
```

## Issue Labels

**Type**:
- `feature` - New feature
- `bug` - Bug fix
- `enhancement` - Improvement to existing feature
- `docs` - Documentation
- `refactor` - Code refactoring
- `test` - Testing improvements

**Priority**:
- `P0-critical` - Breaking issues, security
- `P1-high` - Important features/bugs
- `P2-medium` - Nice to have
- `P3-low` - Future consideration

**Component**:
- `agent` - Agent system
- `mcp` - MCP server management
- `ui` - User interface
- `cli` - Command-line interface
- `api` - API server
- `lighthouse` - CloudShip integration
- `bundle` - Bundle system

**Status**:
- `blocked` - Waiting on dependency
- `help-wanted` - Community contributions welcome
- `good-first-issue` - Easy for newcomers

## Release Cycles

### Semantic Versioning

Station follows [SemVer](https://semver.org/):
- **Major (X.0.0)**: Breaking changes
- **Minor (0.X.0)**: New features (backward compatible)
- **Patch (0.0.X)**: Bug fixes

### Release Schedule

- **Patch releases**: As needed (bug fixes)
- **Minor releases**: Every 2-4 weeks (new features)
- **Major releases**: Every 6-12 months (breaking changes)

### Release Checklist

```bash
# 1. Ensure main is clean and tests pass
git checkout main && git pull
make test

# 2. Review unreleased commits
git log $(git describe --tags --abbrev=0)..HEAD --oneline

# 3. Generate changelog preview
./scripts/changelog.sh preview

# 4. Create release (automated)
./scripts/release.sh minor  # or patch, major

# 5. Verify GitHub Actions completed
gh run list --workflow=release.yml --limit 1

# 6. Test installation
curl -sSL https://getstation.cloudshipai.com | bash

# 7. Update project board and close milestone
gh issue list --milestone v0.17.0
gh api "/repos/cloudshipai/station/milestones/{milestone_number}" -X PATCH -f state=closed
```

## Continuous Integration

### On Every Push/PR
- Go tests (`go test ./...`)
- UI tests (`npm test`)
- Linting (`golangci-lint`)
- Build validation
- Docker image build

### On Tag Push
- All CI checks
- Multi-platform binary builds
- Multi-arch Docker images
- GitHub release creation
- Changelog generation
- Discord notification

## Development Best Practices

### Commit Messages
```bash
# Good
git commit -m "feat(bundle): Add versioning support for bundle metadata"
git commit -m "fix(mcp): Resolve timeout during sync for large configs"
git commit -m "docs: Update installation guide with Docker instructions"

# Bad
git commit -m "fixed stuff"
git commit -m "WIP"
git commit -m "Update code"
```

### PR Titles
```
feat: Add bundle versioning support
fix: Resolve MCP sync timeout for large configs
docs: Update installation guide with Docker instructions
refactor: Simplify agent execution queue logic
```

### Issue Titles
```
Feature: Add S3 backend for bundle storage
Bug: MCP sync times out on large configurations
Enhancement: Improve agent canvas performance
Docs: Add troubleshooting guide for common errors
```

## Quick Reference

### Daily Workflow
```bash
# Morning: Check issues and plan
gh issue list --assignee @me

# Development: Make changes with good commits
git commit -m "feat: Add feature X"

# Testing: Validate changes
make test

# Push: Send to GitHub (triggers CI)
git push origin main

# Release: Automated on tag
./scripts/release.sh patch
```

### Getting Help
- **Questions**: GitHub Discussions
- **Bugs**: GitHub Issues with `bug` label
- **Features**: GitHub Issues with `feature` label
- **Contributing**: See CONTRIBUTING.md

## Next Steps

1. **Set up GitHub Projects** for roadmap visibility
2. **Create issue templates** for consistency
3. **Implement `stn dev` CLI** for automation
4. **Set up Discord webhooks** for notifications
5. **Create release automation script** (`scripts/release.sh`)

---
*Last updated: 2025-10-12*
