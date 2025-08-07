# Basic Agent Template Example

A simple file management agent template demonstrating the fundamentals of Station's Agent Template System.

## 📋 Overview

This example shows:
- Basic variable substitution
- File operations with MCP tools
- Simple deployment workflows
- CLI and API installation methods

## 🎯 Use Case

File management agent that can:
- List directory contents
- Search for files
- Read text files
- Basic file operations

Perfect for getting started with agent templates.

## 📁 Bundle Structure

```
bundle/
├── manifest.json          # Bundle metadata and dependencies
├── agent.json            # Agent configuration template
├── variables.schema.json # Variable definitions and validation
└── README.md            # Bundle documentation
```

## 🔧 Variables

| Variable | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `CLIENT_NAME` | string | Yes | - | Name of the client this agent serves |
| `WORKSPACE_PATH` | string | No | `/home/user` | Default workspace directory |
| `MAX_FILES` | number | No | 1000 | Maximum files to process |
| `VERBOSE_MODE` | boolean | No | false | Enable detailed logging |

## 🚀 Installation Methods

### 1. CLI with Variables File

```bash
# Using JSON variables
stn agent bundle install ./bundle --vars-file ./variables/production.json --env production

# Using YAML variables  
stn agent bundle install ./bundle --vars-file ./variables/staging.yml --env staging
```

### 2. CLI Interactive Mode

```bash
stn agent bundle install ./bundle --interactive --env development
```

### 3. API Installation

```bash
curl -X POST http://localhost:8080/api/v1/agents/templates/install \
  -H "Content-Type: application/json" \
  -d @./api/install-request.json
```

## 📝 Example Outputs

**CLI Success:**
```
✅ Agent bundle installed successfully!
🤖 Agent ID: 1
📝 Agent Name: Acme Corp File Manager
🌍 Environment: production
🔧 Tools Installed: 3
📦 MCP Bundles: [filesystem-tools]
```

**API Success:**
```json
{
  "message": "Agent template installed successfully",
  "agent_id": 1,
  "agent_name": "Acme Corp File Manager",
  "environment": "production",
  "tools_installed": 3,
  "mcp_bundles": ["filesystem-tools"]
}
```

## ⚠️ Common Issues

1. **Bundle Path**: Ensure bundle path exists and is accessible
2. **Environment**: Target environment must exist in Station
3. **Required Variables**: All required variables must be provided
4. **MCP Dependencies**: Required MCP bundles must be available

## 🔍 Testing

```bash
# Validate bundle structure
stn agent bundle validate ./bundle

# Test with minimal variables
stn agent bundle install ./bundle --vars CLIENT_NAME="Test Corp" --env test

# Dry run (if supported)
stn agent bundle install ./bundle --dry-run --vars-file ./variables/test.json
```