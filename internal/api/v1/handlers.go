package v1

// This file serves as the main entry point for v1 API handlers.
// The actual handler implementations are split across multiple files:
//
// - base.go: Base structures, route registration, and common functionality
// - agents.go: Agent-related handlers (list, create, update, delete, execute)
// - agent_runs.go: Agent run handlers (list runs, get run details)
// - environments.go: Environment management handlers
// - mcp_configs.go: MCP configuration handlers (upload, decrypt, manage configs)
// - tools.go: Tool listing and filtering handlers
// - settings.go: Settings management handlers
//
// This modular approach makes the codebase more maintainable and easier to navigate.