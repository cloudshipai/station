package main

import (
	"fmt"
	"sort"

	"station/internal/config"

	"github.com/spf13/cobra"
)

var fakerTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List available faker templates",
	Long: `List all available faker templates (built-in and custom).

Templates provide pre-configured AI instructions for common use cases
like AWS cost management, GCP billing, Datadog monitoring, etc.

Examples:
  stn faker templates
  stn faker templates | grep aws`,
	RunE: runFakerTemplates,
}

func init() {
	fakerCmd.AddCommand(fakerTemplatesCmd)
}

func runFakerTemplates(cmd *cobra.Command, args []string) error {
	// Load config to get templates
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.FakerTemplates) == 0 {
		fmt.Println("No faker templates available.")
		return nil
	}

	fmt.Println("Available Faker Templates:\n")

	// Sort template keys for consistent output
	var keys []string
	for key := range cfg.FakerTemplates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Print templates
	for _, key := range keys {
		template := cfg.FakerTemplates[key]
		fmt.Printf("  %s\n", key)
		fmt.Printf("    Name: %s\n", template.Name)
		fmt.Printf("    Description: %s\n", template.Description)
		fmt.Printf("    Model: %s\n", template.Model)
		fmt.Printf("    Instruction: %s\n", truncateString(template.Instruction, 100))
		fmt.Println()
	}

	fmt.Printf("Total: %d templates\n\n", len(cfg.FakerTemplates))
	fmt.Println("Usage:")
	fmt.Println("  stn faker create <name> --template <template-key> --env <environment>")
	fmt.Println("\nExample:")
	fmt.Println("  stn faker create aws-cost --template aws-finops --env production")

	return nil
}
