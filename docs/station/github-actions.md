# Station Reusable GitHub Actions

Station provides production-ready, reusable GitHub Actions that simplify CI/CD workflows for agent deployment and bundle management.

## Running Agents in CI/CD

To run AI agents in your workflows, use the published [station-action](https://github.com/cloudshipai/station-action):

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: 'Code Reviewer'
    task: 'Review this PR for bugs and security issues'
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

The actions below are for **building and deploying** Station environments.

## Why Reusable Actions?

**Before** (Complex workflow with manual steps):
```yaml
# ❌ 60+ lines of complex Docker/tar/CLI commands
# ❌ Duplicated across repos
# ❌ Error-prone manual scripting
# ❌ Requires deep Station knowledge
```

**After** (Clean, declarative workflows):
```yaml
# ✅ 10 lines using reusable actions
# ✅ Same code across all repos
# ✅ Battle-tested, maintained by Station team
# ✅ Works out of the box
```

---

## Quick Start

### 1. Create a Bundle and Release It

```yaml
name: Release Bundle

on:
  push:
    tags: ['v*']

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Build Bundle
        id: bundle
        uses: cloudshipai/station/.github/actions/build-bundle@main
        with:
          environment: 'production'
          version: ${{ github.ref_name }}
      
      - name: Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            ${{ steps.bundle.outputs.bundle-path }}
            ${{ steps.bundle.outputs.metadata-path }}
```

**Result**: Tag `v1.0.0` → Automatic bundle creation + GitHub Release

---

### 2. Build Deployment Image from Local Environment

```yaml
name: Build Production Image

on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      packages: write
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Build & Push Image
        uses: cloudshipai/station/.github/actions/build-image@main
        with:
          source-type: 'environment'
          environment: 'production'
          image-name: '${{ github.repository_owner }}/station-production'
          image-tag: ${{ github.sha }}
          push: 'true'
          registry-username: ${{ github.actor }}
          registry-password: ${{ secrets.GITHUB_TOKEN }}
```

**Result**: Push to `main` → Automatic Docker image build + GHCR push

---

### 3. Customer Deployment (Download Vendor Bundle → Build Image)

Perfect for consulting firms delivering agent bundles to customers:

```yaml
name: Deploy Vendor Bundle

on:
  workflow_dispatch:
    inputs:
      bundle_url:
        description: 'Bundle URL from vendor'
        required: true
        default: 'https://vendor.com/bundles/agents-v1.0.0.tar.gz'

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      packages: write
    
    steps:
      - name: Build from Vendor Bundle
        uses: cloudshipai/station/.github/actions/build-image@main
        with:
          source-type: 'bundle'
          bundle-url: ${{ inputs.bundle_url }}
          image-name: 'my-company/station-production'
          image-tag: 'latest'
          push: 'true'
          registry-username: ${{ github.actor }}
          registry-password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/station \
            station=ghcr.io/my-company/station-production:latest
```

**Result**: Manual workflow trigger → Download bundle → Build image → Deploy

---

## Available Actions

### `setup-station` - Install Station CLI

Installs and configures Station CLI in GitHub Actions runners.

**Minimal Example:**
```yaml
- uses: cloudshipai/station/.github/actions/setup-station@main
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

**Full Example:**
```yaml
- uses: cloudshipai/station/.github/actions/setup-station@main
  with:
    version: 'latest'             # Or specific version like 'v0.22.0'
    provider: 'openai'            # Or 'anthropic', 'gemini', 'ollama'
    model: 'gpt-4o'               # Optional, uses provider default
    base-url: ''                  # For Azure OpenAI or Ollama
    workspace-path: './agents'    # Default: '.'
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

**Outputs:**
- `version`: Installed Station version
- `cli-path`: Path to `stn` binary

---

### `build-bundle` - Create Agent Bundles

Creates distributable `.tar.gz` bundles from environments.

**Minimal Example:**
```yaml
- uses: cloudshipai/station/.github/actions/build-bundle@main
  with:
    environment: 'production'
    version: '1.0.0'
```

**Full Example:**
```yaml
- uses: cloudshipai/station/.github/actions/build-bundle@main
  id: bundle
  with:
    environment: 'production'
    version: '1.2.3'
    output-path: 'my-custom-bundle.tar.gz'  # Optional
    workspace-path: './custom-workspace'     # Optional

- run: |
    echo "Bundle: ${{ steps.bundle.outputs.bundle-path }}"
    echo "Size: ${{ steps.bundle.outputs.bundle-size }}"
```

**Outputs:**
- `bundle-path`: Path to `.tar.gz` file
- `bundle-size`: Human-readable size
- `metadata-path`: Path to metadata JSON

---

### `build-image` - Build Deployment Images

Builds production Docker images from environments or bundle URLs.

**From Local Environment:**
```yaml
- uses: cloudshipai/station/.github/actions/build-image@main
  with:
    source-type: 'environment'
    environment: 'production'
    image-name: 'station-production'
    image-tag: 'v1.0.0'
    push: 'true'
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

**From Bundle URL:**
```yaml
- uses: cloudshipai/station/.github/actions/build-image@main
  with:
    source-type: 'bundle'
    bundle-url: 'https://releases.acme.com/bundle.tar.gz'
    image-name: 'station-production'
    image-tag: 'v1.0.0'
    push: 'true'
```

**Outputs:**
- `image-name`: Full image name with tag
- `image-digest`: SHA256 digest

---

## Real-World Use Cases

### Use Case 1: SaaS Company Multi-Environment Deployment

```yaml
name: Deploy to Environments

on:
  push:
    branches: [main, staging, develop]

jobs:
  deploy:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        environment: [production, staging, development]
    
    steps:
      - uses: actions/checkout@v4
      
      - uses: cloudshipai/station/.github/actions/build-image@main
        if: github.ref == format('refs/heads/{0}', matrix.environment)
        with:
          source-type: 'environment'
          environment: ${{ matrix.environment }}
          image-name: 'station-${{ matrix.environment }}'
          image-tag: ${{ github.sha }}
          push: 'true'
          registry-username: ${{ github.actor }}
          registry-password: ${{ secrets.GITHUB_TOKEN }}
```

**Result**: Push to `main` → Builds production image. Push to `staging` → Builds staging image.

---

### Use Case 2: Consulting Firm Bundle Distribution

**Service Provider Repo:**
```yaml
name: Publish Client Bundle

on:
  release:
    types: [published]

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: cloudshipai/station/.github/actions/build-bundle@main
        with:
          environment: 'client-production'
          version: ${{ github.ref_name }}
      
      - uses: softprops/action-gh-release@v1
        with:
          files: client-production-${{ github.ref_name }}.tar.gz
```

**Customer Repo:**
```yaml
name: Deploy Vendor Bundle

on:
  schedule:
    - cron: '0 2 * * 1'  # Weekly Monday 2AM

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: cloudshipai/station/.github/actions/build-image@main
        with:
          source-type: 'bundle'
          bundle-url: 'https://vendor.com/releases/latest/bundle.tar.gz'
          image-name: 'station-production'
          push: 'true'
```

**Result**: Service provider publishes bundles → Customers auto-deploy weekly

---

### Use Case 3: GitOps with ArgoCD

```yaml
name: Update Kubernetes Manifests

on:
  push:
    tags: ['v*']

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      # Build and push image
      - uses: cloudshipai/station/.github/actions/build-image@main
        id: build
        with:
          source-type: 'environment'
          environment: 'production'
          image-name: 'station-production'
          image-tag: ${{ github.ref_name }}
          push: 'true'
      
      # Update Kubernetes manifest
      - run: |
          sed -i "s|image:.*|image: ${{ steps.build.outputs.image-name }}|" k8s/deployment.yml
          git config user.name "GitHub Actions"
          git commit -am "chore: update image to ${{ github.ref_name }}"
          git push
```

**Result**: GitOps-ready deployment with automatic manifest updates

---

## Benefits

### For Developers
- **Zero Configuration**: Works out of the box, no Station knowledge required
- **Type-Safe**: GitHub validates all inputs/outputs
- **Reusable**: DRY principle across repos

### For DevOps Teams
- **Consistent Builds**: Same process everywhere
- **Centralized Maintenance**: Station team maintains actions
- **Composable**: Mix and match for custom workflows

### For Organizations
- **Faster Onboarding**: New teams productive immediately
- **Reduced Errors**: Battle-tested, production-proven
- **Service Model Ready**: Enable consulting/SaaS delivery models

---

## Advanced Examples

### Parallel Bundle Creation for Multiple Environments

```yaml
jobs:
  bundle:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        environment: [production, staging, development]
    
    steps:
      - uses: actions/checkout@v4
      
      - uses: cloudshipai/station/.github/actions/build-bundle@main
        with:
          environment: ${{ matrix.environment }}
          version: ${{ github.ref_name }}
```

### Build Image with Custom Dockerfile

```yaml
- uses: cloudshipai/station/.github/actions/build-image@main
  with:
    source-type: 'environment'
    environment: 'production'
    dockerfile: './custom.Dockerfile'  # Coming soon
    build-args: 'ENV=production'        # Coming soon
```

---

## Versioning

### Pin to Specific Version (Recommended for Production)
```yaml
uses: cloudshipai/station/.github/actions/setup-station@v0.21.1
```

### Use Latest (Good for Development)
```yaml
uses: cloudshipai/station/.github/actions/setup-station@main
```

### Pin to Commit (Maximum Stability)
```yaml
uses: cloudshipai/station/.github/actions/setup-station@d6ba798b
```

---

## Troubleshooting

### "API key not found"
- Ensure secret is set in repository: Settings → Secrets → Actions
- Use correct secret name in `api-key-secret` input

### "Environment directory not found"
- Verify environment exists: `./environments/<environment>/`
- Check `workspace-path` input points to correct directory

### "Permission denied pushing to registry"
- Add `permissions: { packages: write }` to job
- Verify `registry-username` and `registry-password` are correct

### "Bundle validation failed"
- Ensure environment contains at least one agent or MCP config
- Check template.json and variables.yml exist

---

## Comparison: Before vs After

### Before (Manual Workflow)
```yaml
# 80 lines of bash scripts
- Install Station CLI manually
- Configure provider manually  
- Write tar commands
- Handle Docker builds
- Manage registry login
- Error handling for each step
```

### After (Reusable Actions)
```yaml
# 3 lines, declarative
- uses: cloudshipai/station/.github/actions/build-image@main
  with:
    environment: 'production'
```

**Result**: 95% less code, 100% more reliable

---

## Next Steps

1. **Add to Your Repo**: Copy examples above into `.github/workflows/`
2. **Set Secrets**: Add `OPENAI_API_KEY` (or `GEMINI_API_KEY`) to repository secrets
3. **Test**: Push to trigger workflows
4. **Customize**: Adjust inputs for your use case

---

## Support

- **Documentation**: See `.github/actions/README.md` in Station repo
- **Examples**: https://github.com/cloudshipai/station-demo
- **Issues**: https://github.com/cloudshipai/station/issues
- **Community**: https://github.com/cloudshipai/station/discussions

---

## Verified Workflows

All workflows and reusable actions have been tested and verified in production:

### ✅ Container-based Workflow (`build-bundle.yml`)
**Repository**: [cloudshipai/station-demo](https://github.com/cloudshipai/station-demo)
- Uses `ghcr.io/cloudshipai/station:latest` container
- Tested with Station v0.21.2+
- Successfully creates bundles and GitHub releases
- **Key Fix**: `stn init --config ./config.yaml` now correctly sets workspace

###✅ Reusable Action (`build-bundle`)
**Location**: `.github/actions/build-bundle/action.yml`
- Downloads Station CLI from GitHub releases
- Runs `stn init` to create config
- Creates bundle using `stn bundle create`
- Generates metadata JSON
- **Verified**: Successfully tested in [station-demo](https://github.com/cloudshipai/station-demo/actions/workflows/test-reusable-action.yml)

### Testing Summary

| Component | Status | Version | Notes |
|-----------|--------|---------|-------|
| Container Workflow | ✅ Passing | v0.21.2 | Fixed workspace path bug |
| Reusable build-bundle | ✅ Passing | v0.21.2 | Downloads CLI, creates bundles |
| Reusable setup-station | ✅ Passing | v0.21.2 | CLI installation verified |
| Bundle Creation | ✅ Working | All | Creates .tar.gz + metadata |
| GitHub Releases | ✅ Working | All | Auto-uploads to releases |

### Known Fixes in v0.21.2

**Critical Bug Fixed**: Prior to v0.21.2, `stn init --config ./config.yaml` did not correctly set the workspace path. This caused bundle creation to fail in CI/CD environments.

**Before v0.21.2** (required workaround):
```bash
stn init --config ./config.yaml --yes
sed -i "s|workspace:.*|workspace: $(pwd)|g" config.yaml  # Manual fix needed
```

**v0.21.2+** (works correctly):
```bash
stn init --config ./config.yaml --yes  # Workspace automatically set correctly
```

