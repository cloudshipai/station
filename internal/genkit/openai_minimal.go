// Minimal Station OpenAI plugin - contains only essential fixes and functionality
// This is the clean, maintainable version that focuses on the core bug fix
package genkit

import (
	"context"
	"fmt"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/genkit"
	openaiGo "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const minimalProvider = "openai-minimal"

// MinimalStationOpenAI provides a clean OpenAI plugin with only essential Station fixes
type MinimalStationOpenAI struct {
	APIKey  string
	BaseURL string
	Opts    []option.RequestOption
	
	// LogCallback for integration with Station's execution logging layer
	LogCallback func(map[string]interface{})
	
	client *openaiGo.Client
}

// Name implements genkit.Plugin
func (o *MinimalStationOpenAI) Name() string {
	return minimalProvider
}

// Init implements genkit.Plugin
func (o *MinimalStationOpenAI) Init(ctx context.Context, g *genkit.Genkit) error {
	apiKey := o.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		return fmt.Errorf("minimal openai plugin: OPENAI_API_KEY required")
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if o.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(o.BaseURL))
	}
	if len(o.Opts) > 0 {
		opts = append(opts, o.Opts...)
	}

	client := openaiGo.NewClient(opts...)
	o.client = &client

	// Define essential models with tool support
	models := map[string]ai.ModelInfo{
		"gpt-4o": {
			Label:    "OpenAI GPT-4o (Station)",
			Supports: &multimodalSupports,
			Versions: []string{"gpt-4o", "gpt-4o-2024-11-20"},
		},
		"gpt-4o-mini": {
			Label:    "OpenAI GPT-4o-mini (Station)",
			Supports: &multimodalSupports,
			Versions: []string{"gpt-4o-mini", "gpt-4o-mini-2024-07-18"},
		},
	}

	for modelID, info := range models {
		if _, err := o.defineModel(g, modelID, info); err != nil {
			return fmt.Errorf("failed to define model %s: %w", modelID, err)
		}
	}

	return nil
}

// Model supports for Station's use cases
var multimodalSupports = ai.ModelSupports{
	Multiturn:  true,
	Tools:      true,
	SystemRole: true,
	Media:      true,
	ToolChoice: true,
}

// defineModel creates a model with Station's essential fixes
func (o *MinimalStationOpenAI) defineModel(g *genkit.Genkit, modelID string, info ai.ModelInfo) (ai.Model, error) {
	return genkit.DefineModel(g, minimalProvider, modelID, &info, func(
		ctx context.Context,
		input *ai.ModelRequest,
		cb func(context.Context, *ai.ModelResponseChunk) error,
	) (*ai.ModelResponse, error) {
		
		// Create minimal generator with our essential fix
		generator := NewMinimalModelGenerator(o.client, modelID, o.LogCallback)
		generator = generator.WithMessages(input.Messages).WithConfig(input.Config).WithTools(input.Tools)
		
		return generator.Generate(ctx, cb)
	}), nil
}

// SetLogCallback allows Station's execution layer to receive logs
func (o *MinimalStationOpenAI) SetLogCallback(callback func(map[string]interface{})) {
	o.LogCallback = callback
}

// ListActions and ResolveAction for plugin compatibility
func (o *MinimalStationOpenAI) ListActions(ctx context.Context) []core.ActionDesc {
	return []core.ActionDesc{} // Minimal implementation
}

func (o *MinimalStationOpenAI) ResolveAction(g *genkit.Genkit, atype core.ActionType, name string) error {
	return nil // Minimal implementation
}