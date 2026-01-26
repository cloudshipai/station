package skills

import (
	"strings"
	"testing"
)

// MockBackend implements Backend for testing
type MockBackend struct {
	Files map[string][]byte
	Dirs  map[string][]DirEntry
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		Files: make(map[string][]byte),
		Dirs:  make(map[string][]DirEntry),
	}
}

func (m *MockBackend) ListDir(path string) ([]DirEntry, error) {
	if entries, ok := m.Dirs[path]; ok {
		return entries, nil
	}
	return nil, &mockError{msg: "directory not found: " + path}
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

func TestParseSkillMetadata(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		dirName  string
		wantName string
		wantDesc string
		wantErr  bool
	}{
		{
			name: "valid skill with all fields",
			content: `---
name: web-research
description: Structured approach to web research
license: MIT
allowed-tools:
  - web_search
  - web_fetch
triggers:
  - research
  - search
---

# Web Research Skill

Instructions here...`,
			dirName:  "web-research",
			wantName: "web-research",
			wantDesc: "Structured approach to web research",
			wantErr:  false,
		},
		{
			name: "skill without name uses directory",
			content: `---
description: Test skill
---

# Test Skill`,
			dirName:  "test-skill",
			wantName: "test-skill",
			wantDesc: "Test skill",
			wantErr:  false,
		},
		{
			name: "name mismatch error",
			content: `---
name: wrong-name
description: Test
---

Content`,
			dirName: "actual-name",
			wantErr: true,
		},
		{
			name:    "no frontmatter",
			content: "# Just markdown content",
			dirName: "test",
			wantErr: true,
		},
		{
			name:    "unclosed frontmatter",
			content: "---\nname: test",
			dirName: "test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := ParseSkillMetadata([]byte(tt.content), "/test/path/SKILL.md", tt.dirName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSkillMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if metadata.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", metadata.Name, tt.wantName)
			}
			if metadata.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", metadata.Description, tt.wantDesc)
			}
		})
	}
}

func TestGetSkillBody(t *testing.T) {
	content := `---
name: test
description: Test skill
---

# Test Skill

This is the body content.`

	body := GetSkillBody([]byte(content))
	if !strings.Contains(body, "# Test Skill") {
		t.Error("body should contain heading")
	}
	if !strings.Contains(body, "This is the body content.") {
		t.Error("body should contain content")
	}
	if strings.Contains(body, "name: test") {
		t.Error("body should not contain frontmatter")
	}
}

func TestSkillsMiddleware_LoadSkills(t *testing.T) {
	backend := NewMockBackend()

	// Set up mock filesystem
	backend.Dirs["/skills/user"] = []DirEntry{
		{Name: "web-research", IsDir: true},
		{Name: "code-review", IsDir: true},
		{Name: "not-a-skill.txt", IsDir: false}, // Should be ignored
	}

	backend.Files["/skills/user/web-research/SKILL.md"] = []byte(`---
name: web-research
description: Web research skill
---
# Web Research`)

	backend.Files["/skills/user/code-review/SKILL.md"] = []byte(`---
name: code-review
description: Code review skill
---
# Code Review`)

	backend.Dirs["/skills/project"] = []DirEntry{
		{Name: "code-review", IsDir: true}, // Override user skill
	}

	backend.Files["/skills/project/code-review/SKILL.md"] = []byte(`---
name: code-review
description: Project-specific code review
---
# Project Code Review`)

	middleware := NewSkillsMiddleware(backend, []string{"/skills/user", "/skills/project"})
	skills, err := middleware.LoadSkills()
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	if len(skills) != 2 {
		t.Errorf("LoadSkills() returned %d skills, want 2", len(skills))
	}

	// Check that code-review was overridden by project
	for _, skill := range skills {
		if skill.Name == "code-review" {
			if skill.Description != "Project-specific code review" {
				t.Errorf("code-review should have project description, got %q", skill.Description)
			}
		}
	}
}

func TestSkillsMiddleware_FormatSystemPromptSection(t *testing.T) {
	skills := []SkillMetadata{
		{
			Name:         "web-research",
			Description:  "Web research skill",
			AllowedTools: []string{"web_search", "web_fetch"},
			Path:         "/skills/user/web-research/SKILL.md",
		},
		{
			Name:        "code-review",
			Description: "Code review skill",
			Path:        "/skills/user/code-review/SKILL.md",
		},
	}

	middleware := NewSkillsMiddleware(nil, nil)
	section := middleware.FormatSystemPromptSection(skills)

	// Check key elements
	if !strings.Contains(section, "## Skills System") {
		t.Error("section should contain header")
	}
	if !strings.Contains(section, "**web-research**") {
		t.Error("section should contain skill name")
	}
	if !strings.Contains(section, "Web research skill") {
		t.Error("section should contain skill description")
	}
	if !strings.Contains(section, "web_search, web_fetch") {
		t.Error("section should contain allowed tools")
	}
	if !strings.Contains(section, "/skills/user/web-research/SKILL.md") {
		t.Error("section should contain skill path")
	}
	if !strings.Contains(section, "Progressive Disclosure") {
		t.Error("section should contain usage instructions")
	}
}

func TestSkillsMiddleware_EmptySkills(t *testing.T) {
	middleware := NewSkillsMiddleware(nil, nil)
	section := middleware.FormatSystemPromptSection(nil)
	if section != "" {
		t.Errorf("empty skills should produce empty section, got %q", section)
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

func TestSkillsMiddleware_MissingSource(t *testing.T) {
	backend := NewMockBackend()
	// No directories set up - all sources should fail gracefully

	middleware := NewSkillsMiddleware(backend, []string{"/nonexistent/path"})
	skills, err := middleware.LoadSkills()
	if err != nil {
		t.Fatalf("LoadSkills() should not error on missing sources, got %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("LoadSkills() should return empty for missing sources, got %d skills", len(skills))
	}
}

func TestSkillsMiddleware_SkillWithoutSkillMd(t *testing.T) {
	backend := NewMockBackend()
	backend.Dirs["/skills"] = []DirEntry{
		{Name: "valid-skill", IsDir: true},
		{Name: "no-skill-md", IsDir: true},
	}

	backend.Files["/skills/valid-skill/SKILL.md"] = []byte(`---
name: valid-skill
description: Valid skill
---
# Valid`)

	// no-skill-md directory has no SKILL.md file

	middleware := NewSkillsMiddleware(backend, []string{"/skills"})
	skills, err := middleware.LoadSkills()
	if err != nil {
		t.Fatalf("LoadSkills() error = %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("LoadSkills() returned %d skills, want 1", len(skills))
	}
	if skills[0].Name != "valid-skill" {
		t.Errorf("skill name = %q, want valid-skill", skills[0].Name)
	}
}
