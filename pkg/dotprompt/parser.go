package dotprompt

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// Parser handles parsing of dotprompt files with multi-role support
type Parser struct{}

// NewParser creates a new dotprompt parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a dotprompt content string into config and messages
func (p *Parser) Parse(content string) (*ParsedPrompt, error) {
	content = strings.TrimSpace(content)
	
	// Check if this is a dotprompt with frontmatter
	if !p.isDotpromptContent(content) {
		// Simple text prompt - convert to single system message
		return &ParsedPrompt{
			Config: &DotpromptConfig{},
			Messages: []*Message{
				{
					Role:    "system",
					Content: content,
				},
			},
			IsMultiRole: false,
		}, nil
	}

	// Split frontmatter and content
	config, promptContent, err := p.parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Parse messages from content
	messages, isMultiRole, err := p.parseMessages(promptContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse messages: %w", err)
	}

	return &ParsedPrompt{
		Config:      config,
		Messages:    messages,
		IsMultiRole: isMultiRole,
	}, nil
}

// isDotpromptContent checks if the content has YAML frontmatter or multi-role syntax
func (p *Parser) isDotpromptContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	
	// Check for YAML frontmatter markers
	hasFrontmatter := strings.HasPrefix(trimmed, "---") && 
		   strings.Contains(content, "\n---\n")
		   
	// Check for multi-role dotprompt syntax
	hasMultiRole := strings.Contains(content, "{{role \"") || strings.Contains(content, "{{role '")
	
	return hasFrontmatter || hasMultiRole
}

// parseFrontmatter extracts and parses the YAML frontmatter
func (p *Parser) parseFrontmatter(content string) (*DotpromptConfig, string, error) {
	// Find the frontmatter boundaries
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, "", fmt.Errorf("invalid frontmatter format")
	}

	// Find the end of frontmatter
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return nil, "", fmt.Errorf("frontmatter not properly closed")
	}

	// Extract frontmatter YAML
	frontmatterYAML := strings.Join(lines[1:endIndex], "\n")
	promptContent := strings.Join(lines[endIndex+1:], "\n")

	// Parse the YAML
	var config DotpromptConfig
	if err := yaml.Unmarshal([]byte(frontmatterYAML), &config); err != nil {
		return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	return &config, promptContent, nil
}

// parseMessages parses the prompt content into role-based messages
func (p *Parser) parseMessages(content string) ([]*Message, bool, error) {
	content = strings.TrimSpace(content)
	
	// Check for role directives
	rolePattern := regexp.MustCompile(`\{\{role\s+"([^"]+)"\}\}`)
	matches := rolePattern.FindAllStringSubmatch(content, -1)
	
	if len(matches) == 0 {
		// No role directives - treat as single system message
		return []*Message{
			{
				Role:    "system",
				Content: content,
			},
		}, false, nil
	}

	// Split content by role directives
	parts := rolePattern.Split(content, -1)
	if len(parts) != len(matches)+1 {
		return nil, false, fmt.Errorf("failed to split content by role directives")
	}

	var messages []*Message

	// Process each role section
	for i, match := range matches {
		role := match[1] // Extract role from regex match
		partIndex := i + 1 // Parts are offset by 1 from matches
		if partIndex < len(parts) {
			content := strings.TrimSpace(parts[partIndex])
			if content != "" {
				messages = append(messages, &Message{
					Role:    role,
					Content: content,
				})
			}
		}
	}

	return messages, true, nil
}

// EnsureUserInputSchema ensures the input schema contains userInput field
func (p *Parser) EnsureUserInputSchema(config *DotpromptConfig) {
	if config.Input.Schema == nil {
		config.Input.Schema = make(map[string]interface{})
	}

	// Add userInput if not present
	if _, exists := config.Input.Schema["userInput"]; !exists {
		config.Input.Schema["userInput"] = map[string]interface{}{
			"type":        "string",
			"description": "The task provided by the user",
		}
	}
}

// ValidateMessages validates that messages follow proper format
func (p *Parser) ValidateMessages(messages []*Message) error {
	validRoles := map[string]bool{
		"system":    true,
		"user":      true,
		"assistant": true,
	}

	for i, msg := range messages {
		if msg == nil {
			return fmt.Errorf("message %d is nil", i)
		}
		
		if msg.Role == "" {
			return fmt.Errorf("message %d has empty role", i)
		}
		
		if !validRoles[msg.Role] {
			return fmt.Errorf("message %d has invalid role '%s', must be 'system', 'user', or 'assistant'", i, msg.Role)
		}
		
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("message %d has empty content", i)
		}
	}

	return nil
}