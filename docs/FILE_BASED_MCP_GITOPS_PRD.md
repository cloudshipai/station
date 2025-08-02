# File-Based MCP Config with GitOps Support - PRD

**Version:** 1.0  
**Date:** August 2, 2025  
**Epic:** File-Based MCP Configuration System with GitOps Integration

## Executive Summary

Transform Station's MCP configuration from SQLite-encrypted storage to a file-based, GitOps-friendly system using Go templates. This enables version control of MCP configurations while keeping secrets separate and secure.

## Problem Statement

### Current Issues
1. **Not GitOps-friendly**: MCP configs stored in encrypted SQLite, can't be version controlled
2. **Secret management complexity**: Encryption keys and config mixed together
3. **No configuration sharing**: Teams can't share agent configs + templates easily
4. **Manual secret input**: No systematic way to collect and manage template variables
5. **Environment coupling**: Config and secrets tightly coupled, hard to promote across environments

### Business Impact
- **DevOps/Infrastructure teams** need GitOps workflows for MCP configurations  
- **Team collaboration** limited by inability to share agent templates
- **Environment management** complexity increases with scale
- **Security concerns** with mixed config/secret storage

## Solution Overview

### Core Concept
Replace SQLite MCP configs with a file-based system using:
- **Templates**: Go template-based MCP configs (version controlled)
- **Variables**: Environment files for secrets/variables (not in git) 
- **Dynamic loading**: Template rendering at runtime
- **GitOps workflow**: Templates committed, secrets managed separately

### File Structure Design
```
~/.config/station/
├── environments/
│   ├── default/
│   │   ├── variables.env           # Environment secrets (not in git)
│   │   ├── mcpconfig.json         # Main MCP template
│   │   ├── github-tools.json      # Additional MCP template
│   │   └── aws-tools.json         # Additional MCP template
│   ├── staging/
│   │   ├── variables.env          # Staging-specific secrets
│   │   ├── mcpconfig.json         # Same template, different vars
│   │   ├── github-tools.json
│   │   └── datadog-tools.json     # Staging-specific template
│   └── prod/
│       ├── variables.env          # Production secrets
│       ├── mcpconfig.json
│       ├── github-tools.json
│       └── newrelic-tools.json    # Prod-specific template
└── secrets/                       # Alternative secrets location
    ├── default.env
    ├── staging.env
    └── prod.env
```

## Technical Architecture

### 1. Template System Design

#### Template Format (Go Templates)
```json
{
  "mcpServers": {
    "{{.ServerName}}": {
      "command": "{{.Command}}",
      "args": {{.Args | toJSON}},
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{.GithubToken}}",
        "AWS_ACCESS_KEY_ID": "{{.AwsAccessKey}}",
        "AWS_SECRET_ACCESS_KEY": "{{.AwsSecretKey}}"
      }
    }
  }
}
```

#### Variable File Format (.env)
```bash
# GitHub Integration
ServerName=github
Command=npx
Args=["@modelcontextprotocol/server-github"]
GithubToken=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxx

# AWS Integration  
AwsAccessKey=AKIAIOSFODNN7EXAMPLE
AwsSecretKey=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### 2. Database Schema Changes

#### Environment Table Update
```sql
-- Add paths for file-based configs
ALTER TABLE environments ADD COLUMN mcp_config_path TEXT;
ALTER TABLE environments ADD COLUMN variables_path TEXT;
```

#### New File Config Tracking
```sql
-- Track loaded file-based configs
CREATE TABLE file_mcp_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment_id INTEGER NOT NULL,
    config_name TEXT NOT NULL,
    template_path TEXT NOT NULL,
    variables_path TEXT NOT NULL,
    last_loaded_at TIMESTAMP,
    template_hash TEXT, -- For change detection
    variables_hash TEXT,
    FOREIGN KEY (environment_id) REFERENCES environments(id),
    UNIQUE(environment_id, config_name)
);
```

### 3. Core Components

#### Config Manager Interface
```go
type FileConfigManager interface {
    // Template operations
    LoadTemplate(envID int64, configName string) (*MCPTemplate, error)
    RenderTemplate(template *MCPTemplate, variables map[string]interface{}) (*MCPConfigData, error)
    ValidateTemplate(templatePath string) error
    
    // Variable operations  
    LoadVariables(envID int64) (map[string]interface{}, error)
    SaveVariables(envID int64, variables map[string]interface{}) error
    PromptForMissingVariables(template *MCPTemplate, existing map[string]interface{}) (map[string]interface{}, error)
    
    // Discovery
    DiscoverTemplates(envPath string) ([]string, error)
    ExtractTemplateVariables(templatePath string) ([]TemplateVariable, error)
}
```

#### Template Variable Definition
```go
type TemplateVariable struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Required    bool   `json:"required"`
    Default     string `json:"default,omitempty"`
    Type        string `json:"type"` // string, number, boolean, array
    Secret      bool   `json:"secret"` // Don't log/display value
}
```

#### Filesystem Abstraction (Afero)
```go
type ConfigFileSystem interface {
    afero.Fs
    // Extended methods for config-specific operations
    EnsureConfigDir(envName string) error
    GetConfigPath(envName, configName string) string
    GetVariablesPath(envName string) string
}
```

### 4. Loading Workflow

```mermaid
graph TD
    A[Station Startup] --> B[Load Environments]
    B --> C[For Each Environment]
    C --> D[Scan Config Directory]
    D --> E[Find Template Files]
    E --> F[Extract Template Variables]
    F --> G[Load Existing Variables]
    G --> H{Missing Variables?}
    H -->|Yes| I[Prompt User for Values]
    H -->|No| J[Render Templates]
    I --> K[Save New Variables]
    K --> J
    J --> L[Initialize MCP Servers]
    L --> M[Register Tools]
```

## Implementation Plan

### Phase 1: Core Infrastructure (2 weeks)
- **Filesystem abstraction layer** with afero
- **Template engine** with Go templates
- **Variable management** with .env file support
- **Config directory structure** creation
- **Basic CLI flags** (--mcp-conf-dir, --vars-dir)

### Phase 2: Template System (2 weeks)  
- **Template discovery** and validation
- **Variable extraction** from templates
- **Interactive variable prompting** 
- **Template rendering** engine
- **Error handling** and validation

### Phase 3: Database Integration (1 week)
- **Schema migration** for file paths
- **Hybrid loading** (file + database fallback)
- **Config change detection** with hashing
- **Environment path management**

### Phase 4: GitOps Features (1 week)
- **Template sharing** mechanisms
- **Configuration validation** 
- **Environment promotion** workflows
- **Template bundling** for agent configs

### Phase 5: CLI Integration (1 week)
- **MCP config commands** (stn mcp config list/create/validate)
- **Variable management** commands
- **Template operations** commands
- **Migration tools** from SQLite to files

## API Design

### CLI Commands
```bash
# Config management
stn mcp config list                          # List templates per environment
stn mcp config create <name> --env <env>     # Create new template
stn mcp config validate <path>               # Validate template syntax
stn mcp config render <name> --env <env>     # Preview rendered config

# Variable management  
stn mcp vars list --env <env>                # List variables
stn mcp vars set <key>=<value> --env <env>   # Set variable
stn mcp vars edit --env <env>                # Interactive variable editing
stn mcp vars import --env <env> <file>       # Import from file

# Template operations
stn mcp template init <name>                 # Create template skeleton
stn mcp template share <name> --env <env>    # Export template bundle
stn mcp template import <bundle>             # Import shared template

# Environment operations
stn env config --env <env>                   # Set config/vars paths
stn env promote <source> <target>            # Promote config between envs

# Global options
--mcp-conf-dir <path>                        # Override config directory
--vars-dir <path>                            # Override variables directory
```

### Configuration API Endpoints
```
GET    /api/v1/environments/{id}/mcp/configs    # List templates
POST   /api/v1/environments/{id}/mcp/configs    # Create template
GET    /api/v1/environments/{id}/mcp/configs/{name} # Get template
PUT    /api/v1/environments/{id}/mcp/configs/{name} # Update template
DELETE /api/v1/environments/{id}/mcp/configs/{name} # Delete template

GET    /api/v1/environments/{id}/mcp/variables   # Get variables
PUT    /api/v1/environments/{id}/mcp/variables   # Update variables
POST   /api/v1/environments/{id}/mcp/render/{name} # Render template
```

## Testing Strategy

### Unit Tests
- **Template parsing** and validation
- **Variable extraction** from templates  
- **Rendering engine** with various inputs
- **File system operations** with afero
- **Error handling** edge cases

### Integration Tests  
- **End-to-end template workflow** (create → variables → render → load)
- **Multi-environment scenarios** with shared templates
- **Migration from SQLite** to file-based configs
- **CLI command integration** 
- **API endpoint functionality**

### E2E Tests
```bash
# Test scenarios
./testing/file-config-e2e.sh
├── Template creation and validation
├── Variable prompting workflow
├── Multi-environment template sharing
├── GitOps workflow simulation
├── CLI flags and overrides
└── Migration from existing configs
```

## Migration Strategy

### Backward Compatibility
1. **Hybrid loading**: Check files first, fallback to SQLite
2. **Gradual migration**: Export SQLite configs to file templates
3. **Flag-based control**: --use-file-configs to enable new system
4. **Deprecation timeline**: 2 releases to fully migrate

### Migration Commands
```bash
stn mcp migrate export --env <env>           # Export SQLite to files
stn mcp migrate import --env <env> <dir>     # Import files to SQLite
stn mcp migrate validate                     # Check migration readiness
```

## Security Considerations

### Secrets Management
- **File permissions**: 600 for .env files, 700 for directories
- **Git exclusion**: Automatic .gitignore for secrets directories
- **Variable validation**: Type checking and sanitization
- **Secret detection**: Warn about potential secrets in templates

### Template Security
- **Template validation**: Prevent code injection in templates
- **Path traversal protection**: Restrict file access to config directories
- **Variable sanitization**: Escape special characters in values
- **Audit logging**: Track template changes and variable access

## Risk Assessment & Mitigation

### High Risk
- **Data loss during migration**: Comprehensive backup strategy required
- **Secret exposure**: Multiple layers of protection (permissions, .gitignore, validation)

### Medium Risk  
- **Template complexity**: Start with simple templates, add features gradually
- **Performance impact**: Cache rendered configs, optimize file I/O

### Low Risk
- **User adoption**: Provide clear migration path and documentation
- **Configuration drift**: Version control and validation prevent issues

## Success Metrics

### Technical Metrics
- **Migration completion**: 100% of existing configs migrated successfully
- **Template coverage**: Support for all current MCP server types
- **Performance**: <200ms template rendering time
- **Reliability**: 99.9% successful config loading

### User Experience Metrics
- **Setup time**: <5 minutes for new environment with templates
- **Error reduction**: 50% fewer config-related errors
- **Team adoption**: GitOps workflow used by 80% of DevOps teams

## Timeline & Resources

### Total Effort: 7 weeks (1 developer)

#### Detailed Breakdown
- **Week 1-2**: Core infrastructure (filesystem, templates, variables)
- **Week 3-4**: Template system (discovery, extraction, rendering)
- **Week 5**: Database integration and migration
- **Week 6**: GitOps features and CLI integration  
- **Week 7**: Testing, documentation, and polish

### Dependencies
- **Go 1.21+**: For template engine and afero filesystem
- **Afero library**: Filesystem abstraction
- **Cobra/Viper**: CLI framework extensions
- **Migration tools**: Database schema updates

## Future Enhancements

### Template Marketplace
- **Community templates**: Share MCP configs across teams
- **Template versioning**: Semantic versioning for templates
- **Dependency management**: Template inheritance and composition

### Advanced Features
- **Remote config sources**: S3, Git repositories, HTTP endpoints
- **Config validation**: JSON Schema validation for templates
- **Automated testing**: Template testing framework
- **Environment synchronization**: Sync configs across environments

## Conclusion

This file-based MCP config system transforms Station into a GitOps-native platform suitable for DevOps and infrastructure teams. The separation of templates and secrets enables secure collaboration while maintaining flexibility for environment-specific configurations.

The phased implementation approach ensures minimal disruption while providing a clear migration path. Success depends on robust testing, comprehensive documentation, and seamless integration with existing workflows.

---
*Generated for Station development - File-based MCP configuration with GitOps support*