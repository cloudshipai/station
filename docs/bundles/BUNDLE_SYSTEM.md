# Station Bundle System

The Station Bundle System provides portable, shareable packages that contain complete agent environments with MCP tools, configurations, and deployment templates. Bundles enable team collaboration, environment replication, and distribution of specialized agent workflows.

## Bundle Architecture

### Bundle Structure

A Station bundle is a compressed archive containing everything needed for a complete agent environment:

```
bundle.tar.gz
├── bundle.json              # Bundle metadata and manifest
├── template.json            # MCP server configuration
├── variables.yml            # Default environment variables
├── agents/                  # Agent definitions
│   ├── Security Scanner.prompt
│   ├── Code Reviewer.prompt
│   └── Deployment Agent.prompt
└── docs/                    # Optional documentation
    ├── README.md
    └── USAGE.md
```

### Bundle Manifest

**Bundle Metadata** (`bundle.json`):
```json
{
  "name": "DevOps Security Bundle",
  "description": "Comprehensive security analysis and monitoring agents for DevOps workflows",
  "version": "1.2.0",
  "author": "security-team",
  "license": "MIT",
  "created_at": "2025-08-27T15:30:00Z",
  "station_version": ">=0.9.0",
  "tags": ["security", "devops", "cicd", "terraform", "docker"],
  "category": "Security",
  "variables": {
    "PROJECT_ROOT": {
      "type": "string",
      "description": "Root directory for project analysis",
      "required": true,
      "default": "/workspace"
    },
    "LOG_LEVEL": {
      "type": "string", 
      "description": "Logging verbosity level",
      "enum": ["debug", "info", "warn", "error"],
      "default": "info"
    },
    "ENABLE_ADVANCED_SCANS": {
      "type": "boolean",
      "description": "Enable resource-intensive deep security scans",
      "default": false
    }
  },
  "mcp_servers": [
    {
      "name": "filesystem",
      "description": "File system operations for code analysis",
      "command": "npx -y @modelcontextprotocol/server-filesystem@latest"
    },
    {
      "name": "ship-security",
      "description": "300+ security tools via Ship CLI",
      "command": "ship mcp security --stdio",
      "required_binaries": ["ship"]
    }
  ],
  "agents": [
    {
      "name": "Security Scanner",
      "description": "Multi-layer security analysis for infrastructure, containers, and code",
      "model": "gpt-4o-mini",
      "max_steps": 12,
      "tags": ["security", "analysis", "cicd"],
      "capabilities": [
        "terraform security analysis",
        "container vulnerability scanning", 
        "secret detection",
        "code security review"
      ]
    },
    {
      "name": "Code Reviewer",
      "description": "Automated code quality and security review agent",
      "model": "gpt-4o-mini", 
      "max_steps": 8,
      "tags": ["code-review", "quality", "security"],
      "capabilities": [
        "code quality analysis",
        "security vulnerability detection",
        "best practices enforcement"
      ]
    }
  ],
  "tools_provided": [
    "__read_text_file", "__list_directory", "__directory_tree", "__search_files",
    "__checkov_scan_directory", "__trivy_scan_filesystem", "__gitleaks_dir",
    "__semgrep_scan", "__hadolint_dockerfile", "__tflint_directory"
  ],
  "requirements": {
    "disk_space": "500MB",
    "memory": "2GB", 
    "network": ["https://registry.npmjs.org", "https://api.openai.com"]
  },
  "examples": [
    {
      "name": "Terraform Security Scan",
      "command": "stn agent run \"Security Scanner\" \"Scan terraform directory for security issues\"",
      "description": "Comprehensive Terraform configuration security analysis"
    }
  ]
}
```

## Bundle Operations

### Creating Bundles

**From Existing Environment**:
```bash
# Create bundle from complete environment
stn bundle create security-bundle --from-env production \
  --name "Production Security Bundle" \
  --version "1.0.0" \
  --description "Production-ready security scanning agents"

# Create bundle with specific agents only
stn bundle create code-review-bundle --from-env development \
  --agents "Code Reviewer,Documentation Generator" \
  --exclude-tools "__debug_*"

# Create bundle with custom metadata
stn bundle create devops-bundle --from-env staging \
  --author "devops-team" \
  --license "Apache-2.0" \
  --tags "devops,automation,monitoring"
```

**Bundle Building Process**:
1. **Environment Analysis**: Scan environment for agents, MCP servers, and variables
2. **Dependency Resolution**: Identify required MCP servers and tools
3. **Agent Export**: Export all agent `.prompt` files with metadata
4. **Template Generation**: Create portable `template.json` with variable placeholders
5. **Manifest Creation**: Generate complete `bundle.json` with metadata
6. **Archive Creation**: Package everything into compressed `.tar.gz` file

### Installing Bundles

**Local Bundle Installation**:
```bash
# Install from local file
stn bundle install ./security-bundle.tar.gz security-env

# Install with custom variables
stn bundle install ./devops-bundle.tar.gz devops \
  --set PROJECT_ROOT="/opt/projects" \
  --set LOG_LEVEL="debug" \
  --set ENABLE_MONITORING="true"

# Install to existing environment (merge)
stn bundle install ./additional-tools.tar.gz production --merge
```

**Remote Bundle Installation**:
```bash
# Install from Station Registry
stn bundle install https://registry.station.dev/bundles/terraform-security.tar.gz terraform

# Install specific version
stn bundle install https://registry.station.dev/bundles/web-automation@1.5.0.tar.gz web-env

# Install with authentication
stn bundle install https://private-registry.company.com/bundles/internal-tools.tar.gz internal \
  --auth-token "$REGISTRY_TOKEN"
```

**Installation Process**:
1. **Bundle Download**: Download and extract bundle archive
2. **Validation**: Verify bundle manifest and Station version compatibility
3. **Environment Creation**: Create new environment or validate existing one
4. **Variable Prompting**: Interactive UI for missing variables (if required)
5. **MCP Server Installation**: Install and configure required MCP servers
6. **Agent Import**: Import agent `.prompt` files to environment
7. **Synchronization**: Connect MCP servers and validate tool availability
8. **Verification**: Test agent execution and tool access

### Bundle Management

**Listing and Inspection**:
```bash
# List installed bundles
stn bundle list

# Show bundle details
stn bundle info security-bundle

# List bundle contents without installing
stn bundle inspect ./devops-bundle.tar.gz

# Show bundle dependencies
stn bundle deps security-env

# Validate bundle integrity
stn bundle validate ./bundle.tar.gz
```

**Updating Bundles**:
```bash
# Check for bundle updates
stn bundle check-updates

# Update specific bundle
stn bundle update security-env --version 1.3.0

# Update all bundles in environment
stn bundle update-all production

# Show update changelog
stn bundle changelog security-bundle 1.2.0..1.3.0
```

**Removing Bundles**:
```bash
# Remove bundle (keeps environment)
stn bundle remove security-bundle

# Remove bundle and environment
stn bundle remove security-env --remove-environment

# Clean up unused bundle artifacts
stn bundle cleanup
```

## Bundle Registry Integration

### Station Registry

The Station Registry hosts publicly available bundles for the community:

**Registry Structure**:
```
https://registry.station.dev/
├── bundles/
│   ├── security/
│   │   ├── devops-security-bundle.tar.gz
│   │   ├── web-security-bundle.tar.gz
│   │   └── cloud-security-bundle.tar.gz
│   ├── development/
│   │   ├── full-stack-dev-bundle.tar.gz
│   │   └── api-development-bundle.tar.gz
│   └── monitoring/
│       ├── prometheus-bundle.tar.gz
│       └── observability-bundle.tar.gz
├── api/
│   ├── bundles/           # Bundle metadata API
│   ├── search/            # Bundle search API
│   └── versions/          # Version management API
└── index.json            # Registry index
```

**Registry API**:
```bash
# Search bundles
curl "https://registry.station.dev/api/search?q=security&category=devops"

# Get bundle metadata
curl "https://registry.station.dev/api/bundles/security/devops-security-bundle"

# List bundle versions
curl "https://registry.station.dev/api/versions/devops-security-bundle"

# Download bundle
curl -O "https://registry.station.dev/bundles/security/devops-security-bundle.tar.gz"
```

### Publishing Bundles

**Publishing to Station Registry**:
```bash
# Authenticate with registry
stn registry login --username your-username --token $REGISTRY_TOKEN

# Publish bundle
stn bundle publish ./security-bundle.tar.gz \
  --category security \
  --visibility public \
  --readme ./README.md

# Publish specific version
stn bundle publish ./security-bundle-v2.tar.gz \
  --version 2.0.0 \
  --changelog "Added container scanning capabilities"
```

**Private Registry Setup**:
```bash
# Configure private registry
stn registry add company-registry https://bundles.company.com \
  --auth-token $COMPANY_TOKEN

# Publish to private registry
stn bundle publish ./internal-tools.tar.gz \
  --registry company-registry \
  --visibility private
```

## Bundle Development Workflow

### Development Lifecycle

**1. Environment Setup**:
```bash
# Create development environment
stn env create bundle-dev

# Install required MCP servers
cat > ~/.config/station/environments/bundle-dev/template.json << EOF
{
  "name": "bundle-development",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    }
  }
}
EOF

# Sync environment
stn sync bundle-dev
```

**2. Agent Development**:
```bash
# Create specialized agents
stn agent create "Bundle Test Agent" \
  --description "Agent for testing bundle functionality" \
  --env bundle-dev

# Test agents with realistic scenarios
stn agent run "Bundle Test Agent" "Test filesystem access and tool execution" --env bundle-dev
```

**3. Bundle Creation and Testing**:
```bash
# Create bundle from development environment
stn bundle create test-bundle --from-env bundle-dev

# Test bundle in clean environment
stn env create test-install
stn bundle install ./test-bundle.tar.gz test-install

# Validate bundle works correctly
stn agent run "Bundle Test Agent" "Verify bundle installation" --env test-install
```

**4. Documentation and Publishing**:
```bash
# Add bundle documentation
mkdir docs
cat > docs/README.md << EOF
# Test Bundle

This bundle provides agents for testing bundle functionality.

## Installation
stn bundle install test-bundle.tar.gz my-env

## Usage
stn agent run "Bundle Test Agent" "Your task description" --env my-env
EOF

# Publish bundle
stn bundle publish ./test-bundle.tar.gz --category testing
```

### Bundle Testing

**Automated Bundle Testing**:
```yaml
# .github/workflows/bundle-test.yml
name: Bundle Testing
on: [push, pull_request]

jobs:
  test-bundle:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Install Station
        run: curl -sSL https://install.station.dev | bash
        
      - name: Create Test Bundle
        run: stn bundle create test-bundle --from-env development
        
      - name: Test Bundle Installation
        run: |
          stn bundle install ./test-bundle.tar.gz test-env
          stn agent list --env test-env
          
      - name: Test Agent Execution
        run: |
          stn agent run "Test Agent" "Validate bundle functionality" --env test-env
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          
      - name: Validate Bundle
        run: stn bundle validate ./test-bundle.tar.gz
```

**Bundle Quality Checks**:
```bash
# Validate bundle structure
stn bundle validate ./bundle.tar.gz --strict

# Check bundle size and dependencies
stn bundle analyze ./bundle.tar.gz --show-dependencies

# Security scan bundle contents
stn bundle scan ./bundle.tar.gz --security-check

# Performance test bundle installation
stn bundle benchmark ./bundle.tar.gz --iterations 5
```

## Advanced Bundle Features

### Conditional Components

**Environment-Specific Components**:
```json
{
  "components": [
    {
      "name": "development-tools",
      "condition": "{{ eq .ENVIRONMENT \"development\" }}",
      "agents": ["Debug Agent", "Development Helper"],
      "mcpServers": ["debug-server"]
    },
    {
      "name": "production-monitoring", 
      "condition": "{{ eq .ENVIRONMENT \"production\" }}",
      "agents": ["Monitoring Agent", "Alert Handler"],
      "mcpServers": ["prometheus-server", "alertmanager-server"]
    }
  ]
}
```

### Bundle Inheritance

**Base Bundle with Extensions**:
```json
{
  "name": "security-scanner-extended",
  "extends": "security-scanner-base@1.0.0",
  "additional_agents": [
    "Advanced Threat Detector",
    "Compliance Auditor"
  ],
  "additional_tools": [
    "__advanced_threat_scan",
    "__compliance_check"
  ],
  "overrides": {
    "variables": {
      "LOG_LEVEL": "debug"
    }
  }
}
```

### Multi-Platform Bundles

**Platform-Specific Configurations**:
```json
{
  "platforms": {
    "linux": {
      "mcpServers": {
        "security-tools": {
          "command": "ship",
          "args": ["mcp", "security", "--stdio"]
        }
      }
    },
    "macos": {
      "mcpServers": {
        "security-tools": {
          "command": "/opt/homebrew/bin/ship",
          "args": ["mcp", "security", "--stdio"]
        }
      }
    },
    "windows": {
      "mcpServers": {
        "security-tools": {
          "command": "ship.exe",
          "args": ["mcp", "security", "--stdio"]
        }
      }
    }
  }
}
```

## Bundle Security and Compliance

### Security Scanning

Station automatically scans bundles for security issues:

```bash
# Security scan during bundle creation
stn bundle create secure-bundle --from-env production --security-scan

# Scan existing bundle
stn bundle security-scan ./bundle.tar.gz

# Show security report
stn bundle security-report security-env
```

**Security Checks Include**:
- Hardcoded secrets detection
- Malicious command detection  
- Unsafe file permissions
- Dependency vulnerability scanning
- Digital signature verification

### Bundle Signing

**Digital Signature Support**:
```bash
# Generate signing key
stn bundle generate-key --name company-signing-key

# Sign bundle during creation
stn bundle create signed-bundle --from-env production --sign

# Verify bundle signature
stn bundle verify ./signed-bundle.tar.gz

# Install only signed bundles
stn config set bundle.require_signature true
```

### Compliance and Auditing

**Compliance Metadata**:
```json
{
  "compliance": {
    "frameworks": ["SOC2", "PCI-DSS", "GDPR"],
    "audit_trail": {
      "created_by": "user@company.com",
      "approved_by": "security@company.com",
      "scan_results": "clean",
      "signature_valid": true
    },
    "data_classification": "internal",
    "retention_policy": "7_years"
  }
}
```

This comprehensive bundle system enables Station users to create, share, and deploy complete agent environments while maintaining security, compliance, and ease of use across different deployment scenarios.