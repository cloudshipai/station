package dotprompt

import (
	"context"
	"fmt"
	"path/filepath"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// Manager provides high-level operations for dotprompt agent management
type Manager struct {
	repos     *repositories.Repositories
	converter *Converter
	executor  *Executor
}

// NewManager creates a new dotprompt manager
func NewManager(repos *repositories.Repositories, genkitProvider interface{}, mcpManager interface{}) *Manager {
	return &Manager{
		repos:     repos,
		converter: NewConverter(ConversionOptions{
			IncludeMetadata:   true,
			PreserveFormatting: true,
			GenerateToolDocs:   true,
			UseMinimalSchema:   false,
		}),
		executor: NewExecutor(genkitProvider, mcpManager),
	}
}

// ExportAgentToDotprompt exports a Station agent to dotprompt format
func (m *Manager) ExportAgentToDotprompt(ctx context.Context, agentID int64, outputDir string) (*AgentBundle, error) {
	// Get agent from database
	agent, err := m.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Get assigned tools
	assignedTools, err := m.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent tools: %w", err)
	}

	// Build tool mappings (this would need to be enhanced with actual MCP server info)
	var toolMappings []ToolMapping
	for _, tool := range assignedTools {
		toolMappings = append(toolMappings, ToolMapping{
			ToolName:    tool.ToolName,
			Environment: "default", // TODO: Get actual environment
			// Additional fields would be populated from MCP server info
		})
	}

	// Convert to dotprompt
	dotprompt, err := m.converter.AgentToDotprompt(agent, toolMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to convert agent to dotprompt: %w", err)
	}

	// Set output path
	if outputDir != "" {
		dotprompt.FilePath = filepath.Join(outputDir, filepath.Base(dotprompt.FilePath))
	}

	// Save dotprompt file
	err = m.converter.SaveDotprompt(dotprompt)
	if err != nil {
		return nil, fmt.Errorf("failed to save dotprompt: %w", err)
	}

	// Create agent bundle for portability
	bundle := &AgentBundle{
		Dotprompt:    *dotprompt,
		ToolMappings: toolMappings,
		MCPServers:   make(map[string]MCPServerInfo), // TODO: Populate from actual MCP config
		Environment:  "default",
		Version:      "1.0",
	}

	return bundle, nil
}

// ImportAgentFromDotprompt imports a dotprompt file as a Station agent
func (m *Manager) ImportAgentFromDotprompt(ctx context.Context, dotpromptPath string, environmentID int64) (*models.Agent, error) {
	// Convert dotprompt to agent
	agent, toolMappings, err := m.converter.DotpromptToAgent(dotpromptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert dotprompt to agent: %w", err)
	}

	// Set environment
	agent.EnvironmentID = environmentID
	agent.CreatedBy = 1 // TODO: Get from context

	// Create agent in database  
	createdAgent, err := m.repos.Agents.Create(agent.Name, agent.Description, agent.Prompt, agent.MaxSteps, environmentID, 1, nil, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// TODO: Create tool assignments from toolMappings
	// This would require resolving tool names to actual tool IDs in the MCP system
	_ = toolMappings // Suppress unused variable warning for now

	return createdAgent, nil
}

// ExecuteAgentWithDotprompt executes an agent using dotprompt methodology
func (m *Manager) ExecuteAgentWithDotprompt(ctx context.Context, agentID int64, request ExecutionRequest) (*ExecutionResponse, error) {
	// Get agent and convert to dotprompt format
	agent, err := m.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Get tool mappings
	assignedTools, err := m.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent tools: %w", err)
	}

	var toolMappings []ToolMapping
	for _, tool := range assignedTools {
		toolMappings = append(toolMappings, ToolMapping{
			ToolName:    tool.ToolName,
			Environment: "default",
		})
	}

	// Convert to dotprompt config
	dotprompt, err := m.converter.AgentToDotprompt(agent, toolMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to convert agent to dotprompt: %w", err)
	}

	// Execute using dotprompt
	return m.executor.ExecuteAgentFromConfig(ctx, dotprompt.Config, dotprompt.Template, request, toolMappings)
}

// ValidateDotprompt validates a dotprompt file for Station compatibility
func (m *Manager) ValidateDotprompt(dotpromptPath string) (*ValidationResult, error) {
	dotprompt, err := m.converter.LoadDotprompt(dotpromptPath)
	if err != nil {
		return &ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to load dotprompt: %v", err)},
		}, nil
	}

	var errors []string
	var warnings []string

	// Validate required fields
	if dotprompt.Config.Metadata.Name == "" {
		errors = append(errors, "metadata.name is required")
	}

	if dotprompt.Config.Model == "" {
		errors = append(errors, "model is required")
	}

	if dotprompt.Template == "" {
		errors = append(errors, "template content is required")
	}

	// Validate tools exist (this would check against MCP servers)
	if len(dotprompt.Config.Tools) == 0 {
		warnings = append(warnings, "No tools specified - agent may have limited functionality")
	}

	// Validate input schema
	if dotprompt.Config.Input.Schema == nil {
		warnings = append(warnings, "No input schema specified")
	}

	return &ValidationResult{
		Valid:    len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}, nil
}

// ListDotprompts lists all dotprompt files in the prompts directory
func (m *Manager) ListDotprompts(promptsDir string) ([]string, error) {
	pattern := filepath.Join(promptsDir, "**", "*.prompt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob dotprompt files: %w", err)
	}
	return matches, nil
}

// ValidationResult represents the result of dotprompt validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}