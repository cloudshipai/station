# Station - MCP Agent Management Platform

## Product Requirements Document

### Mission
Make building agents through MCP with specialized tools easy and manageable.

## Technical Architecture

### Core Technologies
- **Language**: Go with clean structure and best practices
- **API**: Gin framework
- **MCP Server**: Streamable HTTP using mcp-go library
- **Admin Interface**: Wish SSH app serving Bubble Tea applications
- **Database**: SQLite (preferably libsql) with sqlc for state management
- **Agent Framework**: Eino for agent execution

### Application Components

#### 1. SSH Admin Interface (Wish + Bubble Tea)
- SSH endpoint for admins with SSH keys
- Multiple Bubble Tea applications for administration
- Reference implementation: @opencode/packages/tui/
- Admins can:
  - Upload MCP configs via SCP or text editor
  - Define environments for MCP configs
  - Manage MCP users and provision API keys
  - View agent runs and outputs

#### 2. MCP Configuration Management
- **Storage**: JSON blob in SQLite, encrypted (contains API keys)
- **Versioning**: Increment version when servers/configs are added
- **Environment Association**: Each MCP config tied to an environment
- **Structure**: Users define environment → MCP config gets assigned to it

#### 3. Tool Discovery Process
When MCP config updates (v1+ or new upload):
1. Use mcp-go package to list all available tools from all servers in config
2. Store tools per MCP server
3. Link to: environment → mcp config → version → servers → tools

#### 4. User Management
- **Admin Users**: Anyone with SSH access (auto-admin)
- **Regular Users**: Managed by admins via API keys
- **Authentication**: Bearer token based on admin-created API keys

### Database Schema

#### Environments
- Environment management for MCP config organization

#### MCP Configs
- Versioned, encrypted JSON blobs
- Tied to environments
- Contains server configurations and API keys

#### MCP Servers
- Extracted from MCP configs
- Linked to config versions and environments

#### Tools
- Discovered from MCP servers
- Linked to: environment → mcp config → version → servers → tools

#### Agents
- **Config**: Prompt configuration
- **Max Steps**: Default 5, configurable
- **Tools**: List of tool IDs from MCP servers
- **Description**: Agent description

#### Agent Runs
- Final agent response output
- Tool calls and steps (if possible)
- Execution history and results

### Agent Framework Integration (Eino)
- Create template for React agent pattern
- Dynamically create agents with user-defined config
- Execute agents with specified tools and prompts

### MCP Server Implementation

#### Resources
- List environments
- List servers in environment  
- List tools in environment
- List tools per server per environment
- List agents

#### Tools
1. **Create Agent** (Admin only)
   - Input: prompt, description, tools
   - Creates new agent configuration

2. **Call Agent**
   - Input: agent ID, task
   - Executes agent and returns results

#### Authentication
- Bearer token authentication using admin-created API keys

### Deployment Architecture

#### Single Binary Compilation
The application serves three endpoints simultaneously:
1. **SSH Endpoint**: Wish + Bubble Tea admin interface
2. **MCP Server Endpoint**: Streamable HTTP MCP server
3. **HTTP API Endpoint**: Gin API with health route only

#### Graceful Shutdown
- Proper cleanup of all running services
- Safe database connection closure
- Active session management

### Security Requirements
- Encrypted storage of MCP configurations
- API key-based authentication
- SSH key-based admin access
- Secure handling of sensitive configuration data

### User Workflows

#### Admin Workflow
1. SSH into admin interface
2. Upload/edit MCP configuration via text editor
3. Assign to environment
4. System discovers tools automatically
5. Create agents using discovered tools
6. Provision API keys for users

#### User Workflow (via MCP client)
1. Authenticate using API key
2. List available agents and tools
3. Call agents with tasks
4. Receive execution results

#### Agent Execution Flow
1. User calls agent via MCP
2. System loads agent configuration
3. Dynamically creates Eino agent instance
4. Executes with available tools
5. Returns results and logs run history