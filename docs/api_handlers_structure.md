# API Handlers File Structure

The Station API v1 handlers have been refactored from a single large `handlers.go` file into multiple focused files for better maintainability and organization.

## File Structure

### `/internal/api/v1/handlers.go`
**Main entry point with documentation**
- Package declaration and overview documentation
- No actual code - serves as an index to other handler files

### `/internal/api/v1/base.go`
**Core structures and routing**
- `APIHandlers` struct definition with all dependencies
- `NewAPIHandlers` constructor function
- `RegisterRoutes` function that sets up all API routes
- `requireAdminInServerMode` middleware for admin-only endpoints
- Route group organization with proper authentication

### `/internal/api/v1/agents.go`
**Agent management handlers**
- `listAgents` - Get all agents (accessible to regular users)
- `callAgent` - Execute an agent with a task (accessible to regular users)
- `createAgent` - Create a new agent (admin only)
- `getAgent` - Get agent details (admin only)
- `updateAgent` - Update agent configuration (admin only) 
- `deleteAgent` - Remove an agent (admin only)
- `registerAgentAdminRoutes` - Route registration helper

### `/internal/api/v1/agent_runs.go`
**Agent execution tracking handlers**
- `listRuns` - Get recent agent runs with pagination
- `getRun` - Get detailed run information by ID
- `listRunsByAgent` - Get runs for a specific agent
- `registerAgentRunRoutes` - Route registration helper

### `/internal/api/v1/environments.go`
**Environment management handlers**
- `listEnvironments` - Get all environments
- `createEnvironment` - Create a new environment
- `getEnvironment` - Get environment details
- `updateEnvironment` - Update environment configuration
- `deleteEnvironment` - Remove an environment
- `registerEnvironmentRoutes` - Route registration helper

### `/internal/api/v1/mcp_configs.go`
**MCP configuration handlers**
- `listMCPConfigs` - Get MCP configs for an environment
- `uploadMCPConfig` - Upload and encrypt a new MCP configuration
- `getLatestMCPConfig` - Get the most recent config for an environment
- `getMCPConfig` - Get specific config by ID with decryption
- `deleteMCPConfig` - Remove an MCP configuration
- `registerMCPConfigRoutes` - Route registration helper

### `/internal/api/v1/tools.go`
**Tool discovery and listing handlers**
- `listTools` - Get available MCP tools for an environment with filtering
- `registerToolsRoutes` - Route registration helper

### `/internal/api/v1/settings.go`
**System settings management handlers**
- `UpdateSettingRequest` struct for request validation
- `listSettings` - Get all system settings
- `getSetting` - Get a specific setting by key
- `updateSetting` - Update or create a setting
- `deleteSetting` - Remove a setting
- `registerSettingsRoutes` - Route registration helper

### `/internal/api/v1/webhooks.go`
**Webhook system handlers**
- `CreateWebhookRequest` and `UpdateWebhookRequest` structs
- `createWebhook` - Create a new webhook endpoint
- `listWebhooks` - Get all registered webhooks
- `getWebhook` - Get webhook details
- `updateWebhook` - Update webhook configuration
- `deleteWebhook` - Remove a webhook
- `enableWebhook` / `disableWebhook` - Toggle webhook status
- `listWebhookDeliveries` - Get delivery history for a webhook
- `listAllWebhookDeliveries` - Get all webhook deliveries
- `registerWebhookRoutes` - Route registration helper

## Benefits of This Structure

### 🎯 **Better Organization**
- Each file focuses on a single domain/resource type
- Easy to find specific functionality
- Reduced cognitive load when working on specific features

### 🔧 **Improved Maintainability**
- Changes to webhook functionality only affect `webhooks.go`
- Less risk of merge conflicts
- Easier to test individual components

### 👥 **Better Team Collaboration**
- Multiple developers can work on different handler files simultaneously
- Clear ownership boundaries for different API endpoints
- Easier code reviews with focused changesets

### 📚 **Enhanced Readability**
- Each file is focused and self-contained
- Related functionality is grouped together
- Clear separation between public user APIs and admin APIs

### 🚀 **Easier Extensions**
- Adding new webhook features only requires changes to `webhooks.go`
- New resource types get their own dedicated files
- Route registration is centralized but organized

## Route Organization

The API follows a clear pattern:

```
/api/v1/
├── agents/                    # User-accessible agent operations
│   ├── GET    ""             # List agents
│   └── POST   ":id/execute"  # Execute agent
├── agents/                    # Admin-only agent management  
│   ├── POST   ""             # Create agent
│   ├── GET    ":id"          # Get agent
│   ├── PUT    ":id"          # Update agent
│   └── DELETE ":id"          # Delete agent
├── runs/                      # Agent execution history
├── environments/              # Environment management (admin)
│   └── :env_id/
│       ├── mcp-configs/      # MCP configurations
│       └── tools/            # Available tools
├── settings/                  # System settings (admin)
├── webhooks/                  # Webhook management (admin)
│   └── :id/deliveries/       # Webhook delivery history
└── webhook-deliveries/        # All webhook deliveries (admin)
```

## Authentication & Authorization

- **Local Mode**: No authentication required
- **Server Mode**: 
  - All routes require authentication
  - Admin-only routes require admin privileges
  - User routes accessible to regular authenticated users

## Development Workflow

When working on API endpoints:

1. **Find the right file**: Look for the resource type (agents, webhooks, etc.)
2. **Add handler function**: Implement the logic in the appropriate file
3. **Register route**: Add route registration in the `register*Routes` function
4. **Update base.go**: Call the registration function in `RegisterRoutes`

This structure makes Station's API codebase much more maintainable and easier to work with! 🎉