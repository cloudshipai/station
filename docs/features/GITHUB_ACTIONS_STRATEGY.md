# GitHub Actions Strategy for Station

## Overview

Station provides **3 production-ready GitHub Actions workflows** that are automatically bootstrapped when users run `stn init --config`. These workflows enable complete CI/CD for agent teams without any configuration.

## Three Workflows Provided

### 1. `build-bundle.yml` - Bundle Builder
**Purpose**: Create `.tar.gz` agent bundles from environments

**Use Case**: 
- Distribute agent teams as installable bundles
- Publish to GitHub Releases for easy installation
- Version and share agent configurations

**Trigger**:
```bash
gh workflow run build-bundle.yml \
  -f environment_name=production \
  -f bundle_version=1.0.0
```

**Output**: 
- `production-1.0.0.tar.gz` bundle
- GitHub Release with install instructions
- Metadata JSON file

### 2. `build-env-image.yml` - Environment Image Builder
**Purpose**: Build deployment Docker images from local environments

**Use Case**:
- Deploy agents to production infrastructure
- Build from workspace environments
- Version control entire agent team deployments

**Trigger**:
```bash
gh workflow run build-env-image.yml \
  -f environment_name=production \
  -f image_tag=v1.0.0 \
  -f push_to_registry=true
```

**Output**:
- `ghcr.io/{org}/station-production:v1.0.0` image
- Deployment instructions (Docker, K8s, Compose)
- Ready-to-deploy container

### 3. `build-image-from-bundle.yml` - Bundle URL → Image Builder ⭐ NEW
**Purpose**: Build deployment images from ANY bundle URL

**Use Case**:
- **Consulting firms**: Deliver bundles to clients who build their own images
- **Third-party bundles**: Install community or vendor bundles
- **Decoupled workflows**: Bundle creation separate from image creation

**Trigger**:
```bash
gh workflow run build-image-from-bundle.yml \
  -f bundle_url=https://github.com/myorg/bundles/releases/download/v1.0.0/prod-1.0.0.tar.gz \
  -f environment_name=production \
  -f image_tag=v1.0.0
```

**Supported Bundle Sources**:
- ✅ GitHub Releases: `https://github.com/org/repo/releases/download/v1.0.0/bundle.tar.gz`
- ✅ HTTP Endpoints: `https://releases.mycompany.com/bundles/production-v1.tar.gz`
- ✅ Local Files: `./bundles/production.tar.gz` (from repo)

**Output**:
- Deployment-ready Docker image with bundle pre-installed
- Works with ANY bundle, from any source
- Perfect for service provider → customer workflows

## Reusable GitHub Actions (Future)

### Current State
- ✅ Workflows are **embedded templates** distributed with `stn init`
- ✅ Users get workflows automatically in their repos
- ✅ Zero configuration required

### Future: Publishable GitHub Actions

To make Station workflows even more reusable, we could publish official GitHub Actions:

```yaml
# .github/workflows/my-agents.yml
name: Build My Agents

on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      # Hypothetical future action
      - uses: cloudshipai/station-build-bundle@v1
        with:
          environment: production
          version: v1.0.0
      
      # Hypothetical future action
      - uses: cloudshipai/station-build-image@v1
        with:
          bundle-url: https://releases.mysite.com/bundle.tar.gz
          image-tag: v1.0.0
```

**Benefits**:
- Users write even less code
- Centralized updates (fix once, everyone benefits)
- Easier to maintain and version

**How to Publish**:
1. Create separate repo: `cloudshipai/station-actions`
2. Implement each workflow as composite action
3. Publish to GitHub Marketplace
4. Update Station to reference published actions

**Not Implemented Yet** - Current embedded templates work great for now!

## Business Models Enabled

### 1. SaaS/Consulting Firms
**Workflow**: Private repo → Build bundles → Share with customers

```bash
# Firm builds bundle
gh workflow run build-bundle.yml -f environment_name=client-xyz -f bundle_version=1.0.0

# Customer downloads bundle
wget https://github.com/firm/private-bundles/releases/download/v1.0.0/client-xyz-1.0.0.tar.gz

# Customer builds their own image
gh workflow run build-image-from-bundle.yml \
  -f bundle_url=https://github.com/firm/private-bundles/releases/download/v1.0.0/client-xyz-1.0.0.tar.gz \
  -f environment_name=production \
  -f image_tag=v1.0.0
```

### 2. Internal Platform Teams
**Workflow**: Central team maintains agents → Business units deploy

```bash
# Platform team publishes bundles
gh workflow run build-bundle.yml -f environment_name=finops-team

# Business units deploy to their infrastructure
gh workflow run build-image-from-bundle.yml \
  -f bundle_url=https://platform.company.com/bundles/finops-latest.tar.gz
```

### 3. Open Source / Community
**Workflow**: Publish bundles publicly → Anyone can deploy

```bash
# Community bundle install
gh workflow run build-image-from-bundle.yml \
  -f bundle_url=https://github.com/station-community/terraform-security/releases/download/v2.0.0/tf-sec-bundle.tar.gz
```

## Implementation Details

### Official Station Container
All workflows use `ghcr.io/cloudshipai/station:latest`:
- ✅ No building from source in CI
- ✅ Fast workflow execution
- ✅ Always uses latest stable Station
- ✅ Consistent behavior across workflows

### Auto-Detection
Image names automatically use repository owner:
```yaml
IMAGE_NAME: ${{ github.repository_owner }}/station-${{ github.event.inputs.environment_name }}
```

Example: `ghcr.io/acme-corp/station-production:v1.0.0`

### Zero Configuration
Just push to GitHub and workflows work:
- No YAML editing needed
- Just add `OPENAI_API_KEY` secret
- Trigger via GitHub Actions UI or `gh` CLI

## Testing

### Test Environment Image Build
```bash
# Initialize test workspace
mkdir test-workspace && cd test-workspace
stn init --config . --yes

# Create test environment
mkdir -p environments/test/agents
echo '{}' > environments/test/template.json

# Commit and push
git init && git add . && git commit -m "test"
git remote add origin git@github.com:myorg/test.git
git push -u origin main

# Trigger workflow
gh workflow run build-env-image.yml \
  -f environment_name=test \
  -f image_tag=test-v1 \
  -f push_to_registry=false
```

### Test Bundle → Image Build
```bash
# Using public bundle
gh workflow run build-image-from-bundle.yml \
  -f bundle_url=https://github.com/cloudshipai/station-bundles/releases/download/demo/demo-bundle.tar.gz \
  -f environment_name=demo \
  -f image_tag=test-v1
```

## Summary

✅ **3 workflows** automatically added to every `stn init`  
✅ **Zero configuration** - works out of the box  
✅ **Bundle URL → Image** workflow enables service provider workflows  
✅ **Uses official containers** - no source builds  
✅ **Production-ready** - tested and validated  

**Future**: Publish reusable GitHub Actions for even simpler integration
