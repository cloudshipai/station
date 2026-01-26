package memory

import (
	"strings"
	"testing"
)

// MockBackend implements Backend for testing
type MockBackend struct {
	Files map[string][]byte
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		Files: make(map[string][]byte),
	}
}

func (m *MockBackend) ReadFile(path string) ([]byte, error) {
	if content, ok := m.Files[path]; ok {
		return content, nil
	}
	return nil, &mockError{msg: "file not found: " + path}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestMemoryMiddleware_LoadMemory(t *testing.T) {
	backend := NewMockBackend()

	backend.Files["/global/AGENTS.md"] = []byte(`# Global Memory
This is global context.`)

	backend.Files["/project/AGENTS.md"] = []byte(`# Project Memory
This is project-specific context.`)

	middleware := NewMemoryMiddleware(backend, []string{
		"/global/AGENTS.md",
		"/project/AGENTS.md",
	})

	contents, err := middleware.LoadMemory()
	if err != nil {
		t.Fatalf("LoadMemory() error = %v", err)
	}

	if len(contents) != 2 {
		t.Errorf("LoadMemory() returned %d contents, want 2", len(contents))
	}

	if !strings.Contains(contents["/global/AGENTS.md"], "Global Memory") {
		t.Error("missing global memory content")
	}

	if !strings.Contains(contents["/project/AGENTS.md"], "Project Memory") {
		t.Error("missing project memory content")
	}
}

func TestMemoryMiddleware_FormatSystemPromptSection(t *testing.T) {
	backend := NewMockBackend()

	backend.Files["/memory/AGENTS.md"] = []byte(`# Build Commands
- make build
- make test

# Code Style
Use slog for logging`)

	middleware := NewMemoryMiddleware(backend, []string{"/memory/AGENTS.md"})

	section, err := middleware.FormatSystemPromptSection()
	if err != nil {
		t.Fatalf("FormatSystemPromptSection() error = %v", err)
	}

	// Check key elements
	if !strings.Contains(section, "<agent_memory>") {
		t.Error("section should contain agent_memory tag")
	}
	if !strings.Contains(section, "</agent_memory>") {
		t.Error("section should contain closing agent_memory tag")
	}
	if !strings.Contains(section, "Build Commands") {
		t.Error("section should contain memory content")
	}
	if !strings.Contains(section, "Source: /memory/AGENTS.md") {
		t.Error("section should contain source path comment")
	}
	if !strings.Contains(section, "memory_guidelines") {
		t.Error("section should contain guidelines")
	}
	if !strings.Contains(section, "NEVER store API keys") {
		t.Error("section should contain security warning")
	}
}

func TestMemoryMiddleware_EmptyMemory(t *testing.T) {
	backend := NewMockBackend()
	// No files set up

	middleware := NewMemoryMiddleware(backend, []string{"/nonexistent/AGENTS.md"})

	section, err := middleware.FormatSystemPromptSection()
	if err != nil {
		t.Fatalf("FormatSystemPromptSection() should not error, got %v", err)
	}

	if section != "" {
		t.Errorf("empty memory should produce empty section, got %q", section)
	}
}

func TestMemoryMiddleware_MissingSource(t *testing.T) {
	backend := NewMockBackend()
	// Only one file exists
	backend.Files["/exists/AGENTS.md"] = []byte("# Exists")

	middleware := NewMemoryMiddleware(backend, []string{
		"/nonexistent/AGENTS.md",
		"/exists/AGENTS.md",
	})

	contents, err := middleware.LoadMemory()
	if err != nil {
		t.Fatalf("LoadMemory() should not error, got %v", err)
	}

	if len(contents) != 1 {
		t.Errorf("LoadMemory() returned %d contents, want 1", len(contents))
	}

	if _, ok := contents["/exists/AGENTS.md"]; !ok {
		t.Error("should have loaded existing file")
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		// Note: can't easily test ~ expansion without mocking os.UserHomeDir
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandPath(tt.input)
			if !strings.HasPrefix(tt.input, "~/") && got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultMemorySources(t *testing.T) {
	sources := DefaultMemorySources("/env/default")

	if len(sources) < 2 {
		t.Error("should have at least 2 default sources")
	}

	hasGlobal := false
	hasProject := false
	for _, s := range sources {
		if strings.Contains(s, ".config/station") {
			hasGlobal = true
		}
		if strings.Contains(s, ".station") {
			hasProject = true
		}
	}

	if !hasGlobal {
		t.Error("should have global config source")
	}
	if !hasProject {
		t.Error("should have project source")
	}
}
