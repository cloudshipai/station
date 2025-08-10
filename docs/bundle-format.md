# Station Template Bundle Format

## Overview
Station template bundles are packaged environments that include MCP configurations, agent definitions, and metadata for easy sharing and installation.

## Bundle Structure (.bundle format)

A `.bundle` file is a compressed tar.gz archive with the following structure:

```
my-bundle.bundle (tar.gz)
├── bundle.yml          # Bundle metadata
├── mcp-configs/        # MCP server configurations (templated)
│   ├── filesystem.json
│   ├── aws-tools.json
│   └── ...
├── agents/             # Agent prompt files (optional)
│   ├── file-manager.prompt
│   ├── aws-assistant.prompt
│   └── ...
├── variables.example.yml # Example variables file
└── README.md           # Bundle documentation (optional)
```

## Bundle Metadata (bundle.yml)

```yaml
name: "filesystem-aws-toolkit"
version: "1.0.0"
description: "File management and AWS tools with intelligent agents"
author: "cloudshipai"
homepage: "https://github.com/cloudshipai/registry"
tags: ["filesystem", "aws", "productivity"]

# Bundle requirements
requirements:
  station_version: ">=0.1.0"
  
# Variable definitions
variables:
  - name: "AWS_REGION"
    description: "AWS region for operations"
    required: true
    default: "us-east-1"
    
  - name: "AWS_ACCESS_KEY_ID" 
    description: "AWS access key ID"
    required: true
    secret: true
    
  - name: "ROOT_PATH"
    description: "Root filesystem path for operations"
    required: true
    default: "/tmp"

# MCP servers included
mcp_servers:
  - name: "filesystem"
    description: "File system operations"
    
  - name: "aws-tools"
    description: "AWS SDK operations"

# Agents included  
agents:
  - name: "file-manager" 
    description: "Intelligent file management assistant"
    tools: ["__read_file", "__write_file", "__list_directory"]
    
  - name: "aws-assistant"
    description: "AWS operations assistant"
    tools: ["__s3_upload", "__s3_download", "__ec2_list"]
```

## Installation Process

1. **Extract Bundle**: Unpack `.bundle` to temp directory
2. **Validate**: Check bundle.yml format and requirements
3. **Copy MCP Configs**: Copy `mcp-configs/*` to `~/.config/station/environments/{env}/`
4. **Copy Agents**: Copy `agents/*` to `~/.config/station/environments/{env}/agents/`
5. **Create Variables Template**: Create `variables.yml` from bundle metadata
6. **Sync**: Run `stn sync` to process templates and prompt for variables

## Creation Process

1. **Scan Environment**: Detect MCP configs and agents in environment
2. **Generate Metadata**: Create bundle.yml from discovered resources
3. **Create Archive**: Package into .bundle (tar.gz) format
4. **Validate Bundle**: Test installation in clean environment

## Variable Handling

- **Template Variables**: Use Go template syntax `{{ .VAR_NAME }}`
- **Secret Detection**: Automatically detect secret variables by name patterns
- **Interactive Prompting**: Prompt user during `stn sync` for missing variables
- **Example File**: Include `variables.example.yml` with sample values (secrets redacted)