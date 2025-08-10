package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// validEditors contains allowlisted editors to prevent command injection
var validEditors = map[string]bool{
	"vi":      true,
	"vim":     true,
	"nano":    true,
	"emacs":   true,
	"code":    true,
	"gedit":   true,
	"subl":    true,
	"atom":    true,
	"notepad": true,
}

// validExtensions contains allowlisted file extensions to prevent path traversal
var validExtensions = map[string]bool{
	"txt":  true,
	"json": true,
	"yaml": true,
	"yml":  true,
	"toml": true,
	"md":   true,
}

// EditorService handles opening external editors for content editing
type EditorService struct{}

// NewEditorService creates a new editor service
func NewEditorService() *EditorService {
	return &EditorService{}
}

// OpenEditor opens the default editor with optional initial content and returns the edited content
func (e *EditorService) OpenEditor(initialContent string, extension string) (string, error) {
	// Validate extension to prevent path traversal
	if !validExtensions[strings.ToLower(extension)] {
		return "", fmt.Errorf("invalid file extension: %s (allowed: txt, json, yaml, yml, toml, md)", extension)
	}

	// Create temporary file with validated extension
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, fmt.Sprintf("station-template-*.%s", extension))
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

	// Determine and validate editor to use
	editor := e.getEditor()
	
	// Validate editor command against allowlist
	baseEditor := filepath.Base(editor)
	if !validEditors[baseEditor] {
		return "", fmt.Errorf("editor not allowed: %s (allowed: vi, vim, nano, emacs, code, gedit, subl, atom, notepad)", baseEditor)
	}
	
	// Open editor with validated command
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
	content, err := os.ReadFile(tmpFile.Name())
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
	tmpFile, err := os.CreateTemp("", "validate-*.json")
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