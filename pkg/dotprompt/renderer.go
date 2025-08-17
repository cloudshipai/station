package dotprompt

import (
	"fmt"
	"strings"
	
	"github.com/google/dotprompt/go/dotprompt"
)

// Renderer handles rendering of multi-role dotprompt templates
type Renderer struct {
	parser *Parser
}

// NewRenderer creates a new dotprompt renderer
func NewRenderer() *Renderer {
	return &Renderer{
		parser: NewParser(),
	}
}

// Render renders a dotprompt template with the given context using the actual dotprompt library
func (r *Renderer) Render(content string, context *RenderContext) (*ParsedPrompt, error) {
	// 1. Create dotprompt instance
	dp := dotprompt.NewDotprompt(nil)
	
	// 2. Prepare data for rendering using dotprompt's DataArgument structure
	data := &dotprompt.DataArgument{
		Input: map[string]any{
			"userInput": context.UserInput,
		},
		Context: map[string]any{
			"agentName":   context.AgentName,
			"environment": context.Environment,
		},
	}
	
	// Add user-defined variables to Input
	for key, value := range context.UserVariables {
		data.Input[key] = value
	}
	
	// 3. Render using the actual dotprompt library
	rendered, err := dp.Render(content, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to render dotprompt: %w", err)
	}
	
	// 4. Convert rendered result to our ParsedPrompt format
	messages := make([]*Message, len(rendered.Messages))
	for i, msg := range rendered.Messages {
		// Extract text content from message parts
		var textContent strings.Builder
		for _, part := range msg.Content {
			if textPart, ok := part.(*dotprompt.TextPart); ok {
				textContent.WriteString(textPart.Text)
			}
		}
		
		messages[i] = &Message{
			Role:    string(msg.Role),
			Content: textContent.String(),
		}
	}
	
	// 5. Parse the config from the rendered prompt (for completeness)
	parsed, err := r.parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	return &ParsedPrompt{
		Config:      parsed.Config,
		Messages:    messages,
		IsMultiRole: len(messages) > 1,
	}, nil
}


// RenderToGenkit converts rendered messages to format compatible with Genkit ai.Generate
func (r *Renderer) RenderToGenkit(parsed *ParsedPrompt) (string, error) {
	if !parsed.IsMultiRole {
		// Single message - just return the content
		if len(parsed.Messages) > 0 {
			return parsed.Messages[0].Content, nil
		}
		return "", nil
	}

	// Multi-role - combine messages with role labels for now
	// TODO: Later we can use proper ai.Generate with conversation messages
	var result strings.Builder
	for i, msg := range parsed.Messages {
		if i > 0 {
			result.WriteString("\n\n")
		}
		result.WriteString(fmt.Sprintf("[%s]: %s", msg.Role, msg.Content))
	}

	return result.String(), nil
}

// ValidateTemplate validates that a template can be parsed and rendered
func (r *Renderer) ValidateTemplate(content string, sampleContext *RenderContext) error {
	// Try to parse the template
	parsed, err := r.parser.Parse(content)
	if err != nil {
		return fmt.Errorf("template parsing failed: %w", err)
	}

	// Validate messages
	if err := r.parser.ValidateMessages(parsed.Messages); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	// Try to render with sample context
	if sampleContext != nil {
		_, err := r.Render(content, sampleContext)
		if err != nil {
			return fmt.Errorf("template rendering failed: %w", err)
		}
	}

	return nil
}