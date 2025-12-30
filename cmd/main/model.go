package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"station/internal/config"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage AI model selection",
	Long: `View and change the AI model used by Station.

Shows available models for your current AI provider and allows
switching between them.

Examples:
  stn model           # Show current model and list available models
  stn model list      # List all available models
  stn model set <id>  # Set a specific model`,
	RunE: runModelList,
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	RunE:  runModelList,
}

var modelSetCmd = &cobra.Command{
	Use:   "set [model-id]",
	Short: "Set the AI model",
	Long: `Set the AI model to use. If no model ID is provided,
an interactive selection menu will be shown.

Examples:
  stn model set claude-opus-4-5-20251101
  stn model set   # Interactive selection`,
	RunE: runModelSet,
}

func init() {
	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelSetCmd)
	rootCmd.AddCommand(modelCmd)
}

func runModelList(cmd *cobra.Command, args []string) error {
	provider := viper.GetString("ai_provider")
	currentModel := viper.GetString("ai_model")
	authType := viper.GetString("ai_auth_type")

	if provider == "" {
		provider = "openai"
	}

	fmt.Printf("Current provider: %s\n", provider)
	fmt.Printf("Current model: %s\n", currentModel)
	fmt.Println()

	switch provider {
	case "anthropic":
		if authType == "oauth" {
			fmt.Println("Available models (Claude Max/Pro subscription):")
			fmt.Println()
			models := GetAnthropicOAuthModels()
			for _, model := range models {
				marker := "  "
				if model.ID == currentModel {
					marker = "* "
				}
				defaultTag := ""
				if model.Default {
					defaultTag = " (recommended)"
				}
				fmt.Printf("%s%s%s\n", marker, model.Name, defaultTag)
				fmt.Printf("    %s\n", model.Description)
				fmt.Printf("    ID: %s\n", model.ID)
				fmt.Println()
			}
		} else {
			fmt.Println("Available models (API key):")
			fmt.Println("  claude-sonnet-4-20250514 (recommended)")
			fmt.Println("  claude-opus-4-20250514")
			fmt.Println("  claude-haiku-4-5-20251001")
			fmt.Println()
			fmt.Println("For Claude Max/Pro models (Opus 4.5, Sonnet 4.5), run:")
			fmt.Println("  stn auth anthropic login")
		}
	case "openai":
		fmt.Println("Available models:")
		for _, model := range config.GetSupportedOpenAIModels() {
			marker := "  "
			if model == currentModel {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, model)
		}
	case "gemini":
		fmt.Println("Available models:")
		fmt.Println("  gemini-2.5-flash (recommended)")
		fmt.Println("  gemini-2.5-pro")
		fmt.Println("  gemini-2.0-flash")
	default:
		fmt.Printf("Model listing not available for provider: %s\n", provider)
		fmt.Println("You can set any model ID directly with: stn model set <model-id>")
	}

	fmt.Println()
	fmt.Println("To change models: stn model set <model-id>")

	return nil
}

func runModelSet(cmd *cobra.Command, args []string) error {
	provider := viper.GetString("ai_provider")
	authType := viper.GetString("ai_auth_type")

	if provider == "" {
		provider = "openai"
	}

	var selectedModel string

	if len(args) > 0 {
		selectedModel = args[0]
	} else {
		reader := bufio.NewReader(os.Stdin)

		switch provider {
		case "anthropic":
			if authType == "oauth" {
				selectedModel = selectAnthropicModel(reader)
			} else {
				selectedModel = selectAnthropicAPIModel(reader)
			}
		case "openai":
			selectedModel = selectOpenAIModel(reader)
		default:
			fmt.Printf("Interactive selection not available for provider: %s\n", provider)
			fmt.Println("Please specify model ID: stn model set <model-id>")
			return nil
		}
	}

	viper.Set("ai_model", selectedModel)

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = config.GetConfigRoot() + "/config.yaml"
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Model set to: %s\n", selectedModel)

	return nil
}

func selectAnthropicAPIModel(reader *bufio.Reader) string {
	models := []struct {
		ID   string
		Name string
	}{
		{"claude-sonnet-4-20250514", "Claude Sonnet 4 (recommended)"},
		{"claude-opus-4-20250514", "Claude Opus 4"},
		{"claude-haiku-4-5-20251001", "Claude Haiku 4.5"},
	}

	fmt.Println("Select a model:")
	fmt.Println()

	for i, model := range models {
		fmt.Printf("[%d] %s\n", i+1, model.Name)
		fmt.Printf("    ID: %s\n", model.ID)
		fmt.Println()
	}

	fmt.Printf("Enter number (1-%d): ", len(models))

	input, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(input) == "" {
		return models[0].ID
	}

	choice := 0
	_, err = fmt.Sscanf(strings.TrimSpace(input), "%d", &choice)
	if err != nil || choice < 1 || choice > len(models) {
		fmt.Printf("Invalid selection, using default: %s\n", models[0].ID)
		return models[0].ID
	}

	return models[choice-1].ID
}

func selectOpenAIModel(reader *bufio.Reader) string {
	models := config.GetSupportedOpenAIModels()

	fmt.Println("Select a model:")
	fmt.Println()

	for i, model := range models {
		fmt.Printf("[%d] %s\n", i+1, model)
	}

	fmt.Println()
	fmt.Printf("Enter number (1-%d): ", len(models))

	input, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(input) == "" {
		return models[0]
	}

	choice := 0
	_, err = fmt.Sscanf(strings.TrimSpace(input), "%d", &choice)
	if err != nil || choice < 1 || choice > len(models) {
		fmt.Printf("Invalid selection, using default: %s\n", models[0])
		return models[0]
	}

	return models[choice-1]
}
