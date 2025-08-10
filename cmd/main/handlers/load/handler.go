package load

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// discoverMCPConfig looks for MCP configuration files in the current directory
// Returns the first found file or empty string if none found
func (h *LoadHandler) discoverMCPConfig() string {
	// Check for common MCP config file patterns in current directory
	possibleFiles := []string{
		"mcp.json",
		".mcp.json", 
		"mcp-config.json",
		".mcp-config.json",
	}
	
	for _, filename := range possibleFiles {
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
	}
	
	return ""
}

func (h *LoadHandler) RunLoad(cmd *cobra.Command, args []string) error {
	banner := getCLIStyles(h.themeManager).Banner.Render("📂 Loading MCP Configuration")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	envFlag, _ := cmd.Flags().GetString("env")
	configName, _ := cmd.Flags().GetString("config-name")
	detectMode, _ := cmd.Flags().GetBool("detect")
	editorMode, _ := cmd.Flags().GetBool("editor")

	// --env flag takes priority over --environment
	if envFlag != "" {
		environment = envFlag
	}

	// Initialize AI if detect mode is enabled
	if detectMode {
		h.initializeAI()
	}

	// Handle editor mode (-e flag)
	if editorMode {
		return h.handleEditorMode(endpoint, environment, configName)
	}

	var configFile string

	// All HTTPS URLs now use the enhanced TurboTax flow with web content parsing

	// Check if we have a GitHub URL as argument (legacy flow)
	if len(args) > 0 && isGitHubURL(args[0]) {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 GitHub URL detected, starting discovery flow..."))
		return h.runGitHubDiscoveryFlow(args[0], environment, endpoint)
	}

	// Check if we have any HTTPS URL as argument (enhanced flow with web content parsing)
	if len(args) > 0 && isHTTPSURL(args[0]) {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 HTTPS URL detected, starting enhanced TurboTax wizard..."))
		return h.runEnhancedTurboTaxFlow(args[0], environment, endpoint)
	}

	// Check if we have a direct file argument
	if len(args) > 0 {
		if _, err := os.Stat(args[0]); err == nil {
			configFile = args[0]

			// Initialize AI if detect mode is enabled for file input
			if detectMode {
				h.initializeAI()
			}
		} else {
			return fmt.Errorf("file not found: %s", args[0])
		}
	} else {
		// No arguments provided - try to discover MCP config files in current directory
		discoveredFile := h.discoverMCPConfig()
		if discoveredFile != "" {
			configFile = discoveredFile
			fmt.Printf("🔍 Discovered config file: %s\n", configFile)
			
			// Initialize AI if detect mode is enabled for discovered file
			if detectMode {
				h.initializeAI()
			}
		} else {
			// No config files found - open interactive editor
			fmt.Println(getCLIStyles(h.themeManager).Info.Render("📝 No MCP configuration files found in current directory, opening interactive editor..."))
			return h.handleInteractiveEditor(endpoint, environment, configName, detectMode)
		}
	}

	fmt.Printf("📄 Found config file: %s\n", configFile)

	// Read and parse MCP config
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var mcpConfig LoadMCPConfig
	if err := json.Unmarshal(data, &mcpConfig); err != nil {
		return fmt.Errorf("failed to parse MCP config: %w", err)
	}

	if len(mcpConfig.MCPServers) == 0 {
		return fmt.Errorf("no MCP servers found in configuration")
	}

	fmt.Printf("🔧 Found %d MCP server(s)\n", len(mcpConfig.MCPServers))

	// Save the truly original config before any processing (deep copy)
	originalConfigJSON, _ := json.Marshal(mcpConfig)
	var originalConfig LoadMCPConfig
	json.Unmarshal(originalConfigJSON, &originalConfig)
	
	// Check if this is a template configuration and handle it
	var resolvedVariables map[string]string
	if hasTemplates, missingValues := h.detectTemplates(&mcpConfig); hasTemplates {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🧩 Template configuration detected"))

		// Show credential form for missing values - this is just for validation/UX
		processedConfig, vars, err := h.processTemplateConfigWithVariables(&mcpConfig, missingValues)
		if err != nil {
			return fmt.Errorf("failed to process template: %w", err)
		}

		if processedConfig == nil {
			fmt.Println(getCLIStyles(h.themeManager).Info.Render("Template configuration cancelled"))
			return nil
		}

		resolvedVariables = vars
		// Use the processed config for tool discovery, but save original as template
		mcpConfig = *processedConfig
	} else {
		// No templates, so no variables to resolve
		resolvedVariables = make(map[string]string)
	}

	// Use filename as default config name if not provided
	if configName == "" {
		if mcpConfig.Name != "" {
			configName = mcpConfig.Name
		} else {
			configName = filepath.Base(configFile)
			if ext := filepath.Ext(configName); ext != "" {
				configName = configName[:len(configName)-len(ext)]
			}
		}
	}

	// Add unique ID suffix to prevent duplicates
	configName = h.generateUniqueConfigName(configName)

	fmt.Printf("📝 Config name: %s\n", configName)
	fmt.Printf("🌍 Environment: %s\n", environment)

	// Determine if we're in local mode - check config first, then endpoint flag
	isLocal := endpoint == "" && viper.GetBool("local_mode")

	if isLocal {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 Running in local mode"))
		return h.uploadConfigLocalLoadTemplateWithVariables(originalConfig, mcpConfig, resolvedVariables, configName, environment)
	} else if endpoint != "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 Connecting to: " + endpoint))
		return h.uploadConfigRemoteLoad(mcpConfig, configName, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}
