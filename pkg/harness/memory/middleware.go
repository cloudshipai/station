package memory

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryMiddleware loads and injects AGENTS.md content into the system prompt
type MemoryMiddleware struct {
	backend       Backend
	sources       []string
	workspacePath string
	logger        *slog.Logger
}

// NewMemoryMiddleware creates a new memory middleware with the given backend and sources
func NewMemoryMiddleware(backend Backend, sources []string) *MemoryMiddleware {
	return &MemoryMiddleware{
		backend: backend,
		sources: sources,
		logger:  slog.Default().With("component", "memory_middleware"),
	}
}

// NewMemoryMiddlewareWithWorkspace creates a memory middleware with workspace support
func NewMemoryMiddlewareWithWorkspace(backend Backend, sources []string, workspacePath string) *MemoryMiddleware {
	return &MemoryMiddleware{
		backend:       backend,
		sources:       sources,
		workspacePath: workspacePath,
		logger:        slog.Default().With("component", "memory_middleware"),
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

// DefaultMemorySourcesWithWorkspace returns memory sources including workspace paths
func DefaultMemorySourcesWithWorkspace(envPath, workspacePath string) []string {
	sources := DefaultMemorySources(envPath)

	if workspacePath != "" {
		// Add workspace-based memory files
		today := time.Now().Format("2006-01-02")
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

		sources = append(sources,
			filepath.Join(workspacePath, "MEMORY.md"),
			filepath.Join(workspacePath, "memory", today+".md"),
			filepath.Join(workspacePath, "memory", yesterday+".md"),
		)
	}

	return sources
}

// WriteMemory writes content to a memory file in the workspace
func (m *MemoryMiddleware) WriteMemory(relativePath, content string) error {
	if m.workspacePath == "" {
		return fmt.Errorf("workspace path not configured")
	}

	wb, ok := m.backend.(WritableBackend)
	if !ok {
		// Fall back to direct file system operations
		return m.writeMemoryDirect(relativePath, content)
	}

	fullPath := filepath.Join(m.workspacePath, relativePath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := wb.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Append timestamp and content
	entry := fmt.Sprintf("\n## %s\n%s\n", time.Now().Format("2006-01-02 15:04:05"), content)

	// Read existing content
	existing, _ := wb.ReadFile(fullPath)

	// Write combined content
	newContent := string(existing) + entry
	if err := wb.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	m.logger.Debug("wrote to memory", "path", fullPath, "bytes", len(entry))
	return nil
}

// writeMemoryDirect writes directly to the filesystem (fallback)
func (m *MemoryMiddleware) writeMemoryDirect(relativePath, content string) error {
	fullPath := filepath.Join(m.workspacePath, relativePath)
	fullPath = expandPath(fullPath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Open file for appending
	f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open memory file: %w", err)
	}
	defer f.Close()

	// Write timestamped entry
	entry := fmt.Sprintf("\n## %s\n%s\n", time.Now().Format("2006-01-02 15:04:05"), content)
	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write to memory file: %w", err)
	}

	m.logger.Debug("wrote to memory (direct)", "path", fullPath, "bytes", len(entry))
	return nil
}

// DailyLogPath returns the path for today's daily memory log
func (m *MemoryMiddleware) DailyLogPath() string {
	return filepath.Join("memory", time.Now().Format("2006-01-02")+".md")
}

// InitializeWorkspaceMemory creates the workspace memory directory structure
func (m *MemoryMiddleware) InitializeWorkspaceMemory() error {
	if m.workspacePath == "" {
		return nil // No workspace configured
	}

	memoryDir := filepath.Join(expandPath(m.workspacePath), "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Create MEMORY.md if it doesn't exist
	memoryFile := filepath.Join(expandPath(m.workspacePath), "MEMORY.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		header := "# Agent Memory\n\nLong-term facts, preferences, and decisions.\n"
		if err := os.WriteFile(memoryFile, []byte(header), 0644); err != nil {
			m.logger.Warn("failed to create MEMORY.md", "error", err)
		}
	}

	// Create HEARTBEAT.md if it doesn't exist
	heartbeatFile := filepath.Join(expandPath(m.workspacePath), "HEARTBEAT.md")
	if _, err := os.Stat(heartbeatFile); os.IsNotExist(err) {
		template := `# Heartbeat Checklist

## Quick Checks
- Review any pending tasks or notifications
- Check for alerts or issues that need attention

## Rules
- If nothing needs attention, reply with HEARTBEAT_OK
- Keep notifications concise and actionable
`
		if err := os.WriteFile(heartbeatFile, []byte(template), 0644); err != nil {
			m.logger.Warn("failed to create HEARTBEAT.md", "error", err)
		}
	}

	m.logger.Debug("initialized workspace memory", "path", m.workspacePath)
	return nil
}

// FlushSessionMemory extracts and persists important information from a session.
// This should be called before session compaction to preserve key insights.
// Returns the summary written to memory, or empty string if nothing to flush.
func (m *MemoryMiddleware) FlushSessionMemory(summary string) error {
	if m.workspacePath == "" {
		return nil // No workspace, nothing to flush
	}

	if summary == "" {
		return nil // Nothing to flush
	}

	// Write to today's daily log
	dailyPath := m.DailyLogPath()
	return m.WriteMemory(dailyPath, summary)
}

// WorkspacePath returns the configured workspace path
func (m *MemoryMiddleware) WorkspacePath() string {
	return m.workspacePath
}

// SetWorkspacePath updates the workspace path
func (m *MemoryMiddleware) SetWorkspacePath(path string) {
	m.workspacePath = path
}
