package load

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
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

	// Get editor
	editor := e.getEditor()

	fmt.Printf("📝 Opening editor: %s\n", editor)
	fmt.Printf("💡 Paste your MCP configuration template and save to continue...\n")

	// Open editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor command failed: %w", err)
	}

	// Read content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %w", err)
	}

	return strings.TrimSpace(string(content)), nil
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
