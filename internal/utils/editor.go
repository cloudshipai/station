package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EditorService handles opening external editors for content editing
type EditorService struct{}

// NewEditorService creates a new editor service
func NewEditorService() *EditorService {
	return &EditorService{}
}

// OpenEditor opens the default editor with optional initial content and returns the edited content
func (e *EditorService) OpenEditor(initialContent string, extension string) (string, error) {
	// Create temporary file
	tmpDir := os.TempDir()
	tmpFile, err := ioutil.TempFile(tmpDir, fmt.Sprintf("station-template-*.%s", extension))
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write initial content if provided
	if initialContent != "" {
		if _, err := tmpFile.WriteString(initialContent); err != nil {
			return "", fmt.Errorf("failed to write initial content: %w", err)
		}
	}
	tmpFile.Close()

	// Determine editor to use
	editor := e.getEditor()
	
	// Open editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("📝 Opening editor: %s\n", editor)
	fmt.Printf("💡 Paste your MCP configuration template and save to continue...\n")
	
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor command failed: %w", err)
	}

	// Read the edited content
	content, err := ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %w", err)
	}

	return strings.TrimSpace(string(content)), nil
}

// OpenEditorWithTemplate opens editor with a helpful template
func (e *EditorService) OpenEditorWithTemplate() (string, error) {
	template := `{
    "name": "My MCP Configuration",
    "description": "Configuration with placeholders for AI detection",
    "mcpServers": {
        "Example Server": {
            "command": "npx",
            "args": [
                "-y",
                "mcp-sqlite",
                "<path-to-your-sqlite-database.db>"
            ]
        },
        "Another Example": {
            "command": "node",
            "args": ["/path/to/your/server.js"],
            "env": {
                "API_KEY": "YOUR_API_KEY",
                "DATABASE_URL": "postgresql://user:password@localhost/db"
            }
        }
    }
}

# Instructions:
# 1. Replace the example servers with your actual MCP server configurations
# 2. Use any placeholder format - AI will detect them:
#    • <path-to-file>
#    • YOUR_API_KEY
#    • [TOKEN] 
#    • your-username
#    • /path/to/your/file
# 3. Save and close to continue with AI-powered form generation
# 4. Delete these instruction lines before saving`

	return e.OpenEditor(template, "json")
}

// getEditor determines which editor to use
func (e *EditorService) getEditor() string {
	// Check environment variables in order of preference
	editors := []string{
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
	}
	
	// Add common editors to try
	commonEditors := []string{
		"code",    // VS Code
		"subl",    // Sublime Text
		"atom",    // Atom
		"nano",    // Nano
		"vim",     // Vim
		"vi",      // Vi
		"notepad", // Windows Notepad
	}
	
	editors = append(editors, commonEditors...)
	
	// Find the first available editor
	for _, editor := range editors {
		if editor == "" {
			continue
		}
		
		// Check if editor is available
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}
	
	// Fallback to nano
	return "nano"
}

// ValidateJSON checks if the content is valid JSON
func (e *EditorService) ValidateJSON(content string) error {
	// Basic validation - try to parse as JSON
	tmpFile, err := ioutil.TempFile("", "validate-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	
	if _, err := tmpFile.WriteString(content); err != nil {
		return err
	}
	tmpFile.Close()
	
	// Use jq to validate if available, otherwise basic checks
	if _, err := exec.LookPath("jq"); err == nil {
		cmd := exec.Command("jq", ".", tmpFile.Name())
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("invalid JSON format")
		}
	}
	
	return nil
}