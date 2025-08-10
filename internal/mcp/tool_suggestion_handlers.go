package mcp

import (
	"context"
	"fmt"
	"strings"

	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleSuggestAgentConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireAuthInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Extract parameters
	userRequest, err := request.RequireString("user_request")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'user_request' parameter: %v", err)), nil
	}

	var domain string
	var environmentName string
	includeDetails := true

	// Extract optional parameters safely
	if args := request.Params.Arguments; args != nil {
		if argsMap, ok := args.(map[string]interface{}); ok {
			if domainValue, exists := argsMap["domain"]; exists {
				if domainStr, ok := domainValue.(string); ok {
					domain = domainStr
				}
			}
			if envValue, exists := argsMap["environment_name"]; exists {
				if envStr, ok := envValue.(string); ok {
					environmentName = envStr
				}
			}
			if detailsValue, exists := argsMap["include_tool_details"]; exists {
				if detailsBool, ok := detailsValue.(bool); ok {
					includeDetails = detailsBool
				}
			}
		}
	}

	// Find or default environment
	var targetEnv *models.Environment
	if environmentName != "" {
		env, err := s.repos.Environments.GetByName(environmentName)
		if err != nil {
			// If named environment not found, get default
			environments, err := s.repos.Environments.List()
			if err != nil || len(environments) == 0 {
				return mcp.NewToolResultError("No environments available"), nil
			}
			targetEnv = environments[0] // Use first available environment
		} else {
			targetEnv = env
		}
	} else {
		// Get default environment (first available)
		environments, err := s.repos.Environments.List()
		if err != nil || len(environments) == 0 {
			return mcp.NewToolResultError("No environments available"), nil
		}
		targetEnv = environments[0]
	}

	// Get available tools for analysis
	availableTools, err := s.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get available tools: %v", err)), nil
	}

	// Convert tools for analysis
	var envTools []*models.MCPTool
	for _, toolDetail := range availableTools {
		tool := &models.MCPTool{
			ID:          toolDetail.MCPTool.ID,
			Name:        toolDetail.MCPTool.Name,
			Description: toolDetail.Description,
		}
		envTools = append(envTools, tool)
	}

	// Analyze request and suggest tools
	suggestions := s.analyzeUserRequestAndSuggestTools(userRequest, domain, envTools)

	// Build comprehensive response
	response := s.buildAgentSuggestionResponse(userRequest, domain, targetEnv, suggestions, includeDetails)

	return mcp.NewToolResultText(response), nil
}

func (s *Server) analyzeUserRequestAndSuggestTools(userRequest, domain string, availableTools []*models.MCPTool) map[string][]string {
	request := strings.ToLower(userRequest)
	domain = strings.ToLower(domain)

	// Tool suggestion patterns based on keywords
	patterns := map[string][]string{
		// AWS/Cloud patterns
		"aws|cloud|ec2|s3|lambda|cloudwatch": {"aws", "cloud", "s3", "ec2", "lambda", "cloudwatch"},
		
		// Log analysis patterns  
		"log|logs|logging|analyze|analysis|search|grep|find": {"log", "search", "grep", "file", "text"},
		
		// Database patterns
		"database|db|sql|query|data": {"database", "sql", "query", "data"},
		
		// Web/HTTP patterns
		"http|api|web|request|curl|fetch": {"http", "web", "api", "request", "curl"},
		
		// DevOps patterns
		"deploy|deployment|monitor|monitoring|docker|kubernetes": {"docker", "kubernetes", "deploy", "monitor"},
		
		// Security patterns
		"security|scan|vulnerability|audit|compliance": {"security", "scan", "audit", "vulnerability"},
		
		// File operations
		"file|files|directory|folder|read|write": {"file", "directory", "read", "write"},
		
		// Version control
		"git|github|gitlab|repository|repo|commit": {"git", "github", "gitlab", "repository"},
	}

	// Score tools based on pattern matching
	toolScores := make(map[string]int)
	
	for _, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(request, keyword) || strings.Contains(domain, keyword) {
				// Find matching tools
				for _, tool := range availableTools {
					toolName := strings.ToLower(tool.Name)
					if strings.Contains(toolName, keyword) || strings.Contains(strings.ToLower(tool.Description), keyword) {
						toolScores[tool.Name] += 10
					}
				}
			}
		}
	}

	// Categorize suggested tools
	var coreTools, contextTools, fallbackTools []string
	
	for _, tool := range availableTools {
		score := toolScores[tool.Name]
		if score >= 20 {
			coreTools = append(coreTools, tool.Name)
		} else if score >= 10 {
			contextTools = append(contextTools, tool.Name)
		} else {
			// Add some common fallback tools
			toolName := strings.ToLower(tool.Name)
			if strings.Contains(toolName, "file") || strings.Contains(toolName, "search") || 
			   strings.Contains(toolName, "json") || strings.Contains(toolName, "text") {
				fallbackTools = append(fallbackTools, tool.Name)
			}
		}
	}

	// Limit suggestions to prevent context overload
	if len(coreTools) > 5 {
		coreTools = coreTools[:5]
	}
	if len(contextTools) > 3 {
		contextTools = contextTools[:3]
	}
	if len(fallbackTools) > 2 {
		fallbackTools = fallbackTools[:2]
	}

	return map[string][]string{
		"core":     coreTools,
		"context":  contextTools,
		"fallback": fallbackTools,
	}
}

func (s *Server) buildAgentSuggestionResponse(userRequest, domain string, env *models.Environment, suggestions map[string][]string, includeDetails bool) string {
	response := fmt.Sprintf(`# 🤖 Station Agent Configuration Suggestion

## Analysis Summary
**User Request**: %s
**Domain**: %s
**Target Environment**: %s (%s)

## Recommended Agent Configuration

### 🎯 Suggested Agent Details
- **Name**: %s
- **Description**: %s
- **Max Steps**: %d
- **Schedule**: %s

### 🛠️ Tool Selection Strategy

**Core Tools** (Essential for primary workflow):
%s

**Context Tools** (Environment-specific support):
%s

**Fallback Tools** (Edge case handling):
%s

### 📝 Suggested System Prompt

%s

### ⚙️ Execution Configuration
- **Environment**: %s
- **Max Steps**: %d (Balances thoroughness with efficiency)
- **Schedule**: %s
- **Monitoring**: Enable execution logging and error tracking

### 🎯 Success Criteria
- Completes primary task within max steps
- Provides clear, actionable output
- Handles errors gracefully with fallback strategies
- Maintains consistent performance across executions

### 🚀 Next Steps
1. Review the suggested configuration above
2. Modify any settings that don't match your requirements
3. Create the agent using: create_agent tool with these specifications
4. Test with a simple task to validate functionality
5. Deploy to production with monitoring enabled

**Ready to create this agent?** Use the create_agent MCP tool with the configuration above!`,
		userRequest,
		domain,
		env.Name, getStringValue(env.Description),
		s.generateAgentName(userRequest),
		s.generateAgentDescription(userRequest, domain),
		s.suggestMaxSteps(userRequest, len(suggestions["core"])+len(suggestions["context"])),
		s.suggestSchedule(userRequest),
		s.formatToolList(suggestions["core"]),
		s.formatToolList(suggestions["context"]),
		s.formatToolList(suggestions["fallback"]),
		s.generateSystemPrompt(userRequest, domain, suggestions),
		env.Name,
		s.suggestMaxSteps(userRequest, len(suggestions["core"])+len(suggestions["context"])),
		s.suggestSchedule(userRequest))

	return response
}

func (s *Server) formatToolList(tools []string) string {
	if len(tools) == 0 {
		return "- (None recommended for this category)"
	}
	
	var result strings.Builder
	for _, tool := range tools {
		result.WriteString(fmt.Sprintf("- %s\n", tool))
	}
	return result.String()
}

func (s *Server) generateAgentName(userRequest string) string {
	request := strings.ToLower(userRequest)
	
	if strings.Contains(request, "log") || strings.Contains(request, "analyze") {
		return "log-analyzer"
	} else if strings.Contains(request, "monitor") {
		return "system-monitor"
	} else if strings.Contains(request, "deploy") {
		return "deployment-manager"
	} else if strings.Contains(request, "security") || strings.Contains(request, "scan") {
		return "security-scanner"
	} else if strings.Contains(request, "data") || strings.Contains(request, "process") {
		return "data-processor"
	} else if strings.Contains(request, "aws") || strings.Contains(request, "cloud") {
		return "cloud-manager"
	}
	
	return "intelligent-agent"
}

func (s *Server) generateAgentDescription(userRequest, domain string) string {
	return fmt.Sprintf("Intelligent agent designed to %s with focus on %s domain operations", 
		strings.ToLower(userRequest), domain)
}

func (s *Server) suggestMaxSteps(userRequest string, toolCount int) int {
	request := strings.ToLower(userRequest)
	
	// Complex operations need more steps
	if strings.Contains(request, "analyze") || strings.Contains(request, "monitor") || 
	   strings.Contains(request, "process") || strings.Contains(request, "deploy") {
		return min(toolCount+5, 10) // Cap at 10 steps
	}
	
	// Simple operations need fewer steps
	if strings.Contains(request, "list") || strings.Contains(request, "get") || 
	   strings.Contains(request, "show") {
		return min(toolCount+2, 5)
	}
	
	// Default reasonable step count
	return min(toolCount+3, 7)
}

func (s *Server) suggestSchedule(userRequest string) string {
	request := strings.ToLower(userRequest)
	
	if strings.Contains(request, "monitor") || strings.Contains(request, "watch") {
		return "*/15 * * * * (every 15 minutes)" // Frequent monitoring
	} else if strings.Contains(request, "daily") || strings.Contains(request, "report") {
		return "0 9 * * * (daily at 9 AM)" // Daily reporting
	} else if strings.Contains(request, "backup") || strings.Contains(request, "cleanup") {
		return "0 2 * * * (daily at 2 AM)" // Maintenance tasks
	}
	
	return "on-demand (manual execution)"
}

func (s *Server) generateSystemPrompt(userRequest, domain string, suggestions map[string][]string) string {
	return fmt.Sprintf(`You are an intelligent Station agent specialized in %s.

Your primary mission: %s

Workflow:
1. Analyze the input task thoroughly
2. Use your core tools strategically: %v
3. Leverage context tools when needed: %v
4. Fall back to basic tools for edge cases: %v
5. Provide clear, actionable results
6. Handle errors gracefully with detailed explanations

Guidelines:
- Be thorough but efficient - respect the max steps limit
- Always explain your reasoning and next steps
- If something fails, try alternative approaches
- Provide specific, actionable recommendations
- Format output clearly for easy consumption

Quality Gates:
- Validate inputs before processing
- Check tool outputs for completeness
- Provide confidence levels for recommendations
- Include fallback options when appropriate`, 
		domain, userRequest, suggestions["core"], suggestions["context"], suggestions["fallback"])
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getStringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}