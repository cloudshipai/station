package dotprompt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"gopkg.in/yaml.v3"
)

// DotPromptConfig represents the YAML frontmatter in a .prompt file
type DotPromptConfig struct {
	Model       string                 `yaml:"model"`
	Config      map[string]interface{} `yaml:"config"`
	Metadata    map[string]interface{} `yaml:"metadata"`
	Tools       []string               `yaml:"tools"`
	Station     map[string]interface{} `yaml:"station"`
	Input       map[string]interface{} `yaml:"input"`
	Output      map[string]interface{} `yaml:"output"`
}

// Handler handles dotprompt operations
type Handler struct{}

// NewHandler creates a new dotprompt handler
func NewHandler() *Handler {
	return &Handler{}
}

// ParseDotPrompt parses a .prompt file with YAML frontmatter and prompt content
func (h *Handler) ParseDotPrompt(content string) (*DotPromptConfig, string, error) {
	// Split on the first occurrence of "---" after the initial "---"
	parts := strings.Split(content, "---")
	if len(parts) < 3 {
		// No frontmatter, treat entire content as prompt
		return &DotPromptConfig{}, content, nil
	}
	
	// The YAML frontmatter is parts[1], the prompt content starts from parts[2]
	yamlContent := strings.TrimSpace(parts[1])
	promptContent := strings.TrimSpace(strings.Join(parts[2:], "---"))
	
	var config DotPromptConfig
	if yamlContent != "" {
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w", err)
		}
	}
	
	return &config, promptContent, nil
}

// LoadAgentPrompts loads all .prompt files from the specified agents directory
func (h *Handler) LoadAgentPrompts(ctx context.Context, genkitApp *genkit.Genkit, agentsDir, environment string) (int, error) {
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("agents directory does not exist: %s", agentsDir)
	}
	
	promptCount := 0
	err := filepath.Walk(agentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Only process .prompt files
		if !strings.HasSuffix(info.Name(), ".prompt") {
			return nil
		}
		
		// Get the agent name from filename (without .prompt extension)
		agentName := strings.TrimSuffix(info.Name(), ".prompt")
		
		// Read the prompt file
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to read prompt file %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		// Parse the dotprompt format (YAML frontmatter + prompt content)
		promptConfig, promptContent, err := h.ParseDotPrompt(string(content))
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to parse prompt file %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		// Log what we parsed for debugging
		fmt.Printf("   📝 Parsed prompt: %s (model: %s, %d tools)\n", agentName, promptConfig.Model, len(promptConfig.Tools))
		
		// Build GenKit prompt options with parsed content
		promptOptions := []ai.PromptOption{
			ai.WithPrompt(promptContent),
		}
		
		// Log the model from frontmatter for debugging
		if promptConfig.Model != "" {
			fmt.Printf("   🎯 Prompt specifies model: %s\n", promptConfig.Model)
		}
		
		// Add input schema matching the prompt template variables
		inputType := struct {
			TASK        string `json:"TASK" jsonschema:"description=The specific task or instruction for the agent to perform"`
			ENVIRONMENT string `json:"ENVIRONMENT" jsonschema:"description=The environment context (e.g., dev, staging, production)"`
		}{
			TASK:        "Please analyze and explore the file structure within the allowed directories",
			ENVIRONMENT: environment,
		}
		
		if promptConfig.Input != nil && len(promptConfig.Input) > 0 {
			// TODO: Parse the frontmatter input schema properly
			// For now, use the default template variable schema
			promptOptions = append(promptOptions, ai.WithInputType(inputType))
		} else {
			// Default input matches Station prompt template variables ({{TASK}}, {{ENVIRONMENT}})
			promptOptions = append(promptOptions, ai.WithInputType(inputType))
		}
		
		// Add output schema if defined in frontmatter
		if promptConfig.Output != nil && len(promptConfig.Output) > 0 {
			// Create output type - for most Station agents, this should be a string response
			promptOptions = append(promptOptions, ai.WithOutputType(""))
		} else {
			// Default to string output to avoid schema errors
			promptOptions = append(promptOptions, ai.WithOutputType(""))
		}
		
		// Define the prompt in GenKit with proper configuration
		_, err = genkit.DefinePrompt(genkitApp, agentName, promptOptions...)
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to define prompt %s: %v\n", agentName, err)
			return nil // Continue with other files
		}
		
		fmt.Printf("   ✅ Agent Prompt: %s\n", agentName)
		promptCount++
		return nil
	})
	
	return promptCount, err
}