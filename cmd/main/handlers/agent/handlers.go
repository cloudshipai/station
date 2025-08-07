package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
	"station/cmd/main/handlers/common"
	"station/internal/theme"
	agent_bundle "station/pkg/agent-bundle"
	"station/pkg/agent-bundle/creator"
	"station/pkg/agent-bundle/manager"
	"station/pkg/agent-bundle/validator"
	"station/pkg/bundle"
)

// AgentHandler handles agent-related CLI commands
type AgentHandler struct {
	themeManager *theme.ThemeManager
}

func NewAgentHandler(themeManager *theme.ThemeManager) *AgentHandler {
	return &AgentHandler{themeManager: themeManager}
}

// RunAgentList lists all agents
func (h *AgentHandler) RunAgentList(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🤖 Agents")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	envFilter, _ := cmd.Flags().GetString("env")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing agents from: " + endpoint))
		return h.listAgentsRemote(endpoint)
	} else {
		if envFilter != "" {
			fmt.Println(styles.Info.Render(fmt.Sprintf("🏠 Listing local agents (Environment: %s)", envFilter)))
		} else {
			fmt.Println(styles.Info.Render("🏠 Listing local agents"))
		}
		return h.listAgentsLocalWithFilter(envFilter)
	}
}

// RunAgentShow shows details of a specific agent
func (h *AgentHandler) RunAgentShow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID is required")
	}

	agentID, err := common.ParseIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("🤖 Agent #%d", agentID))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Showing agent from: " + endpoint))
		return h.showAgentRemote(agentID, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Showing local agent"))
		return h.showAgentLocal(agentID)
	}
}

// RunAgentRun executes an agent
func (h *AgentHandler) RunAgentRun(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("agent ID and task are required")
	}

	agentID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	task := args[1]
	tail, _ := cmd.Flags().GetBool("tail")

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("▶️  Running Agent #%d", agentID))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Running agent on: " + endpoint))
		return h.runAgentRemote(agentID, task, endpoint, tail)
	} else {
		fmt.Println(styles.Info.Render("🏠 Running local agent"))
		return h.runAgentLocal(agentID, task, tail)
	}
}

// RunAgentDelete deletes an agent
func (h *AgentHandler) RunAgentDelete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID is required")
	}

	agentID, err := common.ParseIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("🗑️  Delete Agent #%d", agentID))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Error.Render("❌ Deleting remote agents is not currently supported"))
		return fmt.Errorf("remote agent deletion not supported")
	} else {
		fmt.Println(styles.Info.Render("🏠 Deleting local agent"))
		return h.deleteAgentLocal(agentID)
	}
}

// RunAgentCreate creates a new agent
func (h *AgentHandler) RunAgentCreate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("➕ Create Agent")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	domain, _ := cmd.Flags().GetString("domain")
	schedule, _ := cmd.Flags().GetString("schedule")
	environment, _ := cmd.Flags().GetString("env")

	if endpoint != "" {
		fmt.Println(styles.Error.Render("❌ Creating remote agents is not currently supported"))
		return fmt.Errorf("remote agent creation not supported")
	} else {
		fmt.Println(styles.Info.Render("🏠 Creating local agent"))
		return h.createAgentLocal(name, description, domain, schedule, environment)
	}
}

// RunAgentExport exports an agent to file-based config
func (h *AgentHandler) RunAgentExport(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📤 Export Agent")
	fmt.Println(banner)

	agentID, err := common.ParseIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	var environment string
	if len(args) > 1 {
		environment = args[1]
	} else {
		environment = "default"
	}

	fmt.Println(styles.Info.Render(fmt.Sprintf("🏠 Exporting agent %d to environment '%s'", agentID, environment)))
	return h.exportAgentLocal(agentID, environment)
}

// RunAgentImport imports agents from file-based configs
func (h *AgentHandler) RunAgentImport(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📥 Import Agents")
	fmt.Println(banner)

	var environment string
	if len(args) > 0 {
		environment = args[0]
	} else {
		environment = "default"
	}

	fmt.Println(styles.Info.Render(fmt.Sprintf("🏠 Importing agents from environment '%s'", environment)))
	return h.importAgentsLocal(environment)
}

// Agent Bundle Handler Methods

// RunAgentBundleCreate creates a new agent bundle
func (h *AgentHandler) RunAgentBundleCreate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🎁 Create Agent Bundle")
	fmt.Println(banner)

	bundlePath := args[0]

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	author, _ := cmd.Flags().GetString("author")
	description, _ := cmd.Flags().GetString("description")
	agentType, _ := cmd.Flags().GetString("type")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	// Validate required flags
	if author == "" {
		return fmt.Errorf("--author flag is required")
	}
	if description == "" {
		return fmt.Errorf("--description flag is required")
	}

	fmt.Println(styles.Info.Render(fmt.Sprintf("📁 Creating bundle at: %s", bundlePath)))

	// Create bundle creator with mock registry
	fs := afero.NewOsFs()
	mockRegistry := &MockBundleRegistry{}
	bundleCreator := creator.New(fs, mockRegistry)

	// Set up creation options
	opts := agent_bundle.CreateOptions{
		Name:        name,
		Author:      author,
		Description: description,
		AgentType:   agentType,
		Tags:        tags,
	}

	if err := bundleCreator.Create(bundlePath, opts); err != nil {
		fmt.Println(styles.Error.Render("❌ Failed to create bundle: " + err.Error()))
		return err
	}

	fmt.Println(styles.Success.Render("✅ Agent bundle created successfully!"))
	fmt.Println(styles.Info.Render("📝 Next steps:"))
	fmt.Println("   • Edit agent.json to configure your agent")
	fmt.Println("   • Update variables.schema.json with required variables")
	fmt.Println("   • Add MCP dependencies to manifest.json")
	fmt.Println("   • Run 'stn agent bundle validate " + bundlePath + "' to validate")
	
	return nil
}

// RunAgentBundleValidate validates an agent bundle
func (h *AgentHandler) RunAgentBundleValidate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("✅ Validate Agent Bundle")
	fmt.Println(banner)

	bundlePath := args[0]
	fmt.Println(styles.Info.Render(fmt.Sprintf("📁 Validating bundle at: %s", bundlePath)))

	// Create validator
	fs := afero.NewOsFs()
	bundleValidator := validator.New(fs)

	// Validate the bundle
	result, err := bundleValidator.Validate(bundlePath)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Validation failed: " + err.Error()))
		return err
	}

	// Display results
	if result.Valid {
		fmt.Println(styles.Success.Render("✅ Bundle is valid!"))
	} else {
		fmt.Println(styles.Error.Render("❌ Bundle validation failed"))
	}

	// Show statistics
	stats := result.Statistics
	fmt.Printf("\n📊 Bundle Statistics:\n")
	fmt.Printf("   Variables: %d total (%d required, %d optional)\n", 
		stats.TotalVariables, stats.RequiredVariables, stats.OptionalVariables)
	fmt.Printf("   MCP Dependencies: %d\n", stats.MCPDependencies)
	fmt.Printf("   Tools: %d required, %d optional\n", stats.RequiredTools, stats.OptionalTools)

	// Show errors
	if len(result.Errors) > 0 {
		fmt.Printf("\n❌ Errors (%d):\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Printf("   • %s", err.Message)
			if err.Suggestion != "" {
				fmt.Printf(" (💡 %s)", err.Suggestion)
			}
			fmt.Println()
		}
	}

	// Show warnings
	if len(result.Warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings (%d):\n", len(result.Warnings))
		for _, warning := range result.Warnings {
			fmt.Printf("   • %s", warning.Message)
			if warning.Suggestion != "" {
				fmt.Printf(" (💡 %s)", warning.Suggestion)
			}
			fmt.Println()
		}
	}

	if !result.Valid {
		return fmt.Errorf("bundle validation failed")
	}
	
	return nil
}

// RunAgentBundleInstall installs an agent bundle
func (h *AgentHandler) RunAgentBundleInstall(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📦 Install Agent Bundle")
	fmt.Println(banner)

	bundlePath := args[0]
	environment := "default"
	if len(args) > 1 {
		environment = args[1]
	}

	// Get flags
	envFlag, _ := cmd.Flags().GetString("env")
	if envFlag != "" {
		environment = envFlag
	}

	variables, _ := cmd.Flags().GetStringToString("vars")
	varsFile, _ := cmd.Flags().GetString("vars-file")
	interactive, _ := cmd.Flags().GetBool("interactive")

	fmt.Println(styles.Info.Render(fmt.Sprintf("📁 Installing bundle from: %s", bundlePath)))
	fmt.Println(styles.Info.Render(fmt.Sprintf("🌍 Target environment: %s", environment)))

	// Handle vars-file
	var fileVars map[string]interface{}
	if varsFile != "" {
		var err error
		fileVars, err = h.loadVariablesFromFile(varsFile)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Failed to load variables file: " + err.Error()))
			return err
		}
	}

	// Start with file variables (preserving types), then override with CLI variables
	variablesMap := make(map[string]interface{})
	
	// Add file variables first
	if fileVars != nil {
		for k, v := range fileVars {
			variablesMap[k] = v
		}
	}
	
	// Add/override with CLI variables (convert strings to interface{})
	for k, v := range variables {
		variablesMap[k] = v
	}

	// Handle interactive mode - prompt for missing variables
	if interactive {
		// Load bundle to get variable schema
		fs := afero.NewOsFs()
		bundleValidator := validator.New(fs)
		validationResult, err := bundleValidator.Validate(bundlePath)
		if err != nil || !validationResult.Valid {
			fmt.Println(styles.Error.Render("❌ Cannot use interactive mode with invalid bundle"))
			return fmt.Errorf("bundle validation failed")
		}

		// Load manifest to get variable schema
		manifest, err := h.loadManifestFromPath(bundlePath)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Failed to load bundle manifest: " + err.Error()))
			return err
		}

		// Prompt for missing variables
		updatedVars, err := h.promptForVariables(manifest.RequiredVariables, variablesMap, styles)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Interactive variable collection failed: " + err.Error()))
			return err
		}
		variablesMap = updatedVars
	}

	// Create manager with dependencies
	fs := afero.NewOsFs()
	bundleValidator := validator.New(fs)
	// Use mock resolver for now - in production, you'd create a real resolver with:
	// realResolver := resolver.New(toolRepo, bundleRegistry)
	mockResolver := &MockResolver{}
	bundleManager := manager.New(fs, bundleValidator, mockResolver)

	// Install the bundle
	result, err := bundleManager.Install(bundlePath, environment, variablesMap)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Installation failed: " + err.Error()))
		return err
	}

	if !result.Success {
		fmt.Println(styles.Error.Render("❌ Installation failed: " + result.Error))
		return fmt.Errorf("installation failed")
	}

	fmt.Println(styles.Success.Render("✅ Agent bundle installed successfully!"))
	fmt.Printf("🤖 Agent ID: %d\n", result.AgentID)
	fmt.Printf("📝 Agent Name: %s\n", result.AgentName)
	fmt.Printf("🌍 Environment: %s\n", result.Environment)
	fmt.Printf("🔧 Tools Installed: %d\n", result.ToolsInstalled)
	if len(result.MCPBundles) > 0 {
		fmt.Printf("📦 MCP Bundles: %v\n", result.MCPBundles)
	}

	return nil
}

// RunAgentBundleDuplicate duplicates an agent across environments
func (h *AgentHandler) RunAgentBundleDuplicate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("👥 Duplicate Agent")
	fmt.Println(banner)

	agentIDStr := args[0]
	targetEnv := args[1]

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	variables, _ := cmd.Flags().GetStringToString("vars")
	varsFile, _ := cmd.Flags().GetString("vars-file")
	interactive, _ := cmd.Flags().GetBool("interactive")

	fmt.Printf("🤖 Duplicating agent %d to environment '%s'\n", agentID, targetEnv)

	// Handle vars-file
	var fileVars map[string]interface{}
	if varsFile != "" {
		var err error
		fileVars, err = h.loadVariablesFromFile(varsFile)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Failed to load variables file: " + err.Error()))
			return err
		}
	}

	// Start with file variables (preserving types), then override with CLI variables
	variablesMap := make(map[string]interface{})
	
	// Add file variables first
	if fileVars != nil {
		for k, v := range fileVars {
			variablesMap[k] = v
		}
	}
	
	// Add/override with CLI variables (convert strings to interface{})
	for k, v := range variables {
		variablesMap[k] = v
	}

	// Interactive mode for duplicate - load original agent's schema from database
	if interactive {
		// This would require database integration to:
		// 1. Load original agent configuration  
		// 2. Determine what template it was created from (if any)
		// 3. Load the variable schema for that template
		// 4. Prompt for variables that are different or missing
		
		fmt.Println(styles.Info.Render("🎯 Interactive Duplicate Mode"))
		fmt.Println(styles.Info.Render("Loading original agent configuration..."))
		
		// For now, create a basic variable schema based on common agent properties
		basicSchema := map[string]agent_bundle.VariableSpec{
			"AGENT_NAME": {
				Type:        "string",
				Description: "Name for the duplicated agent",
				Required:    true,
			},
			"AGENT_DESCRIPTION": {
				Type:        "string", 
				Description: "Description for the duplicated agent",
				Required:    false,
			},
			"ENVIRONMENT": {
				Type:        "string",
				Description: "Target environment name",
				Required:    false,
				Default:     targetEnv,
			},
		}
		
		// Merge existing variables with interactive input
		updatedVars, err := h.promptForVariables(basicSchema, variablesMap, styles)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Interactive variable collection failed: " + err.Error()))
			return err
		}
		variablesMap = updatedVars
		
		// Update the name if provided
		if newName, exists := variablesMap["AGENT_NAME"]; exists {
			if nameStr, ok := newName.(string); ok && nameStr != "" {
				name = nameStr
			}
		}
	}

	// Create manager with dependencies
	fs := afero.NewOsFs()
	bundleValidator := validator.New(fs)
	mockResolver := &MockResolver{}
	bundleManager := manager.New(fs, bundleValidator, mockResolver)

	// Set up duplication options
	opts := agent_bundle.DuplicateOptions{
		Name:      name,
		Variables: variablesMap,
	}

	// Duplicate the agent
	result, err := bundleManager.Duplicate(agentID, targetEnv, opts)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Duplication failed: " + err.Error()))
		return err
	}

	if !result.Success {
		fmt.Println(styles.Error.Render("❌ Duplication failed: " + result.Error))
		return fmt.Errorf("duplication failed")
	}

	fmt.Println(styles.Success.Render("✅ Agent duplicated successfully!"))
	fmt.Printf("🤖 New Agent ID: %d\n", result.AgentID)
	fmt.Printf("📝 Agent Name: %s\n", result.AgentName)
	fmt.Printf("🌍 Environment: %s\n", result.Environment)

	return nil
}

// RunAgentBundleExport exports an agent as a template bundle
func (h *AgentHandler) RunAgentBundleExport(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📤 Export Agent Bundle")
	fmt.Println(banner)

	agentIDStr := args[0]
	outputPath := args[1]

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	// Get flags
	sourceEnv, _ := cmd.Flags().GetString("env")
	includeDeps, _ := cmd.Flags().GetBool("include-deps")
	includeExamples, _ := cmd.Flags().GetBool("include-examples")
	analyzeVars, _ := cmd.Flags().GetBool("analyze-vars")

	fmt.Printf("🤖 Exporting agent %d to: %s\n", agentID, outputPath)
	if sourceEnv != "" {
		fmt.Printf("🌍 Source environment: %s\n", sourceEnv)
	}

	// Create bundle creator with mock registry
	fs := afero.NewOsFs()
	mockRegistry := &MockBundleRegistry{}
	bundleCreator := creator.New(fs, mockRegistry)

	// Set up export options
	opts := agent_bundle.ExportOptions{
		IncludeDependencies: includeDeps,
		IncludeExamples:    includeExamples,
		VariableAnalysis:   analyzeVars,
		OutputFormat:       "bundle",
	}

	// Export the agent (this will require database integration)
	err = bundleCreator.ExportFromAgent(agentID, outputPath, opts)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Export failed: " + err.Error()))
		return err
	}

	fmt.Println(styles.Success.Render("✅ Agent exported successfully!"))
	fmt.Printf("📁 Bundle created at: %s\n", outputPath)
	fmt.Println(styles.Info.Render("📝 Next steps:"))
	fmt.Println("   • Review and customize the bundle files")
	fmt.Println("   • Run 'stn agent bundle validate " + outputPath + "' to validate")
	fmt.Println("   • Share or install the bundle in other environments")

	return nil
}

// MockResolver is a temporary mock for dependency resolution
type MockResolver struct{}

func (r *MockResolver) Resolve(ctx context.Context, deps []agent_bundle.MCPBundleDependency, env string) (*agent_bundle.ResolutionResult, error) {
	return &agent_bundle.ResolutionResult{
		Success: true,
		ResolvedBundles: []agent_bundle.MCPBundleRef{
			{Name: "filesystem-tools", Version: "1.0.0", Source: "registry"},
		},
		MissingBundles: []agent_bundle.MCPBundleDependency{},
		Conflicts:      []agent_bundle.ToolConflict{},
		InstallOrder:   []string{"filesystem-tools"},
	}, nil
}

func (r *MockResolver) InstallMCPBundles(ctx context.Context, bundles []agent_bundle.MCPBundleRef, env string) error {
	return nil
}

func (r *MockResolver) ValidateToolAvailability(ctx context.Context, tools []agent_bundle.ToolRequirement, env string) error {
	return nil
}

func (r *MockResolver) ResolveConflicts(conflicts []agent_bundle.ToolConflict) (*agent_bundle.ConflictResolution, error) {
	return &agent_bundle.ConflictResolution{
		Strategy:    "auto",
		Resolutions: make(map[string]string),
		Warnings:    []string{},
	}, nil
}

// MockBundleRegistry is a temporary mock for bundle registry
type MockBundleRegistry struct{}

func (r *MockBundleRegistry) List(ctx context.Context, opts bundle.ListOptions) ([]bundle.BundleManifest, error) {
	return []bundle.BundleManifest{}, nil
}

func (r *MockBundleRegistry) Get(ctx context.Context, name, version string) (*bundle.BundleManifest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *MockBundleRegistry) Download(ctx context.Context, name, version string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *MockBundleRegistry) GetVersions(ctx context.Context, name string) ([]string, error) {
	return []string{}, nil
}

// Helper functions for interactive and file-based variable handling

// loadVariablesFromFile loads variables from JSON or YAML file
func (h *AgentHandler) loadVariablesFromFile(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read variables file: %w", err)
	}

	variables := make(map[string]interface{})

	// Try JSON first, then YAML
	if err := json.Unmarshal(data, &variables); err != nil {
		if err := yaml.Unmarshal(data, &variables); err != nil {
			return nil, fmt.Errorf("failed to parse variables file as JSON or YAML: %w", err)
		}
	}

	return variables, nil
}

// loadManifestFromPath loads the agent bundle manifest from a path
func (h *AgentHandler) loadManifestFromPath(bundlePath string) (*agent_bundle.AgentBundleManifest, error) {
	manifestPath := fmt.Sprintf("%s/manifest.json", bundlePath)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest agent_bundle.AgentBundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// promptForVariables interactively prompts user for missing or undefined variables
func (h *AgentHandler) promptForVariables(schema map[string]agent_bundle.VariableSpec, existingVars map[string]interface{}, styles common.CLIStyles) (map[string]interface{}, error) {
	fmt.Println(styles.Info.Render("🎯 Interactive Variable Collection"))
	fmt.Println()

	result := make(map[string]interface{})
	// Copy existing variables
	for k, v := range existingVars {
		result[k] = v
	}

	reader := bufio.NewReader(os.Stdin)

	// Sort variables by required first, then alphabetically
	var requiredVars, optionalVars []string
	for varName, spec := range schema {
		if spec.Required {
			requiredVars = append(requiredVars, varName)
		} else {
			optionalVars = append(optionalVars, varName)
		}
	}

	// Process required variables first
	if len(requiredVars) > 0 {
		fmt.Println(styles.Info.Render("📋 Required Variables:"))
		for _, varName := range requiredVars {
			spec := schema[varName]
			if _, exists := result[varName]; !exists {
				value, err := h.promptForSingleVariable(varName, spec, reader, styles)
				if err != nil {
					return nil, err
				}
				result[varName] = value
			} else {
				fmt.Printf("   ✅ %s: %v (already provided)\n", varName, h.formatVariableValue(result[varName], spec.Sensitive))
			}
		}
		fmt.Println()
	}

	// Process optional variables
	if len(optionalVars) > 0 {
		fmt.Println(styles.Info.Render("🔧 Optional Variables (press Enter to use default):"))
		for _, varName := range optionalVars {
			spec := schema[varName]
			if _, exists := result[varName]; !exists {
				value, err := h.promptForSingleVariable(varName, spec, reader, styles)
				if err != nil {
					return nil, err
				}
				if value != nil {
					result[varName] = value
				} else if spec.Default != nil {
					result[varName] = spec.Default
					fmt.Printf("   📌 %s: %v (using default)\n", varName, h.formatVariableValue(spec.Default, spec.Sensitive))
				}
			} else {
				fmt.Printf("   ✅ %s: %v (already provided)\n", varName, h.formatVariableValue(result[varName], spec.Sensitive))
			}
		}
	}

	fmt.Println(styles.Success.Render("✅ All variables collected!"))
	return result, nil
}

// promptForSingleVariable prompts for a single variable with proper input handling
func (h *AgentHandler) promptForSingleVariable(varName string, spec agent_bundle.VariableSpec, reader *bufio.Reader, styles common.CLIStyles) (interface{}, error) {
	// Build prompt
	prompt := fmt.Sprintf("   %s", varName)
	if spec.Description != "" {
		prompt += fmt.Sprintf(" (%s)", spec.Description)
	}
	if spec.Default != nil && !spec.Required {
		prompt += fmt.Sprintf(" [default: %v]", spec.Default)
	}
	prompt += ": "

	fmt.Print(prompt)

	var input string
	var err error

	// Handle sensitive variables (passwords, API keys)
	if spec.Sensitive || strings.Contains(strings.ToLower(varName), "password") || strings.Contains(strings.ToLower(varName), "secret") || strings.Contains(strings.ToLower(varName), "key") {
		// Read password without echo
		fmt.Print("[hidden] ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, fmt.Errorf("failed to read sensitive input: %w", err)
		}
		input = string(bytePassword)
		fmt.Println() // Add newline after hidden input
	} else {
		// Read normal input
		input, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)
	}

	// Handle empty input for optional variables
	if input == "" && !spec.Required {
		return nil, nil // Will use default value
	}

	// Validate required variables
	if input == "" && spec.Required {
		fmt.Println(styles.Error.Render("   ❌ This variable is required"))
		return h.promptForSingleVariable(varName, spec, reader, styles)
	}

	// Type conversion based on variable spec
	return h.convertVariableValue(input, spec)
}

// convertVariableValue converts string input to appropriate type
func (h *AgentHandler) convertVariableValue(input string, spec agent_bundle.VariableSpec) (interface{}, error) {
	switch spec.Type {
	case "string", "secret":
		return input, nil
	case "number":
		if strings.Contains(input, ".") {
			return strconv.ParseFloat(input, 64)
		}
		return strconv.ParseInt(input, 10, 64)
	case "boolean":
		return strconv.ParseBool(input)
	default:
		return input, nil
	}
}

// formatVariableValue formats a variable value for display (masking sensitive ones)
func (h *AgentHandler) formatVariableValue(value interface{}, sensitive bool) string {
	if sensitive {
		return "••••••••"
	}
	return fmt.Sprintf("%v", value)
}