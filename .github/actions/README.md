# Station Reusable GitHub Actions

This directory contains reusable GitHub Actions that simplify Station deployment and bundle management workflows.

## Available Actions

### 1. `setup-station` - Install and Configure Station CLI

Sets up Station CLI in GitHub Actions runners with automatic installation and configuration.

**Inputs:**
- `version`: Station version (default: `latest`)
- `provider`: AI provider (`openai`, `gemini`, `anthropic`) 
- `model`: AI model name (default: `gpt-4o-mini`)
- `api-key-secret`: **Required** - GitHub secret containing API key
- `workspace-path`: Workspace directory (default: `.`)

**Outputs:**
- `version`: Installed Station version
- `cli-path`: Path to Station CLI binary

**Example Usage:**
```yaml
- name: Setup Station
  uses: cloudshipai/station/.github/actions/setup-station@main
  with:
    version: 'latest'
    provider: 'openai'
    model: 'gpt-4o-mini'
    api-key-secret: ${{ secrets.OPENAI_API_KEY }}
```

---

### 2. `build-bundle` - Create Station Agent Bundles

Creates distributable `.tar.gz` bundles from Station environments with metadata.

**Inputs:**
- `environment`: **Required** - Environment name to bundle
- `version`: Bundle version (default: `1.0.0`)
- `output-path`: Custom output path (optional)
- `workspace-path`: Workspace directory (default: `.`)

**Outputs:**
- `bundle-path`: Path to created bundle file
- `bundle-size`: Human-readable bundle size
- `metadata-path`: Path to bundle metadata JSON

**Example Usage:**
```yaml
- name: Build Agent Bundle
  uses: cloudshipai/station/.github/actions/build-bundle@main
  with:
    environment: 'production'
    version: '1.2.3'
```

---

### 3. `build-image` - Build Deployment Docker Images

Builds production-ready Docker images from Station environments or bundles.

**Inputs:**
- `source-type`: **Required** - `environment` (local) or `bundle` (URL/ID)
- `environment`: Environment name (required if `source-type=environment`)
- `bundle-url`: Bundle URL (required if `source-type=bundle` and no `bundle-id`)
- `bundle-id`: CloudShip bundle ID (UUID) - alternative to `bundle-url`
- `cloudship-api-key`: API key for CloudShip bundle downloads (required with `bundle-id`)
- `image-name`: **Required** - Docker image name
- `image-tag`: Image tag (default: `latest`)
- `registry`: Container registry (default: `ghcr.io`)
- `registry-username`: Registry username (optional)
- `registry-password`: Registry password/token (optional)
- `push`: Push to registry (default: `false`)

**Outputs:**
- `image-name`: Full image name with tag
- `image-digest`: Image SHA256 digest

**Example Usage (from environment):**
```yaml
- name: Build Deployment Image
  uses: cloudshipai/station/.github/actions/build-image@main
  with:
    source-type: 'environment'
    environment: 'production'
    image-name: 'station-production'
    image-tag: 'v1.0.0'
    push: 'true'
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

**Example Usage (from bundle URL):**
```yaml
- name: Build from Bundle
  uses: cloudshipai/station/.github/actions/build-image@main
  with:
    source-type: 'bundle'
    bundle-url: 'https://releases.mycompany.com/bundles/prod-1.0.0.tar.gz'
    image-name: 'station-production'
    image-tag: 'v1.0.0'
    push: 'true'
```

**Example Usage (from CloudShip bundle ID):**
```yaml
- name: Build from CloudShip Bundle
  uses: cloudshipai/station/.github/actions/build-image@main
  with:
    source-type: 'bundle'
    bundle-id: 'e26b414a-f076-4135-927f-810bc1dc892a'
    cloudship-api-key: ${{ secrets.CLOUDSHIP_API_KEY }}
    image-name: 'station-production'
    image-tag: 'v1.0.0'
    push: 'true'
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

---

## Complete Workflow Examples

### Example 1: Build and Release Bundle

```yaml
name: Release Agent Bundle

on:
  release:
    types: [created]

jobs:
  build-bundle:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Build Bundle
        id: bundle
        uses: cloudshipai/station/.github/actions/build-bundle@main
        with:
          environment: 'production'
          version: ${{ github.ref_name }}
      
      - name: Upload to Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            ${{ steps.bundle.outputs.bundle-path }}
            ${{ steps.bundle.outputs.metadata-path }}
```

### Example 2: Build and Push Deployment Image

```yaml
name: Build Deployment Image

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  build-image:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Station
        uses: cloudshipai/station/.github/actions/setup-station@main
        with:
          api-key-secret: ${{ secrets.OPENAI_API_KEY }}
      
      - name: Build and Push Image
        uses: cloudshipai/station/.github/actions/build-image@main
        with:
          source-type: 'environment'
          environment: 'production'
          image-name: '${{ github.repository_owner }}/station-production'
          image-tag: ${{ github.sha }}
          push: 'true'
          registry: 'ghcr.io'
          registry-username: ${{ github.actor }}
          registry-password: ${{ secrets.GITHUB_TOKEN }}
```

### Example 3: Customer Deployment (CloudShip Bundle → Image)

Perfect for deploying bundles from CloudShip:

```yaml
name: Deploy CloudShip Bundle

on:
  workflow_dispatch:
    inputs:
      bundle_id:
        description: 'CloudShip bundle ID (UUID)'
        required: true
      version:
        description: 'Version tag'
        required: true

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      packages: write
    
    steps:
      - name: Build from CloudShip Bundle
        uses: cloudshipai/station/.github/actions/build-image@main
        with:
          source-type: 'bundle'
          bundle-id: ${{ inputs.bundle_id }}
          cloudship-api-key: ${{ secrets.CLOUDSHIP_API_KEY }}
          image-name: '${{ github.repository_owner }}/station-production'
          image-tag: ${{ inputs.version }}
          push: 'true'
          registry-username: ${{ github.actor }}
          registry-password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/station \
            station=${{ github.repository_owner }}/station-production:${{ inputs.version }}
```

### Example 4: Customer Deployment (Bundle URL → Image)

For bundles distributed via direct URL:

```yaml
name: Deploy Bundle from URL

on:
  workflow_dispatch:
    inputs:
      bundle_url:
        description: 'Bundle URL from vendor'
        required: true
      version:
        description: 'Version tag'
        required: true

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
          image-name: '${{ github.repository_owner }}/station-production'
          image-tag: ${{ inputs.version }}
          push: 'true'
          registry-username: ${{ github.actor }}
          registry-password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/station \
            station=${{ github.repository_owner }}/station-production:${{ inputs.version }}
```

---

## Key Benefits

1. **Simplified Workflows**: No need to manage Station installation, Docker builds, or complex scripts
2. **Reusable**: Use across multiple repositories without duplicating code
3. **Consistent**: Same build process for all environments
4. **Flexible**: Support both local environments and remote bundle URLs
5. **Business Model Enabler**: 
   - Service providers create bundles
   - Customers build images from bundle URLs
   - Zero Station knowledge required for deployment

---

## Action Versioning

When using these actions, you can reference them by:

- **Branch**: `cloudshipai/station/.github/actions/setup-station@main` (latest)
- **Tag**: `cloudshipai/station/.github/actions/setup-station@v0.21.0` (specific version)
- **Commit**: `cloudshipai/station/.github/actions/setup-station@abc123` (pinned)

**Recommendation**: Use `@main` for development, pin to specific tags for production.

---

## Permissions Required

### For Bundle Builds:
```yaml
permissions:
  contents: write  # For creating releases
```

### For Image Builds with Registry Push:
```yaml
permissions:
  contents: read
  packages: write  # For GHCR push
```

---

## Troubleshooting

### Station CLI not found
- Ensure `setup-station` action runs before other Station commands
- Check that `version` input is a valid release tag

### Bundle creation fails
- Verify environment directory exists at `./environments/<environment>`
- Check that environment contains agents or MCP configs
- Ensure workspace-path points to correct directory

### Image push fails
- Verify `registry-username` and `registry-password` are provided
- Check repository has `packages: write` permission
- Ensure registry URL is correct (e.g., `ghcr.io`)

### Bundle URL download fails
- Verify URL is publicly accessible or credentials are provided
- Check bundle URL returns valid `.tar.gz` file
- Ensure network connectivity from GitHub Actions runner

---

## Contributing

To add new reusable actions:

1. Create directory: `.github/actions/<action-name>/`
2. Add `action.yml` with metadata and implementation
3. Update this README with documentation
4. Test in a workflow before committing

---

## Support

- **Documentation**: https://station.dev/docs
- **Issues**: https://github.com/cloudshipai/station/issues
- **Discussions**: https://github.com/cloudshipai/station/discussions
