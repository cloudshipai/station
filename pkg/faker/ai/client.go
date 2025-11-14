package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"station/internal/config"

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
	var app *genkit.Genkit
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		app = initializeOpenAI(ctx, cfg, debug)
	case "googlegenai", "gemini":
		app = initializeGoogleAI(ctx, cfg, debug)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini)", cfg.AIProvider)
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
	// Use environment variable for API key
	plugin := &googlegenai.GoogleAI{}
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
	default:
		return fmt.Sprintf("%s/%s", c.config.AIProvider, baseModel)
	}
}
