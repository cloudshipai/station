package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	fakerCreateEnv         string
	fakerCreateName        string
	fakerCreateInstruction string
	fakerCreateOffline     bool
	fakerCreateDebug       bool
	fakerCreateNoSync      bool
)

var fakerCreateCmd = &cobra.Command{
	Use:   "create <mcp-server-name>",
	Short: "Create a faker version of an existing MCP server",
	Long: `Create a faker-wrapped version of an existing MCP server in the same environment.

This command reads the configuration of an existing MCP server from the environment's
template.json, wraps it in a faker proxy, and adds it back to the same template.json
with a new name. This allows you to have both the real and simulated versions of the
same MCP server in the environment.

Examples:
  # Create a faker from existing server
  stn faker create aws-cloudwatch \
    --env production \
    --instruction "Simulate high-traffic production environment" \
    --name aws-cloudwatch-faker

  # Create a faker with debugging
  stn faker create stripe-payments \
    --env staging \
    --instruction "Simulate payment processing with various scenarios" \
    --name stripe-faker \
    --debug`,
	Args: cobra.ExactArgs(1),
	RunE: runFakerCreate,
}

func init() {
	fakerCreateCmd.Flags().StringVar(&fakerCreateEnv, "env", "", "Environment name (required)")
	fakerCreateCmd.Flags().StringVar(&fakerCreateName, "name", "", "Name for the faker server (default: <server>-faker)")
	fakerCreateCmd.Flags().StringVar(&fakerCreateInstruction, "instruction", "", "AI instruction for data generation (required)")
	fakerCreateCmd.Flags().BoolVar(&fakerCreateOffline, "offline", false, "Offline mode (no real MCP server connection)")
	fakerCreateCmd.Flags().BoolVar(&fakerCreateDebug, "debug", false, "Enable debug logging for faker")
	fakerCreateCmd.Flags().BoolVar(&fakerCreateNoSync, "no-sync", false, "Don't automatically sync environment after creation")

	fakerCreateCmd.MarkFlagRequired("env")
	fakerCreateCmd.MarkFlagRequired("instruction")

	fakerCmd.AddCommand(fakerCreateCmd)
}

// TemplateConfig represents the structure of template.json
type TemplateConfig struct {
	Name        string                       `json:"name,omitempty"`
	Description string                       `json:"description,omitempty"`
	MCPServers  map[string]MCPServerTemplate `json:"mcpServers"`
}

// MCPServerTemplate represents an MCP server configuration in template.json
type MCPServerTemplate struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func runFakerCreate(cmd *cobra.Command, args []string) error {
	sourceMCPName := args[0]

	// Default faker name if not provided
	if fakerCreateName == "" {
		fakerCreateName = sourceMCPName + "-faker"
	}

	// Get environment directory
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}
	envDir := filepath.Join(configHome, "station", "environments", fakerCreateEnv)

	// Check environment exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' does not exist at %s", fakerCreateEnv, envDir)
	}

	templatePath := filepath.Join(envDir, "template.json")

	// Read existing template.json
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template.json: %w", err)
	}

	var config TemplateConfig
	if err := json.Unmarshal(templateData, &config); err != nil {
		return fmt.Errorf("failed to parse template.json: %w", err)
	}

	// Check if source MCP server exists
	sourceMCP, exists := config.MCPServers[sourceMCPName]
	if !exists {
		availableServers := make([]string, 0, len(config.MCPServers))
		for name := range config.MCPServers {
			availableServers = append(availableServers, name)
		}
		return fmt.Errorf("MCP server '%s' not found in environment '%s'\nAvailable servers: %s",
			sourceMCPName, fakerCreateEnv, strings.Join(availableServers, ", "))
	}

	// Check if faker name already exists
	if _, exists := config.MCPServers[fakerCreateName]; exists {
		return fmt.Errorf("MCP server '%s' already exists in environment '%s'", fakerCreateName, fakerCreateEnv)
	}

	// Get stn binary path (use current binary)
	stnPath, err := os.Executable()
	if err != nil {
		stnPath = "stn" // Fallback to PATH lookup
	}

	// Build faker MCP server configuration
	fakerMCP := MCPServerTemplate{
		Command: stnPath,
		Args:    buildFakerArgs(sourceMCP, fakerCreateInstruction, fakerCreateOffline, fakerCreateDebug),
	}

	// Add faker to config
	config.MCPServers[fakerCreateName] = fakerMCP

	// Write updated template.json
	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template.json: %w", err)
	}

	if err := os.WriteFile(templatePath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write template.json: %w", err)
	}

	fmt.Printf("✓ Created faker '%s' in environment '%s'\n", fakerCreateName, fakerCreateEnv)
	fmt.Printf("  Source: %s\n", sourceMCPName)
	fmt.Printf("  Faker: %s\n", fakerCreateName)
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Command: %s\n", fakerMCP.Command)
	fmt.Printf("  Instruction: %s\n", fakerCreateInstruction)

	// Sync environment if not disabled
	if !fakerCreateNoSync {
		fmt.Printf("\nSyncing environment to discover faker tools...\n")
		// TODO: Call sync command or service
		fmt.Printf("Run: stn sync %s\n", fakerCreateEnv)
	}

	return nil
}

// buildFakerArgs constructs the faker command arguments from source MCP config
func buildFakerArgs(sourceMCP MCPServerTemplate, instruction string, offline bool, debug bool) []string {
	args := []string{"faker"}

	// Add --command flag
	args = append(args, "--command", sourceMCP.Command)

	// Add --args flag if source has args
	if len(sourceMCP.Args) > 0 {
		argsStr := strings.Join(sourceMCP.Args, ",")
		args = append(args, "--args", argsStr)
	}

	// Add environment variables
	for key, value := range sourceMCP.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	// Add AI instruction
	args = append(args, "--ai-enabled")
	args = append(args, "--ai-instruction", instruction)

	// Add optional flags
	if offline {
		args = append(args, "--passthrough")
	}
	if debug {
		args = append(args, "--debug")
	}

	return args
}
