package load

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
	"station/internal/logging"
)

// handleEditorMode opens an editor for the user to paste configuration
func (h *LoadHandler) handleEditorMode(endpoint, environment, configName string) error {
	styles := getCLIStyles(h.themeManager)

	fmt.Println(styles.Info.Render("📝 Opening editor for template configuration..."))

	// Import editor service
	editorService := &EditorService{}

	// Open editor with template
	content, err := editorService.OpenEditorWithTemplate()
	if err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		fmt.Println(styles.Info.Render("⚠️  No content provided. Operation cancelled."))
		return nil
	}

	// Clean up the content (remove instruction comments)
	content = h.cleanEditorContent(content)

	// Validate JSON
	if err := editorService.ValidateJSON(content); err != nil {
		fmt.Printf("%s Invalid JSON format. Please check your configuration.\n", styles.Error.Render("❌"))
		return err
	}

	fmt.Println(styles.Success.Render("✅ Configuration received successfully!"))

	// Parse the configuration
	var config LoadMCPConfig
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Always use AI detection in editor mode
	h.initializeAI()

	// Process with AI detection
	hasTemplates, missingValues := h.detectTemplates(&config)
	if hasTemplates {
		fmt.Println(styles.Info.Render("🔍 AI detected placeholders, generating intelligent form..."))
		processedConfig, err := h.processTemplateConfig(&config, missingValues)
		if err != nil {
			return fmt.Errorf("failed to process templates: %w", err)
		}
		config = *processedConfig
	}

	// Upload the configuration
	return h.uploadConfiguration(&config, endpoint, environment, configName)
}

// cleanEditorContent removes instruction comments from editor content
func (h *LoadHandler) cleanEditorContent(content string) string {
	lines := strings.Split(content, "\n")
	var cleanLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip instruction comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}

	return strings.Join(cleanLines, "\n")
}

// uploadConfiguration uploads the configuration using the same logic as the main load flow
func (h *LoadHandler) uploadConfiguration(config *LoadMCPConfig, endpoint, environment, configName string) error {
	// Use filename as default config name if not provided
	if configName == "" {
		if config.Name != "" {
			configName = config.Name
		} else {
			configName = "editor-config"
		}
	}

	// Upload the configuration using existing logic
	localMode := viper.GetBool("local_mode")

	if localMode || endpoint == "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 Running in local mode"))
		return h.uploadConfigLocalLoad(*config, configName, environment)
	} else if endpoint != "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 Connecting to: " + endpoint))
		return h.uploadConfigRemoteLoad(*config, configName, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// EditorService is a simple editor service for opening external editors
type EditorService struct{}

// OpenEditorWithTemplate opens editor with a helpful template
func (e *EditorService) OpenEditorWithTemplate() (string, error) {
	template := `{
    "name": "My MCP Configuration",
    "description": "Configuration with placeholders for AI detection",
    "mcpServers": {
        "SQLite Server": {
            "command": "npx",
            "args": [
                "-y",
                "mcp-sqlite",
                "<path-to-your-sqlite-database.db>"
            ]
        },
        "API Server": {
            "command": "node",
            "args": ["/path/to/your/server.js"],
            "env": {
                "API_KEY": "YOUR_API_KEY",
                "DATABASE_URL": "postgresql://user:password@localhost/db",
                "PORT": "3000"
            }
        }
    }
}

# Instructions:
# 1. Replace example servers with your actual MCP configurations
# 2. Use any placeholder format - AI will detect them:
#    • <path-to-file>        → File paths with angle brackets
#    • YOUR_API_KEY          → ALL CAPS environment variables  
#    • [TOKEN]              → Square bracket tokens
#    • your-username        → Hyphenated placeholders
#    • /path/to/your/file   → Path-like placeholders
# 3. Save and close to continue with AI form generation
# 4. Delete these instruction lines before saving`

	return e.openEditor(template, "json")
}

// OpenMCPEditorTemplate opens editor with a clean MCP template
func (e *EditorService) OpenMCPEditorTemplate() (string, error) {
	template := `{
  "name": "My MCP Server",
  "description": "Description of what this MCP server provides",
  "mcpServers": {
    "server-name": {
      "command": "your-command-here",
      "args": ["arg1", "arg2"],
      "env": {
        "API_KEY": "{{ .API_KEY }}",
        "DATABASE_URL": "{{ .DATABASE_URL }}"
      }
    }
  }
}

# Instructions:
# 1. Replace the example above with your actual MCP server configuration
# 2. Common MCP server examples:
#
#    Filesystem MCP:
#    "filesystem": {
#      "command": "npx",
#      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .ALLOWED_PATH }}"]
#    }
#
#    Database MCP:
#    "database": {
#      "command": "npx", 
#      "args": ["-y", "@modelcontextprotocol/server-sqlite@latest", "{{ .DATABASE_PATH }}"]
#    }
#
#    Custom server:
#    "my-server": {
#      "command": "python",
#      "args": ["{{ .SERVER_SCRIPT_PATH }}"],
#      "env": {
#        "CONFIG_PATH": "{{ .CONFIG_PATH }}"
#      }
#    }
#
# 3. Use template variables for values that change per environment:
#    • {{ .VARIABLE_NAME }} - Go template format (recommended)
#    • Variables will be prompted during 'stn sync'
#
# 4. Save and close when done. Quit without saving (:q) to cancel.
# 5. Delete these instruction lines before saving`

	return e.openEditor(template, "json")
}

// ValidateJSON validates JSON content
func (e *EditorService) ValidateJSON(content string) error {
	var js json.RawMessage
	return json.Unmarshal([]byte(content), &js)
}

// openEditor opens the default editor with content
func (e *EditorService) openEditor(initialContent, extension string) (string, error) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("station-template-*.%s", extension))
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write initial content
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		return "", fmt.Errorf("failed to write initial content: %w", err)
	}
	tmpFile.Close()

	// Calculate hash of initial content to detect changes
	initialHash := e.hashContent(initialContent)

	// Get editor
	editor := e.getEditor()

	logging.Info("📝 Opening editor: %s", editor)
	logging.Info("💡 Paste your MCP configuration template and save to continue...")
	logging.Info("💡 To cancel, quit without saving (e.g., :q in vim, Ctrl+X in nano)")

	// Open editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor command failed: %w", err)
	}

	// Read content after editing
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %w", err)
	}

	finalContent := strings.TrimSpace(string(content))
	finalHash := e.hashContent(finalContent)

	// If content hash hasn't changed, user didn't make meaningful changes
	if finalHash == initialHash {
		return "", nil // Return empty string to indicate no changes
	}

	return finalContent, nil
}

// hashContent creates a SHA256 hash of normalized content for comparison
func (e *EditorService) hashContent(content string) string {
	// Normalize content by trimming whitespace and removing empty lines
	lines := strings.Split(content, "\n")
	var normalizedLines []string
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalizedLines = append(normalizedLines, trimmed)
		}
	}
	
	normalizedContent := strings.Join(normalizedLines, "\n")
	hash := sha256.Sum256([]byte(normalizedContent))
	return hex.EncodeToString(hash[:])
}

// getEditor determines which editor to use
func (e *EditorService) getEditor() string {
	// Check environment variables
	if editor := os.Getenv("VISUAL"); editor != "" {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}

	// Try common editors
	editors := []string{"code", "nano", "vim", "vi"}
	for _, editor := range editors {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}

	return "nano" // Fallback
}

// handleInteractiveEditor opens editor when no arguments are provided to load command
func (h *LoadHandler) handleInteractiveEditor(endpoint, environment, configName string, detectMode bool) error {
	styles := getCLIStyles(h.themeManager)

	fmt.Println(styles.Info.Render("📝 Opening interactive editor for MCP template configuration..."))
	fmt.Println(styles.Info.Render("💡 Paste your MCP configuration template and save to continue"))

	// Initialize AI if detect mode is enabled
	if detectMode {
		h.initializeAI()
	}

	// Import editor service
	editorService := &EditorService{}

	// Open editor with template
	content, err := editorService.OpenEditorWithTemplate()
	if err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		fmt.Println(styles.Info.Render("⚠️  No content provided. Operation cancelled."))
		return nil
	}

	// Clean up the content (remove instruction comments)
	content = h.cleanEditorContent(content)

	// Validate JSON
	if err := editorService.ValidateJSON(content); err != nil {
		fmt.Printf("%s Invalid JSON format. Please check your configuration.\n", styles.Error.Render("❌"))
		return err
	}

	fmt.Println(styles.Success.Render("✅ Configuration received successfully!"))

	// Parse the configuration
	var config LoadMCPConfig
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Always use AI detection in interactive editor mode
	if !detectMode {
		h.initializeAI()
	}

	// Detect and process templates
	hasTemplates, missingValues := h.detectTemplates(&config)
	if hasTemplates {
		fmt.Println(styles.Info.Render("🔍 Template variables detected, generating form for values..."))
		
		// Show detected variables
		if len(missingValues) > 0 {
			fmt.Printf("📋 Found %d template variable(s) that need values:\n", len(missingValues))
			for i, variable := range missingValues {
				fmt.Printf("  %d. %s\n", i+1, variable)
			}
			fmt.Println()
		}

		processedConfig, err := h.processTemplateConfig(&config, missingValues)
		if err != nil {
			return fmt.Errorf("failed to process templates: %w", err)
		}
		
		if processedConfig == nil {
			fmt.Println(styles.Info.Render("Template configuration cancelled"))
			return nil
		}
		
		config = *processedConfig
	} else {
		fmt.Println(styles.Info.Render("✅ No template variables detected - configuration ready to load"))
	}

	// Generate config name if not provided
	if configName == "" {
		if config.Name != "" {
			configName = config.Name
		} else {
			configName = "interactive-config"
		}
	}

	// Add unique ID suffix to prevent duplicates
	configName = h.generateUniqueConfigName(configName)

	fmt.Printf("📝 Config name: %s\n", configName)
	fmt.Printf("🌍 Environment: %s\n", environment)

	// Upload the configuration using the file-based system
	return h.uploadConfiguration(&config, endpoint, environment, configName)
}

// HandleMCPEditor opens an editor for creating new MCP configurations
func (h *LoadHandler) HandleMCPEditor(endpoint, environment, configName string) error {
	styles := getCLIStyles(h.themeManager)

	fmt.Println(styles.Info.Render("📝 Opening editor for new MCP configuration..."))
	logging.Info("💾 Config will be saved as: %s.json", configName)
	logging.Info("🌍 Environment: %s", environment)

	// Import editor service
	editorService := &EditorService{}

	// Open editor with MCP-specific template
	content, err := editorService.OpenMCPEditorTemplate()
	if err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		fmt.Println(styles.Info.Render("⚠️  Editor closed without saving. Operation cancelled."))
		return nil
	}

	// Clean up the content (remove instruction comments)
	content = h.cleanEditorContent(content)

	// Validate JSON
	if err := editorService.ValidateJSON(content); err != nil {
		fmt.Printf("%s Invalid JSON format. Please check your configuration.\n", styles.Error.Render("❌"))
		return err
	}

	fmt.Println(styles.Success.Render("✅ Configuration received successfully!"))

	// Parse the configuration
	var config LoadMCPConfig
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Set the config name from the provided argument
	if config.Name == "" {
		config.Name = configName
	}

	// Save the configuration directly without any AI processing
	// Template variables will be handled during 'stn sync'
	return h.uploadConfiguration(&config, endpoint, environment, configName)
}
