// Package anthropic_oauth provides a custom GenKit plugin for Anthropic with OAuth support.
// This plugin uses the native Anthropic Messages API (not OpenAI-compatible) and supports
// full tool calling functionality with OAuth authentication.
package anthropic_oauth

import (
	"context"
	"log"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
)

const (
	ProviderName   = "anthropic"
	DefaultBaseURL = "https://api.anthropic.com"
)

// SupportedModels defines the models available with full tool support
var SupportedModels = map[string]ai.ModelOptions{
	// Claude 4 Opus models
	"claude-opus-4-5-20251101": {
		Label: "Claude Opus 4.5",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
	"claude-opus-4-20250514": {
		Label: "Claude Opus 4",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
	// Claude 4 Sonnet models
	"claude-sonnet-4-5-20250929": {
		Label: "Claude Sonnet 4.5",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
	"claude-sonnet-4-20250514": {
		Label: "Claude Sonnet 4",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
	// Claude 4 Haiku models
	"claude-haiku-4-5-20251001": {
		Label: "Claude Haiku 4.5",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
	// Claude 3.5 models (legacy)
	"claude-3-5-sonnet-20241022": {
		Label: "Claude 3.5 Sonnet",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
	"claude-3-5-haiku-20241022": {
		Label: "Claude 3.5 Haiku",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
	// Claude 3 models (legacy)
	"claude-3-opus-20240229": {
		Label: "Claude 3 Opus",
		Supports: &ai.ModelSupports{
			Multiturn:  true,
			Tools:      true,
			SystemRole: true,
			Media:      true,
			ToolChoice: true,
		},
	},
}

// AnthropicOAuth is a GenKit plugin for Anthropic with OAuth authentication
type AnthropicOAuth struct {
	// OAuthToken is the Bearer token for OAuth authentication
	OAuthToken string

	// APIKey is the standard API key (mutually exclusive with OAuthToken)
	APIKey string

	// BaseURL allows overriding the Anthropic API endpoint
	BaseURL string

	// client is the initialized Anthropic client
	client anthropic.Client

	// initialized tracks if Init() has been called
	initialized bool
}

// Name implements genkit.Plugin
func (a *AnthropicOAuth) Name() string {
	return ProviderName
}

// Init implements genkit.Plugin - initializes the Anthropic client
func (a *AnthropicOAuth) Init(ctx context.Context) []api.Action {
	log.Printf("[AnthropicOAuth] Init called, initialized=%v", a.initialized)
	if a.initialized {
		log.Printf("[AnthropicOAuth] Already initialized, returning nil")
		return nil
	}

	log.Printf("[AnthropicOAuth] Building options...")
	var opts []option.RequestOption

	baseURL := a.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	opts = append(opts, option.WithBaseURL(baseURL))

	if a.OAuthToken != "" {
		log.Printf("[AnthropicOAuth] Using OAuth token authentication")
		opts = append(opts,
			option.WithHeader("Authorization", "Bearer "+a.OAuthToken),
			option.WithHeader("anthropic-beta", "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14"),
		)
		opts = append(opts, option.WithAPIKey(""))
	} else if a.APIKey != "" {
		log.Printf("[AnthropicOAuth] Using API key authentication")
		opts = append(opts, option.WithAPIKey(a.APIKey))
	}

	log.Printf("[AnthropicOAuth] Creating Anthropic client...")
	a.client = anthropic.NewClient(opts...)
	a.initialized = true
	log.Printf("[AnthropicOAuth] Client created successfully")

	log.Printf("[AnthropicOAuth] Registering %d models...", len(SupportedModels))
	var actions []api.Action
	for modelID, modelOpts := range SupportedModels {
		log.Printf("[AnthropicOAuth] Registering model: %s", modelID)
		model := a.DefineModel(modelID, modelOpts)
		actions = append(actions, model.(api.Action))
	}

	log.Printf("[AnthropicOAuth] Init complete, returning %d actions", len(actions))
	return actions
}

// DefineModel creates a GenKit model backed by the Anthropic Messages API
func (a *AnthropicOAuth) DefineModel(modelID string, opts ai.ModelOptions) ai.Model {
	useOAuth := a.OAuthToken != ""
	return ai.NewModel(
		api.NewName(ProviderName, modelID),
		&opts,
		func(ctx context.Context, req *ai.ModelRequest, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
			generator := NewGenerator(&a.client, modelID)
			if useOAuth {
				generator = generator.WithClaudeCodeSystemPrompt()
			}
			return generator.Generate(ctx, req, cb)
		},
	)
}

// GetClient returns the initialized Anthropic client
func (a *AnthropicOAuth) GetClient() *anthropic.Client {
	return &a.client
}
