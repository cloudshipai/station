# Template Bundle System - Product Requirements Document

## Overview

The Template Bundle System enables users to discover, install, and share MCP template configurations across multiple registries (official, S3, GitHub, local). This system separates public template bundles from private environment variables, enabling teams to quickly adopt proven configurations while maintaining security.

## Problem Statement

### Current Pain Points
- **Limited Template Discovery**: Users can't easily discover new MCP configurations
- **No Template Sharing**: No mechanism to share proven configurations across teams
- **Tight Coupling**: Templates and secrets are mixed in single configuration files
- **Manual Setup**: Users must manually create complex MCP configurations
- **No Versioning**: No way to version or update template configurations

### User Stories

**As a DevOps Engineer**, I want to quickly install and test AWS tooling templates so I can evaluate them before customizing for my team.

**As a Team Lead**, I want to publish internal template bundles to a private registry so my team can standardize on approved configurations.

**As a Security Engineer**, I want templates and secrets separated so templates can be shared publicly while secrets remain private.

**As a Platform Engineer**, I want to deploy Station with pre-configured template bundles so new environments come with approved tooling.

## Solution Architecture

### High-Level Design

```
Template Bundle Ecosystem:
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Official Hosted │    │ Private S3      │    │ GitHub/HTTP     │
│ Registry        │    │ Registry        │    │ Registry        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │ Station Client  │
                    │ Bundle Manager  │
                    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │ Local Bundle    │
                    │ Cache           │
                    └─────────────────┘
```

### Core Components

#### 1. Multi-Source Registry System
- **Abstract Registry Interface**: Supports HTTP/HTTPS, S3, and local file system registries
- **Afero Integration**: File system abstraction for consistent handling across sources
- **Registry Configuration**: YAML-based registry management in Station config

#### 2. Bundle Structure
```
bundle-name/
├── manifest.json              # Bundle metadata & dependencies
├── template.json              # MCP template configuration
├── variables.schema.json      # Required variables schema
├── README.md                  # Documentation
├── examples/                  # Example configurations
│   ├── development.vars.yml   # Dev variable examples
│   └── production.vars.yml    # Prod variable examples
└── tests/                     # Validation tests (optional)
    └── template.test.json
```

#### 3. Bundle Lifecycle Management
- **Creation**: `stn template create` - scaffolds bundle structure
- **Validation**: `stn template validate` - validates bundle integrity
- **Packaging**: `stn template package` - creates distributable zip
- **Publishing**: `stn template publish` - uploads to registry
- **Installation**: `stn template install` - downloads and caches locally

### Technical Implementation

#### Registry Abstraction Layer
```go
type BundleRegistry interface {
    List() ([]BundleManifest, error)
    Download(bundleName, version string) ([]byte, error)
    Upload(bundlePath string) error  // Optional: for writeable registries
}

type BundleManager struct {
    registries map[string]BundleRegistry
    cacheFS    afero.Fs
    httpClient *http.Client
}
```

#### Enhanced Variable Resolution
Current priority: `template.vars.yml` → `environment/variables.yml` → `os.Getenv()` → prompts

Enhanced priority: `bundle.schema.defaults` → `template.vars.yml` → `environment/variables.yml` → `os.Getenv()` → prompts

#### Integration with Existing System
- **Enhanced `stn mcp sync`**: Discovers both local templates and installed bundles
- **Backward Compatible**: Existing local templates continue to work unchanged
- **Auto-Save Variables**: Interactive prompts save responses to `environments/{env}/variables.yml`

## User Experience

### New Command Structure

```bash
# Discovery & Installation
stn template list                           # List all available bundles
stn template list --registry company       # List from specific registry
stn template install aws-powertools        # Install from default registry
stn template install aws-powertools@1.2.0  # Install specific version
stn template install company/aws-custom    # Install from specific registry

# Bundle Creation
stn template create my-bundle               # Create bundle structure
stn template validate my-bundle            # Validate bundle
stn template package my-bundle             # Create zip file
stn template publish my-bundle --registry company

# Management
stn template update aws-powertools         # Update to latest
stn template remove aws-powertools         # Remove from cache
stn template registries                    # List configured registries
stn template registry add name url         # Add new registry
```

### Example User Workflow

#### Developer Testing New Bundle
```bash
# Discover available bundles
stn template list | grep aws

# Install AWS powertools bundle
stn template install aws-powertools

# Sync with development environment (prompts for missing variables)
stn mcp sync development
# → Prompts: AWS_ACCESS_KEY_ID, AWS_REGION
# → Saves responses to ~/.config/station/environments/development/variables.yml

# Bundle tools are now available to agents
stn agent create aws-analyzer --tools aws-powertools
```

#### Team Lead Publishing Internal Bundle
```bash
# Create new bundle for team
stn template create company-security-tools

# Edit manifest.json, template.json, variables.schema.json
# Add documentation and examples

# Validate bundle structure
stn template validate company-security-tools

# Package and publish to private S3 registry
stn template package company-security-tools
stn template publish company-security-tools --registry company-private

# Team members can now install
# stn template install company-private/company-security-tools
```

### Configuration Example

```yaml
# ~/.config/station/config.yaml (enhanced)
template_registries:
  - name: "official"
    type: "https"
    url: "https://templates.station.ai"
    default: true
  - name: "company"
    type: "s3"
    bucket: "acme-corp-station-templates"
    region: "us-west-2"
    prefix: "bundles/"
    access_key_id: "${AWS_ACCESS_KEY_ID}"
    secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
  - name: "github-community"
    type: "https"
    url: "https://raw.githubusercontent.com/station-community/bundles/main"
```

## GitOps & Multi-Environment Support

### Development Workflow
```bash
# Local development with interactive prompts
stn template install aws-powertools
stn mcp sync development
# → Interactive prompts save to environments/development/variables.yml
```

### Production Deployment
```dockerfile
FROM station:base

# Install required bundles
COPY deployment/installed-bundles.yml /tmp/
RUN stn template install --from-file /tmp/installed-bundles.yml

# Copy environment-specific encrypted variables
COPY environments/ /station/environments/

# Decrypt secrets using SOPS/sealed-secrets/etc
RUN decrypt-secrets.sh

# Sync all configurations (no prompts, uses env vars + files)
RUN stn mcp sync production

CMD ["stn", "serve"]
```

### CI/CD Integration
```yaml
deploy_production:
  script:
    - export AWS_ACCESS_KEY_ID=$PROD_AWS_KEY
    - export AWS_REGION=us-west-2
    - docker build --build-arg ENVIRONMENT=production .
    - docker run station:prod stn mcp sync production --validate-only
    - kubectl apply -f deployment.yaml
```

## Bundle Registry Specification

### Bundle Manifest Schema
```json
{
  "name": "aws-powertools",
  "version": "1.2.0",
  "description": "AWS CLI, S3, and CloudWatch monitoring tools",
  "author": "Station Team <team@station.ai>",
  "license": "MIT",
  "repository": "https://github.com/station-ai/bundles/aws-powertools",
  "station_version": ">=0.1.0",
  "created_at": "2025-08-07T15:30:00Z",
  "updated_at": "2025-08-07T15:30:00Z",
  "tags": ["aws", "cloud", "monitoring", "infrastructure"],
  "required_variables": {
    "AWS_ACCESS_KEY_ID": {
      "type": "string",
      "description": "AWS Access Key ID for API authentication",
      "secret": true,
      "required": true,
      "validation": "^[A-Z0-9]{20}$"
    },
    "AWS_SECRET_ACCESS_KEY": {
      "type": "string",
      "description": "AWS Secret Access Key",
      "secret": true,
      "required": true
    },
    "AWS_REGION": {
      "type": "string",
      "description": "AWS region for operations",
      "default": "us-east-1",
      "enum": ["us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"],
      "required": false
    },
    "MONITORING_ENABLED": {
      "type": "boolean",
      "description": "Enable CloudWatch monitoring",
      "default": true,
      "required": false
    }
  },
  "dependencies": {
    "aws-cli": ">=2.0.0",
    "docker": ">=20.0.0"
  },
  "tools_count": 8,
  "download_count": 1250,
  "checksum": "sha256:a1b2c3d4...",
  "size_bytes": 2048
}
```

### Registry API Endpoints

#### HTTP/HTTPS Registry
```
GET  /bundles                          # List all bundles
GET  /bundles?tag=aws&search=tools     # Search bundles
GET  /bundles/{name}                   # Get bundle metadata
GET  /bundles/{name}/versions          # List versions
GET  /bundles/{name}/{version}.zip     # Download bundle
POST /bundles                          # Upload bundle (authenticated)
```

#### S3 Registry Structure
```
s3://bucket-name/bundles/
├── index.json                         # Bundle registry index
├── aws-powertools/
│   ├── 1.0.0/
│   │   ├── aws-powertools.zip
│   │   └── manifest.json
│   ├── 1.1.0/
│   └── 1.2.0/
└── github-automation/
    └── 1.0.0/
```

## Security Considerations

### Bundle Security
- **Checksum Validation**: SHA256 checksums in manifests prevent tampering
- **Schema Validation**: JSON schema validation for all bundle components
- **Signature Support**: Future: GPG signature verification for trusted publishers

### Variable Security
- **Secret Separation**: Templates public, variables private
- **Encryption at Rest**: Support for SOPS, sealed-secrets, vault integration
- **Environment Isolation**: Environment-specific variable files
- **No Secret Logging**: Sensitive variables marked and handled appropriately

### Registry Security
- **Access Control**: S3 IAM policies, HTTPS authentication
- **Private Registries**: Team-specific registries with access controls
- **Audit Logging**: Bundle download and usage tracking

## Implementation Phases

### Phase 1: Foundation (Week 1)
- [ ] Bundle structure definition and validation
- [ ] Local bundle creation tools (`create`, `validate`, `package`)
- [ ] Basic HTTP registry support
- [ ] Integration with existing `stn mcp sync`

### Phase 2: Multi-Registry (Week 2)
- [ ] Afero abstraction layer implementation
- [ ] S3 registry support with AWS SDK
- [ ] Registry configuration management
- [ ] Bundle installation and caching

### Phase 3: Publishing & Management (Week 3)
- [ ] Bundle publishing workflow
- [ ] Update and version management
- [ ] Enhanced CLI commands
- [ ] Documentation and examples

### Phase 4: Production Features (Week 4)
- [ ] GitOps deployment patterns
- [ ] Bundle signing and verification
- [ ] Registry mirroring and caching
- [ ] Monitoring and analytics

## Testing Strategy

### Unit Tests
- Bundle creation and validation logic
- Registry interface implementations
- Variable resolution hierarchy
- Afero file system abstractions

### Integration Tests
- End-to-end bundle workflow (create → validate → package → install)
- Multi-registry discovery and installation
- Template rendering with bundle variables
- MCP sync integration

### System Tests
- Docker-based GitOps deployment scenarios
- Multi-environment variable resolution
- Registry failover and caching
- Performance with large bundle catalogs

## Metrics & Success Criteria

### User Adoption
- Number of bundles installed per month
- Bundle creation and publishing rates
- Registry usage distribution (official vs private)

### Developer Experience
- Time from bundle discovery to working agent (target: <5 minutes)
- Template installation success rate (target: >95%)
- User satisfaction scores for bundle system

### System Performance
- Bundle download and installation time (target: <30 seconds)
- Registry response times (target: <2 seconds)
- Cache hit rates (target: >80%)

## Future Enhancements

### Advanced Features
- **Bundle Dependencies**: Bundles that depend on other bundles
- **Template Composition**: Combining multiple bundles into environments
- **A/B Testing**: Multiple versions of bundles in different environments
- **Analytics Dashboard**: Usage metrics and bundle performance

### Ecosystem Growth
- **Community Registry**: Open registry for community-contributed bundles
- **Bundle Marketplace**: Rated and reviewed bundle ecosystem
- **Enterprise Features**: LDAP/SAML authentication, audit logging
- **CI/CD Integrations**: Native GitHub Actions, GitLab CI, Jenkins plugins

---

## Appendix

### Current State Analysis
- **Existing System**: File-based templates in `~/.config/station/environments/`
- **Variable Resolution**: Template-specific → Environment → System → Interactive
- **MCP Integration**: `stn mcp sync` handles template rendering and tool registration
- **Database**: SQLite stores environment, agent, and tool metadata

### Migration Strategy
- **Backward Compatibility**: Existing local templates continue working
- **Gradual Adoption**: Users can mix local templates and bundles
- **No Breaking Changes**: All current functionality preserved
- **Optional Migration**: Tools to convert local templates to bundles