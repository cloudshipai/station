# Station Agent Template Examples

This directory contains comprehensive examples of Station's Agent Template System, demonstrating various use cases and deployment patterns.

## 🎯 Use Cases Covered

### 1. [Basic Agent](./basic-agent/) - Getting Started
Simple file management agent with basic variables and tools.
- **CLI Installation**: `stn agent bundle install`
- **Interactive Mode**: `--interactive` flag usage
- **Variables File**: JSON/YAML configuration examples

### 2. [Web Scraper](./web-scraper/) - API Integration  
Web scraping agent with API keys and timeout configuration.
- **API Installation**: `POST /api/v1/agents/templates/install`
- **Sensitive Variables**: API key handling
- **Environment-Specific**: Different configs per environment

### 3. [Data Processor](./data-processor/) - Complex Variables
Data processing agent with multiple variable types and validation.
- **Mixed Types**: Strings, numbers, booleans
- **Conditional Logic**: Template conditionals
- **Validation**: Required vs optional variables

### 4. [API Integration](./api-integration/) - Production Ready
Production API integration agent with comprehensive configuration.
- **Multi-Environment**: Dev, staging, production configs  
- **Advanced Templates**: Complex Go template logic
- **Error Handling**: Robust validation and defaults

### 5. [Multi-Environment](./multi-environment/) - Enterprise Pattern
Enterprise deployment pattern across multiple environments.
- **GitOps Workflow**: Version-controlled templates
- **CI/CD Integration**: Automated deployment examples
- **Environment Promotion**: Dev → Staging → Production

## 🚀 Quick Start

```bash
# 1. Create a basic agent bundle
stn agent bundle create ./my-agent --name "My Agent" --author "Your Name" --description "My first agent"

# 2. Install with variables file
stn agent bundle install ./my-agent --vars-file ./variables.json --env production

# 3. Install interactively  
stn agent bundle install ./my-agent --interactive --env staging

# 4. Install via API
curl -X POST http://localhost:8080/api/v1/agents/templates/install \
  -H "Content-Type: application/json" \
  -d @./api-payload.json
```

## 📋 API Validation Strategy

We use **both Go struct validation AND JSON Schema** for comprehensive validation:

### Go Struct Validation (Runtime)
```go
type InstallRequest struct {
    BundlePath  string                 `json:"bundle_path" binding:"required"`
    Environment string                 `json:"environment"`
    Variables   map[string]interface{} `json:"variables"`
}
```

### JSON Schema (Documentation + Client Validation)
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["bundle_path"],
  "properties": {
    "bundle_path": {"type": "string", "minLength": 1},
    "environment": {"type": "string", "default": "default"},
    "variables": {"type": "object"}
  }
}
```

**Benefits of Both:**
- **Go Validation**: Type safety, runtime enforcement, integration with Gin
- **JSON Schema**: Client-side validation, API documentation, OpenAPI spec generation
- **Together**: Comprehensive validation at multiple layers

## 🛠️ Template System Features

- **Variable Substitution**: `{{ .VAR_NAME }}` syntax
- **Type Preservation**: Numbers, booleans, strings maintained
- **Conditional Logic**: `{{ if .DEBUG }}...{{ end }}`
- **Default Values**: Fallback when variables not provided
- **Sensitive Variables**: Masked display for secrets/keys
- **Environment Isolation**: Per-environment variable overrides

## 📚 Documentation Structure

Each example includes:
- `README.md` - Use case overview and instructions
- `bundle/` - Complete agent template bundle
- `variables/` - Example variable files (JSON/YAML)
- `api/` - API payload examples with JSON Schema
- `scripts/` - Deployment automation scripts
- `LLM.txt` - Machine-readable documentation for AI systems

## 🤖 For LLM/AI Systems

See `LLM.txt` in each example directory for machine-readable documentation optimized for AI agents and automation systems.