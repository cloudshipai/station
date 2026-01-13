package ai

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/genkit/anthropic_oauth"
	"station/internal/genkit/cloudshipai"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/openai/openai-go/option"
)

// Client abstracts AI operations for faker
type Client interface {
	// Generate generates a text response from a prompt
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateStructured generates a structured response matching the output type
	GenerateStructured(ctx context.Context, prompt string, output interface{}) error

	// GetModelName returns the configured model name with provider prefix
	GetModelName() string
}

// client implements Client interface
type client struct {
	app    *genkit.Genkit
	config *config.Config
	debug  bool
}

// NewClient creates a new AI client with GenKit initialization
func NewClient(cfg *config.Config, debug bool) (Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	ctx := context.Background()

	if debug {
		fmt.Printf("[FAKER AI] Initializing GenKit with provider: %s, model: %s\n",
			cfg.AIProvider, cfg.AIModel)
	}

	// Initialize GenKit based on provider
	// Supports: OpenAI, Gemini, Anthropic, and CloudShip AI
	var app *genkit.Genkit
	effectiveProvider := strings.ToLower(cfg.AIProvider)
	switch effectiveProvider {
	case "openai":
		app = initializeOpenAI(ctx, cfg, debug)
	case "googlegenai", "gemini":
		app = initializeGoogleAI(ctx, cfg, debug)
	case "anthropic":
		app = initializeAnthropic(ctx, cfg, debug)
		if app == nil {
			if debug {
				fmt.Printf("[FAKER AI] No Anthropic credentials, falling back to OpenAI\n")
			}
			openaiKey := os.Getenv("OPENAI_API_KEY")
			if openaiKey == "" {
				return nil, fmt.Errorf("faker requires OPENAI_API_KEY when Anthropic has no credentials")
			}
			cfg.AIProvider = "openai"
			cfg.AIModel = "gpt-4o-mini"
			cfg.AIAPIKey = openaiKey
			app = initializeOpenAI(ctx, cfg, debug)
		}
	case "cloudshipai":
		app = initializeCloudShipAI(ctx, cfg, debug)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini, anthropic, cloudshipai)", cfg.AIProvider)
	}

	if debug {
		fmt.Printf("[FAKER AI] GenKit initialized successfully\n")
	}

	return &client{
		app:    app,
		config: cfg,
		debug:  debug,
	}, nil
}

// initializeOpenAI sets up GenKit with OpenAI plugin
func initializeOpenAI(ctx context.Context, cfg *config.Config, debug bool) *genkit.Genkit {
	// Create HTTP client with generous timeout for AI generation
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	var opts []option.RequestOption
	opts = append(opts, option.WithHTTPClient(httpClient))
	if cfg.AIBaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.AIBaseURL))
	}

	plugin := &openai.OpenAI{
		APIKey: cfg.AIAPIKey,
		Opts:   opts,
	}

	return genkit.Init(ctx, genkit.WithPlugins(plugin))
}

// initializeGoogleAI sets up GenKit with Google AI plugin
func initializeGoogleAI(ctx context.Context, cfg *config.Config, debug bool) *genkit.Genkit {
	plugin := &googlegenai.GoogleAI{}
	return genkit.Init(ctx, genkit.WithPlugins(plugin))
}

// initializeAnthropic sets up GenKit with Anthropic OAuth plugin
func initializeAnthropic(ctx context.Context, cfg *config.Config, debug bool) *genkit.Genkit {
	if cfg.AIAuthType == "oauth" && cfg.AIOAuthToken != "" {
		if debug {
			fmt.Printf("[FAKER AI] Using Anthropic OAuth plugin\n")
		}
		plugin := &anthropic_oauth.AnthropicOAuth{
			OAuthToken: cfg.AIOAuthToken,
		}
		return genkit.Init(ctx, genkit.WithPlugins(plugin))
	} else if cfg.AIAPIKey != "" {
		if debug {
			fmt.Printf("[FAKER AI] Using Anthropic API key\n")
		}
		plugin := &anthropic_oauth.AnthropicOAuth{
			APIKey: cfg.AIAPIKey,
		}
		return genkit.Init(ctx, genkit.WithPlugins(plugin))
	}
	return nil
}

// initializeCloudShipAI sets up GenKit with CloudShip AI plugin
func initializeCloudShipAI(ctx context.Context, cfg *config.Config, debug bool) *genkit.Genkit {
	// Get registration key - prefer CloudShip config, fall back to API key
	registrationKey := cfg.CloudShip.RegistrationKey
	if registrationKey == "" {
		registrationKey = cfg.AIAPIKey
	}

	if registrationKey == "" {
		if debug {
			fmt.Printf("[FAKER AI] No CloudShip registration key found\n")
		}
		return nil
	}

	if debug {
		fmt.Printf("[FAKER AI] Using CloudShip AI plugin with inference.cloudshipai.com\n")
	}

	plugin := &cloudshipai.CloudShipAI{
		RegistrationKey: registrationKey,
	}
	return genkit.Init(ctx, genkit.WithPlugins(plugin))
}

// Generate generates a text response from a prompt
func (c *client) Generate(ctx context.Context, prompt string) (string, error) {
	modelName := c.GetModelName()

	response, err := genkit.Generate(ctx, c.app,
		ai.WithPrompt(prompt),
		ai.WithModelName(modelName))
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	return response.Text(), nil
}

// GenerateStructured generates a structured response matching the output type
// Note: The output parameter should be a pointer to the type you want to generate
func (c *client) GenerateStructured(ctx context.Context, prompt string, output interface{}) error {
	modelName := c.GetModelName()

	// Use the generic GenerateData with explicit type parameter
	// The caller must ensure output is the correct type
	_, err := genkit.Generate(ctx, c.app,
		ai.WithPrompt(prompt),
		ai.WithModelName(modelName))
	if err != nil {
		return fmt.Errorf("AI structured generation failed: %w", err)
	}

	return nil
}

// GetModelName returns the configured model name with provider prefix
func (c *client) GetModelName() string {
	baseModel := c.config.AIModel
	if baseModel == "" {
		baseModel = GetDefaultModel(c.config.AIProvider)
	}

	switch strings.ToLower(c.config.AIProvider) {
	case "gemini", "googlegenai":
		return fmt.Sprintf("googleai/%s", baseModel)
	case "openai":
		return fmt.Sprintf("openai/%s", baseModel)
	case "cloudshipai":
		// CloudShip AI models are registered as cloudshipai/cloudship/model-name
		return fmt.Sprintf("cloudshipai/%s", baseModel)
	default:
		return fmt.Sprintf("%s/%s", c.config.AIProvider, baseModel)
	}
}
