# TurboTax-Style MCP Configuration Wizard

## Overview

The TurboTax-style wizard provides a user-friendly, step-by-step interface for configuring MCP (Model Context Protocol) servers. It automatically parses different MCP configuration formats and guides users through customization.

## Features

### 🎯 **Smart MCP Format Detection**
- **STDIO Transport**: Direct process communication (python, node, etc.)
- **Docker Transport**: Containerized servers with mount configuration  
- **HTTP Transport**: REST API-based servers
- **SSE Transport**: Server-Sent Events for real-time communication

### 🧙 **TurboTax-Style Flow**
1. **Block Selection**: Review and select which MCP servers to configure
2. **Configuration**: Step-by-step customization for each server
3. **Field Editing**: Interactive editing of commands, URLs, API keys
4. **Environment Selection**: Choose deployment environment
5. **Review**: Final configuration review before saving

### 🔧 **Intelligent Field Detection**
- **API Keys**: Automatically detects and masks sensitive fields
- **File Paths**: Smart handling of directory and file path configurations
- **URLs**: Validation and formatting for endpoint URLs
- **Docker Mounts**: Visual editing of container mount points

## Usage

### Via Command Line
```bash
# Load MCP server configurations from a README
stn load https://raw.githubusercontent.com/user/repo/main/README.md

# Specify target environment
stn load https://github.com/user/repo --environment production
```

### Programmatic Usage
```go
import "station/internal/services"

// Parse MCP configurations from various sources
blocks := []services.MCPServerBlock{
    {
        ServerName:  "filesystem",
        Description: "File system operations",
        RawBlock:    `{"command": "docker", "args": ["run", "--mount", "type=bind,src=/home,dst=/projects"]}`,
    },
}

// Run TurboTax wizard
config, environment, err := services.RunTurboWizard(blocks, []string{"dev", "staging", "prod"})
```

## Supported MCP Configuration Formats

### 1. JSON Configuration
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--mount", "type=bind,src=/Users/username/Desktop,dst=/projects/Desktop",
        "mcp/filesystem", "/projects"
      ]
    }
  }
}
```

### 2. STDIO Configuration
```json
{
  "command": "python",
  "args": ["server.py"],
  "env": {
    "API_KEY": "your-api-key-here",
    "BASE_URL": "https://api.example.com"
  }
}
```

### 3. HTTP/SSE Configuration
```json
{
  "url": "https://mcp-server.example.com/mcp",
  "env": {
    "AUTH_TOKEN": "bearer-token-here"
  }
}
```

## Architecture

### Package Structure
```
internal/services/turbo_wizard/
├── types.go          # Core types and constants
├── parser.go         # MCP configuration parsing
├── ui_components.go  # BubbleTea UI rendering
├── wizard.go         # Main wizard logic
└── handlers.go       # Input handling and state management
```

### Key Components

#### ConfigParser
- Detects transport types (STDIO, Docker, HTTP, SSE)
- Parses JSON and text-based configurations
- Extracts environment variables and API keys
- Handles Docker mount syntax

#### UIRenderer
- TurboTax-style progressive disclosure
- Consistent styling with Lipgloss
- Interactive field editing
- Environment selection interface

#### TurboWizardModel
- State management for wizard flow
- BubbleTea integration
- Configuration validation
- Environment selection

## UI Flow

### 1. Block Selection Screen
```
🧙 MCP Server Configuration Wizard

Found MCP server configurations. Select which ones you want to configure:

> ☑ filesystem [DOCKER] - File system operations with container isolation
  ☐ cost-explorer [HTTP] - AWS Cost Explorer API integration  
  ☐ knowledge-base [STDIO] - Document search and retrieval

Preview: {"command": "docker", "args": ["run", "--mount", "type=bind...

Controls: ↑/↓ navigate, SPACE toggle selection, N next, Q quit
```

### 2. Server Configuration Screen
```
🔧 Configuring: filesystem

Transport: DOCKER
Description: File system operations with container isolation

Configuration Steps:
> 1. Server Name *
  2. Docker Image *  
  3. Mounts
  4. Environment

Current Step: Server Name
Description: Unique name for this server configuration

Value: filesystem

Controls: Y accept, E edit, B back, Q quit
```

### 3. Field Editor Screen
```
✏️ Editing: Docker Image

Current value: mcp/filesystem
Enter new value (or press Enter to keep current):

┌─────────────────────────────┐
│ mcp/filesystem█             │
└─────────────────────────────┘

Controls: Type to edit, Enter to save, Esc to cancel
```

### 4. Environment Selection Screen
```
🌍 Select Environment

Choose the environment to deploy these MCP servers:

  development
> staging
  production

Controls: ↑/↓ navigate, Enter to select, B back, Q quit
```

### 5. Review Screen
```
📋 Review Configuration

You have configured 1 MCP server(s):

┌─────────────────────────────────────────────┐
│ Server 1: filesystem (docker)              │
│ Image: mcp/filesystem                       │
│ Mounts:                                     │
│   /Users/john/Desktop → /projects/Desktop   │
│ Environment:                                │
│   API_KEY=***hidden***                      │
└─────────────────────────────────────────────┘

Controls: Y accept and save, B go back, Q quit
```

## Integration Points

### Load Command Integration
The turbo wizard integrates with the existing `stn load` command:

```go
// In cmd/main/load.go
func runTurboTaxMCPFlow(readmeURL, environment, endpoint string) error {
    // Extract MCP server blocks from README
    blocks, err := discoveryService.DiscoverMCPServerBlocks(ctx, readmeURL)
    
    // Launch Turbo wizard
    wizard := services.NewTurboWizardModel(blocks)
    program := tea.NewProgram(wizard, tea.WithAltScreen())
    
    // Process results
    finalModel, err := program.Run()
    // ... handle configuration and save
}
```

### Environment Integration
- Loads available environments from database
- Validates environment selection
- Applies environment-specific configurations

### Configuration Saving
- Converts wizard output to `models.MCPConfigData`
- Handles encryption of sensitive values
- Stores in database with version control

## Benefits

### For Users
- **Intuitive**: Familiar TurboTax-style progressive interface
- **Smart**: Automatic detection of configuration formats
- **Safe**: Built-in validation and error prevention
- **Fast**: Quick configuration of complex MCP setups

### For Developers
- **Modular**: Clean separation of concerns across packages
- **Extensible**: Easy to add new transport types
- **Testable**: Isolated components with clear interfaces
- **Maintainable**: < 500 lines per file, focused responsibilities

## Future Enhancements

- **Configuration Templates**: Pre-built templates for common scenarios
- **Bulk Import**: Handle multiple README files simultaneously  
- **Advanced Validation**: Real-time connectivity testing
- **Configuration Migration**: Update existing configurations
- **Team Sharing**: Export/import configuration templates