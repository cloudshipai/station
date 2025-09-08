package dotprompt

import (
	"fmt"
	"strings"

	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/google/dotprompt/go/dotprompt"
)

// ============================================================================
// DOTPROMPT TEMPLATE UTILITIES
// ============================================================================

func (e *GenKitExecutor) extractToolNames(agentTools []*models.AgentToolWithDetails) []string {
	var toolNames []string
	for _, tool := range agentTools {
		// AgentToolWithDetails has ToolName field from the join
		toolNames = append(toolNames, tool.ToolName)
	}
	return toolNames
}

// isDotpromptContent checks if the prompt contains dotprompt frontmatter or multi-role syntax
func (e *GenKitExecutor) isDotpromptContent(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	
	// Check for frontmatter (starts with ---)
	if strings.HasPrefix(trimmed, "---") {
		return true
	}
	
	// Check for multi-role syntax like {{role "system"}} or {{role "user"}}
	rolePatterns := []string{
		"{{role \"system\"}}",
		"{{role \"user\"}}",
		"{{role \"assistant\"}}",
		"{{role 'system'}}",
		"{{role 'user'}}",
		"{{role 'assistant'}}",
	}
	
	lowerPrompt := strings.ToLower(trimmed)
	for _, pattern := range rolePatterns {
		if strings.Contains(lowerPrompt, strings.ToLower(pattern)) {
			return true
		}
	}
	
	return false
}

// getPromptSource returns a string indicating the source of the prompt format
func (e *GenKitExecutor) getPromptSource(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	
	if strings.HasPrefix(trimmed, "---") {
		return "frontmatter"
	}
	
	rolePatterns := []string{
		"{{role \"system\"}}",
		"{{role \"user\"}}",
		"{{role \"assistant\"}}",
	}
	
	lowerPrompt := strings.ToLower(trimmed)
	for _, pattern := range rolePatterns {
		if strings.Contains(lowerPrompt, strings.ToLower(pattern)) {
			return "multi_role"
		}
	}
	
	return "legacy"
}

// RenderDotpromptContent renders a dotprompt template with the given variables using the new multi-role system
func (e *GenKitExecutor) RenderDotpromptContent(dotpromptContent, task, agentName string) (string, error) {
	// Create dotprompt instance for rendering
	dp := dotprompt.NewDotprompt(nil)
	
	// Prepare data for rendering
	data := &dotprompt.DataArgument{
		Input: map[string]any{
			"userInput": task,
		},
		Context: map[string]any{
			"agentName": agentName,
		},
	}
	
	// Render the template
	renderedPrompt, err := dp.Render(dotpromptContent, data, nil)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}
	
	// Convert to string (simplified - in practice you might want to handle messages differently)
	var result strings.Builder
	for _, msg := range renderedPrompt.Messages {
		result.WriteString(fmt.Sprintf("[%s]: ", msg.Role))
		if len(msg.Content) > 0 && msg.Content[0] != nil {
			result.WriteString(fmt.Sprintf("%v", msg.Content[0]))
		}
		result.WriteString("\n")
	}
	
	return result.String(), nil
}

// getActiveModelFromConfig returns the active AI model from configuration
func (e *GenKitExecutor) getActiveModelFromConfig() string {
	// TODO: Load from actual Station config
	// For now, return a reasonable default
	
	// Fallback if Station config not available
	return "gemini-1.5-flash"
}

// renderTemplate performs basic template variable substitution
func (e *GenKitExecutor) renderTemplate(template string, variables map[string]interface{}) (string, error) {
	rendered := template
	
	// Basic variable substitution (for backward compatibility)
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		rendered = strings.ReplaceAll(rendered, placeholder, fmt.Sprintf("%v", value))
	}
	
	return rendered, nil
}

// getActiveModel returns the model name from dotprompt config or fallback
func (e *GenKitExecutor) getActiveModel(dpConfig *DotpromptConfig) string {
	if dpConfig != nil && dpConfig.Model != "" {
		return dpConfig.Model
	}
	
	// Use same fallback as getActiveModelFromConfig
	return e.getActiveModelFromConfig()
}

// isModelSupported checks if the given model is supported
func (e *GenKitExecutor) isModelSupported(dpConfig *DotpromptConfig) bool {
	if dpConfig == nil {
		return true // Default models are supported
	}
	
	// For now, assume all models are supported
	// TODO: Add actual model validation logic
	return true
}

// buildDotpromptFromAgent constructs complete dotprompt content from database agent data
func (e *GenKitExecutor) buildDotpromptFromAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, environment string) string {
	// Build tool names list
	var toolNames []string
	for _, tool := range agentTools {
		toolNames = append(toolNames, fmt.Sprintf("  - \"%s\"", tool.ToolName))
	}
	
	toolsSection := ""
	if len(toolNames) > 0 {
		toolsSection = fmt.Sprintf("tools:\n%s\n", strings.Join(toolNames, "\n"))
	}
	
	// Build complete dotprompt template
	return fmt.Sprintf(`---
metadata:
  name: "%s"
  description: "%s"
  environment: "%s"
model: gpt-4o-mini
max_steps: %d
%s---

{{role "system"}}
%s

{{role "user"}}
{{userInput}}`,
		agent.Name,
		agent.Description,
		environment,
		agent.MaxSteps,
		toolsSection,
		agent.Prompt)
}

// extractPromptContent extracts the prompt content from dotprompt format
func (e *GenKitExecutor) extractPromptContent(dotpromptContent string) (string, error) {
	// Remove frontmatter if present
	if strings.HasPrefix(strings.TrimSpace(dotpromptContent), "---") {
		parts := strings.SplitN(dotpromptContent, "---", 3)
		if len(parts) >= 3 {
			return strings.TrimSpace(parts[2]), nil
		}
	}
	
	return strings.TrimSpace(dotpromptContent), nil
}

// isMultiRolePrompt checks if prompt uses multi-role syntax
func (e *GenKitExecutor) isMultiRolePrompt(prompt string) bool {
	return strings.Contains(prompt, "{{role")
}

// convertDotpromptToGenkitMessages converts dotprompt messages to GenKit messages
func (e *GenKitExecutor) convertDotpromptToGenkitMessages(dotpromptMessages []dotprompt.Message) ([]*ai.Message, error) {
	var genkitMessages []*ai.Message
	
	for _, dpMsg := range dotpromptMessages {
		var role ai.Role
		switch dpMsg.Role {
		case "system":
			role = ai.RoleSystem
		case "user":
			role = ai.RoleUser
		case "assistant":
			role = ai.RoleModel
		default:
			continue // Skip unknown roles
		}
		
		// Extract text content from dotprompt message
		content := ""
		if len(dpMsg.Content) > 0 {
			// Try to extract text content from the first part
			firstPart := dpMsg.Content[0]
			if firstPart != nil {
				content = fmt.Sprintf("%v", firstPart)
			}
		}
		
		if content != "" {
			message := ai.NewTextMessage(role, content)
			genkitMessages = append(genkitMessages, message)
		}
	}
	
	return genkitMessages, nil
}