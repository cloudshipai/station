package memory

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// MemoryMiddleware loads and injects AGENTS.md content into the system prompt
type MemoryMiddleware struct {
	backend Backend
	sources []string
	logger  *slog.Logger
}

// NewMemoryMiddleware creates a new memory middleware with the given backend and sources
func NewMemoryMiddleware(backend Backend, sources []string) *MemoryMiddleware {
	return &MemoryMiddleware{
		backend: backend,
		sources: sources,
		logger:  slog.Default().With("component", "memory_middleware"),
	}
}

// LoadMemory reads all AGENTS.md files from configured sources
// Returns a map of source path to content
func (m *MemoryMiddleware) LoadMemory() (map[string]string, error) {
	contents := make(map[string]string)

	for _, source := range m.sources {
		expandedPath := expandPath(source)
		content, err := m.backend.ReadFile(expandedPath)
		if err != nil {
			// Memory files are optional - just log and continue
			m.logger.Debug("memory source not found",
				"source", expandedPath,
				"error", err)
			continue
		}

		contents[source] = string(content)
		m.logger.Debug("loaded memory",
			"source", expandedPath,
			"bytes", len(content))
	}

	return contents, nil
}

// FormatSystemPromptSection formats memory content for injection into system prompt
// Memory is always loaded (unlike skills which use progressive disclosure)
func (m *MemoryMiddleware) FormatSystemPromptSection() (string, error) {
	contents, err := m.LoadMemory()
	if err != nil {
		return "", err
	}

	if len(contents) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("\n\n<agent_memory>\n")

	for source, content := range contents {
		sb.WriteString(fmt.Sprintf("\n<!-- Source: %s -->\n", source))
		sb.WriteString(content)
		sb.WriteString("\n")
	}

	sb.WriteString("</agent_memory>\n")
	sb.WriteString(`
<memory_guidelines>
The above context was loaded from AGENTS.md files in your filesystem. As you learn from interactions:

**When to update memory (use edit_file on AGENTS.md):**
- User explicitly asks you to remember something
- User describes role or behavior preferences
- User provides feedback on your work that should persist
- You discover new patterns or preferences

**When NOT to update memory:**
- Temporary or transient information
- One-time task requests
- Simple questions that don't reveal lasting preferences

**Security:**
- NEVER store API keys, passwords, or credentials in memory files
</memory_guidelines>
`)

	return sb.String(), nil
}

// expandPath expands ~ to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// DefaultMemorySources returns the default memory source paths
func DefaultMemorySources(envPath string) []string {
	return []string{
		"~/.config/station/AGENTS.md",
		filepath.Join(envPath, "memory", "AGENTS.md"),
		".station/AGENTS.md",
	}
}
