package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// setupToolSuggestion adds the intelligent agent configuration suggestion tool
func (s *Server) setupToolSuggestion() {
	// Agent configuration suggestion tool
	suggestAgentTool := mcp.NewTool("suggest_agent_config",
		mcp.WithDescription("Analyze user requirements and suggest optimal agent configuration with tool recommendations"),
		mcp.WithString("user_request", mcp.Required(), mcp.Description("What the user wants to accomplish with this agent")),
		mcp.WithString("domain", mcp.Description("Area of work (devops, data-science, security, etc.)")),
		mcp.WithString("environment_name", mcp.Description("Preferred environment name (optional)")),
		mcp.WithBoolean("include_tool_details", mcp.Description("Include detailed tool descriptions (default: true)")),
	)

	s.mcpServer.AddTool(suggestAgentTool, s.handleSuggestAgentConfig)
}
