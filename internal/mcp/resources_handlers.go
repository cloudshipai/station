package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
)

// Real resource handlers with proper database integration

func (s *Server) handleEnvironmentsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Get all environments from database
	environments, err := s.repos.Environments.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	// Format response optimized for LLM context loading
	response := map[string]interface{}{
		"total_count":  len(environments),
		"environments": environments,
		"resource_uri": "station://environments",
		"timestamp":    time.Now(),
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal environments: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:  request.Params.URI,
			Text: string(jsonData),
		},
	}, nil
}

func (s *Server) handleAgentsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Get all agents from database
	agents, err := s.repos.Agents.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Format response optimized for LLM context loading
	response := map[string]interface{}{
		"total_count": len(agents),
		"agents":      agents,
		"resource_uri": "station://agents",
		"timestamp":   time.Now(),
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agents: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:  request.Params.URI,
			Text: string(jsonData),
		},
	}, nil
}

func (s *Server) handleMCPConfigsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Get all file-based MCP configs across environments
	environments, err := s.repos.Environments.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	
	var allConfigs []interface{}
	for _, env := range environments {
		fileConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(env.ID)
		if err != nil {
			continue // Skip environments with no configs
		}
		for _, fc := range fileConfigs {
			allConfigs = append(allConfigs, map[string]interface{}{
				"id":             fc.ID,
				"name":           fc.ConfigName,
				"environment_id": fc.EnvironmentID,
				"environment":    env.Name,
				"path":           fc.TemplatePath,
				"type":           "file",
				"last_loaded":    fc.LastLoadedAt,
			})
		}
	}

	// Format response optimized for LLM context loading
	response := map[string]interface{}{
		"total_count": len(allConfigs),
		"mcp_configs": allConfigs,
		"resource_uri": "station://mcp-configs",
		"timestamp":   time.Now(),
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP configs: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:  request.Params.URI,
			Text: string(jsonData),
		},
	}, nil
}

func (s *Server) handleAgentDetailsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract agent ID from URI using regex
	agentID, err := s.extractIDFromURI(request.Params.URI, `station://agents/(\d+)`)
	if err != nil {
		return nil, fmt.Errorf("invalid agent URI: %w", err)
	}

	// Get agent details from database
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Get environment details
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		log.Printf("Warning: Could not get environment for agent %d: %v", agentID, err)
		environment = &models.Environment{Name: "Unknown", Description: nil}
	}

	// Get assigned tools
	agentTools, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		log.Printf("Warning: Could not get tools for agent %d: %v", agentID, err)
		agentTools = []*models.AgentToolWithDetails{}
	}

	// Extract tool details for response
	tools := make([]map[string]interface{}, len(agentTools))
	for i, agentTool := range agentTools {
		tools[i] = map[string]interface{}{
			"name":        agentTool.ToolName,
			"id":          agentTool.ID,
			"server_name": agentTool.ServerName,
		}
	}

	// Format comprehensive response
	response := map[string]interface{}{
		"agent": map[string]interface{}{
			"id":          agent.ID,
			"name":        agent.Name,
			"description": agent.Description,
			"prompt":      agent.Prompt,
			"max_steps":   agent.MaxSteps,
			"created_at":  agent.CreatedAt,
			"updated_at":  agent.UpdatedAt,
		},
		"environment": map[string]interface{}{
			"id":          environment.ID,
			"name":        environment.Name,
			"description": environment.Description,
		},
		"tools":       tools,
		"tools_count": len(tools),
		"resource_uri": request.Params.URI,
		"timestamp":   time.Now(),
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent details: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:  request.Params.URI,
			Text: string(jsonData),
		},
	}, nil
}

func (s *Server) handleEnvironmentToolsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract environment ID from URI
	envID, err := s.extractIDFromURI(request.Params.URI, `station://environments/(\d+)/tools`)
	if err != nil {
		return nil, fmt.Errorf("invalid environment tools URI: %w", err)
	}

	// Get environment details
	environment, err := s.repos.Environments.GetByID(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	// Get tools for this environment
	tools, err := s.repos.MCPTools.GetByEnvironmentID(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools for environment: %w", err)
	}

	// Format tools for response
	var envTools []interface{}
	for _, tool := range tools {
		envTools = append(envTools, map[string]interface{}{
			"id":          tool.ID,
			"name":        tool.Name,
			"description": tool.Description,
			// ServerName is not available in MCPTool model, would need to join with server data
		})
	}

	// Format response
	response := map[string]interface{}{
		"environment": map[string]interface{}{
			"id":          environment.ID,
			"name":        environment.Name,
			"description": environment.Description,
		},
		"tools":       envTools,
		"tools_count": len(envTools),
		"resource_uri": request.Params.URI,
		"timestamp":   time.Now(),
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal environment tools: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:  request.Params.URI,
			Text: string(jsonData),
		},
	}, nil
}

func (s *Server) handleAgentRunsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract agent ID from URI
	agentID, err := s.extractIDFromURI(request.Params.URI, `station://agents/(\d+)/runs`)
	if err != nil {
		return nil, fmt.Errorf("invalid agent runs URI: %w", err)
	}

	// Get agent details for context
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Get recent runs for this agent
	// Note: The repository interface might vary - this is a common pattern
	var runs []interface{}
	
	// Try to get runs if a runs repository exists
	// This would need to be implemented based on the actual repository structure
	runsCount := 0

	// Format response
	response := map[string]interface{}{
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"runs":       runs,
		"runs_count": runsCount,
		"resource_uri": request.Params.URI,
		"timestamp":  time.Now(),
		"note":       "Run history integration pending - repository interface needed",
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent runs: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:  request.Params.URI,
			Text: string(jsonData),
		},
	}, nil
}

// Utility function for extracting IDs from resource URIs
func (s *Server) extractIDFromURI(uri, pattern string) (int64, error) {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(uri)
	if len(matches) < 2 {
		return 0, fmt.Errorf("no ID found in URI: %s", uri)
	}
	
	id, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ID format: %s", matches[1])
	}
	
	return id, nil
}