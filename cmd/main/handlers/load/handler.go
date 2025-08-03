package load

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func (h *LoadHandler) RunLoad(cmd *cobra.Command, args []string) error {
	banner := getCLIStyles(h.themeManager).Banner.Render("📂 Loading MCP Configuration")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	configName, _ := cmd.Flags().GetString("config-name")
	detectMode, _ := cmd.Flags().GetBool("detect")
	editorMode, _ := cmd.Flags().GetBool("editor")

	// Initialize AI if detect mode is enabled
	if detectMode {
		h.initializeAI()
	}

	// Handle editor mode (-e flag)
	if editorMode {
		return h.handleEditorMode(endpoint, environment, configName)
	}

	var configFile string
	var found bool

	// Check if we have a direct README URL as argument
	if len(args) > 0 && isDirectReadmeURL(args[0]) {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("📄 README URL detected, starting TurboTax-style flow..."))
		return h.runTurboTaxMCPFlow(args[0], environment, endpoint)
	}

	// Check if we have a GitHub URL as argument (legacy flow)
	if len(args) > 0 && isGitHubURL(args[0]) {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 GitHub URL detected, starting discovery flow..."))
		return h.runGitHubDiscoveryFlow(args[0], environment, endpoint)
	}

	// Check if we have a direct file argument
	if len(args) > 0 {
		if _, err := os.Stat(args[0]); err == nil {
			configFile = args[0]
			found = true

			// Initialize AI if detect mode is enabled for file input
			if detectMode {
				h.initializeAI()
			}
		} else {
			return fmt.Errorf("file not found: %s", args[0])
		}
	} else {
		// Look for MCP configuration file in current directory
		configFiles := []string{"mcp.json", ".mcp.json"}

		for _, file := range configFiles {
			if _, err := os.Stat(file); err == nil {
				configFile = file
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("no MCP configuration file found. Looking for: %s", configFiles)
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

	// Check if this is a template configuration and handle it
	if hasTemplates, missingValues := h.detectTemplates(&mcpConfig); hasTemplates {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🧩 Template configuration detected"))

		// Show credential form for missing values
		processedConfig, err := h.processTemplateConfig(&mcpConfig, missingValues)
		if err != nil {
			return fmt.Errorf("failed to process template: %w", err)
		}

		if processedConfig == nil {
			fmt.Println(getCLIStyles(h.themeManager).Info.Render("Template configuration cancelled"))
			return nil
		}

		// Use the processed config
		mcpConfig = *processedConfig
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
		return h.uploadConfigLocalLoad(mcpConfig, configName, environment)
	} else if endpoint != "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 Connecting to: " + endpoint))
		return h.uploadConfigRemoteLoad(mcpConfig, configName, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}
