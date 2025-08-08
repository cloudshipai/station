package services

import (
	"context"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// IntelligentAgentCreator is a backward compatibility wrapper
// It delegates to the new modular architecture while maintaining the same interface
type IntelligentAgentCreator struct {
	service      *IntelligentAgentService
	agentService AgentServiceInterface // For backward compatibility
}

// NewIntelligentAgentCreator creates a new intelligent agent creator (backward compatibility)
func NewIntelligentAgentCreator(repos *repositories.Repositories, agentService AgentServiceInterface) *IntelligentAgentCreator {
	return &IntelligentAgentCreator{
		service:      NewIntelligentAgentService(repos, agentService),
		agentService: agentService,
	}
}

// CreateIntelligentAgent creates a new agent using AI-powered planning
func (iac *IntelligentAgentCreator) CreateIntelligentAgent(ctx context.Context, req AgentCreationRequest) (*models.Agent, error) {
	return iac.service.CreateIntelligentAgent(ctx, req)
}

// ExecuteAgentViaStdioMCP executes an agent using the MCP architecture
func (iac *IntelligentAgentCreator) ExecuteAgentViaStdioMCP(ctx context.Context, agent *models.Agent, task string, runID int64) (*AgentExecutionResult, error) {
	return iac.service.ExecuteAgentViaStdioMCP(ctx, agent, task, runID)
}

// TestStdioMCPConnection tests the MCP connection for debugging
func (iac *IntelligentAgentCreator) TestStdioMCPConnection(ctx context.Context) error {
	return iac.service.TestStdioMCPConnection(ctx)
}

// assignToolsToAgent provides backward compatibility for the old method
func (iac *IntelligentAgentCreator) assignToolsToAgent(agentID int64, toolNames []string, environmentID int64) int {
	return iac.service.planGenerator.assignToolsToAgent(agentID, toolNames, environmentID)
}