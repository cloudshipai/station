// Package cloudshipai provides a GenKit plugin for CloudShip AI inference endpoint.
// This plugin uses the OpenAI-compatible API at inference.cloudshipai.com with
// CloudShip registration key authentication and full tool support.
package cloudshipai

import (
	"context"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	ProviderName   = "cloudshipai"
	DefaultBaseURL = "https://inference.cloudshipai.com/v1"
)

// SupportedModels defines the models available on CloudShip inference endpoint
var SupportedModels = map[string]ai.ModelOptions{
	// Llama 3.1 models via Together AI backend
	"cloudship/llama-3.1-8b": {
		Label: "CloudShip Llama 3.1 8B",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      false,
			ToolChoice: true,
		},
	},
	"cloudship/llama-3.1-70b": {
		Label: "CloudShip Llama 3.1 70B",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      false,
			ToolChoice: true,
		},
	},
	// Qwen models via Together AI backend
	"cloudship/qwen-72b": {
		Label: "CloudShip Qwen 2.5 72B",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      false,
			ToolChoice: true,
		},
	},
}

// CloudShipAI is a GenKit plugin for CloudShip's inference endpoint
type CloudShipAI struct {
	// RegistrationKey is the CloudShip registration key for authentication
	RegistrationKey string

	// BaseURL allows overriding the inference endpoint (for testing)
	BaseURL string

	// client is the initialized OpenAI-compatible client
	client *openai.Client

	// initialized tracks if Init() has been called
	initialized bool
}

// Name implements genkit.Plugin
func (c *CloudShipAI) Name() string {
	return ProviderName
}

// Init implements genkit.Plugin - initializes the OpenAI-compatible client
func (c *CloudShipAI) Init(ctx context.Context) []api.Action {
	log.Printf("[CloudShipAI] Init called, initialized=%v", c.initialized)
	if c.initialized {
		log.Printf("[CloudShipAI] Already initialized, returning nil")
		return nil
	}

	log.Printf("[CloudShipAI] Building options...")
	var opts []option.RequestOption

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	opts = append(opts, option.WithBaseURL(baseURL))

	// Use registration key as Bearer token for authentication
	if c.RegistrationKey != "" {
		log.Printf("[CloudShipAI] Using registration key authentication")
		opts = append(opts, option.WithAPIKey(c.RegistrationKey))
	}

	log.Printf("[CloudShipAI] Creating OpenAI-compatible client for %s...", baseURL)
	client := openai.NewClient(opts...)
	c.client = &client
	c.initialized = true
	log.Printf("[CloudShipAI] Client created successfully")

	log.Printf("[CloudShipAI] Registering %d models...", len(SupportedModels))
	var actions []api.Action
	for modelID, modelOpts := range SupportedModels {
		log.Printf("[CloudShipAI] Registering model: %s", modelID)
		model := c.DefineModel(modelID, modelOpts)
		actions = append(actions, model.(api.Action))
	}

	log.Printf("[CloudShipAI] Init complete, returning %d actions", len(actions))
	return actions
}

// DefineModel creates a GenKit model backed by the CloudShip inference API
func (c *CloudShipAI) DefineModel(modelID string, opts ai.ModelOptions) ai.Model {
	return ai.NewModel(
		api.NewName(ProviderName, modelID),
		&opts,
		func(ctx context.Context, req *ai.ModelRequest, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
			generator := NewGenerator(c.client, modelID)
			return generator.Generate(ctx, req, cb)
		},
	)
}

// GetClient returns the initialized OpenAI-compatible client
func (c *CloudShipAI) GetClient() *openai.Client {
	return c.client
}
