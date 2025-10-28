package mcp

import (
	"log"

	"github.com/mark3labs/mcp-go/mcp"
)

// setupResources initializes all MCP resources for read-only data access
func (s *Server) setupResources() {
	// Add static resources for read-only data access
	s.setupStaticResources()

	// Add dynamic resource templates for parameterized access
	s.setupResourceTemplates()

	log.Printf("MCP resources setup complete - read-only data access via Resources, operations via Tools")
}

// setupStaticResources adds static resources for Station data discovery
func (s *Server) setupStaticResources() {
	// Environments list resource
	environmentsResource := mcp.NewResource(
		"station://environments",
		"Station Environments",
		mcp.WithResourceDescription("List all available environments with their configurations"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(environmentsResource, s.handleEnvironmentsResource)

	// Agents list resource
	agentsResource := mcp.NewResource(
		"station://agents",
		"Station Agents",
		mcp.WithResourceDescription("List all available agents with basic information"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(agentsResource, s.handleAgentsResource)

	// MCP configs list resource
	configsResource := mcp.NewResource(
		"station://mcp-configs",
		"MCP Configurations",
		mcp.WithResourceDescription("List all MCP server configurations across environments"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(configsResource, s.handleMCPConfigsResource)
}

// setupResourceTemplates adds dynamic resource templates for parameterized access
func (s *Server) setupResourceTemplates() {
	// Agent details resource template
	agentDetailsTemplate := mcp.NewResource(
		"station://agents/{id}",
		"Agent Details",
		mcp.WithResourceDescription("Get detailed information about a specific agent including tools and configuration"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(agentDetailsTemplate, s.handleAgentDetailsResource)

	// Environment tools resource template
	envToolsTemplate := mcp.NewResource(
		"station://environments/{id}/tools",
		"Environment Tools",
		mcp.WithResourceDescription("List all MCP tools available in a specific environment"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(envToolsTemplate, s.handleEnvironmentToolsResource)

	// Agent runs resource template
	agentRunsTemplate := mcp.NewResource(
		"station://agents/{id}/runs",
		"Agent Execution History",
		mcp.WithResourceDescription("Get execution history and results for a specific agent"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(agentRunsTemplate, s.handleAgentRunsResource)
}
