package prompt

import (
	"strings"
	"testing"
)

func TestBuilder_Build_Minimal(t *testing.T) {
	builder := NewBuilder().
		WithWorkspace("/workspace", "host")

	result := builder.Build()

	if !strings.Contains(result, "Working directory: /workspace") {
		t.Error("expected working directory in output")
	}

	if !strings.Contains(result, "Workspace mode: host") {
		t.Error("expected workspace mode in output")
	}

	if !strings.Contains(result, "# Tool Guidelines") {
		t.Error("expected tool guidelines section")
	}
}

func TestBuilder_Build_WithAgentPrompt(t *testing.T) {
	builder := NewBuilder().
		WithWorkspace("/workspace", "host").
		WithAgentPrompt("You are a helpful coding assistant.")

	result := builder.Build()

	if !strings.Contains(result, "# Agent Instructions") {
		t.Error("expected agent instructions section")
	}

	if !strings.Contains(result, "You are a helpful coding assistant.") {
		t.Error("expected agent prompt in output")
	}
}

func TestBuilder_Build_WithGit(t *testing.T) {
	builder := NewBuilder().
		WithWorkspace("/workspace", "host").
		WithGit(true, "feature/test")

	result := builder.Build()

	if !strings.Contains(result, "Git enabled: yes") {
		t.Error("expected git enabled in output")
	}

	if !strings.Contains(result, "Git branch: feature/test") {
		t.Error("expected git branch in output")
	}

	if !strings.Contains(result, "## Git Operations") {
		t.Error("expected git operations section")
	}
}

func TestBuilder_Build_WithPreviousContext(t *testing.T) {
	builder := NewBuilder().
		WithWorkspace("/workspace", "host").
		WithPreviousContext(&PreviousStepContext{
			StepName:      "analyze",
			AgentName:     "analyzer",
			Summary:       "Analyzed the codebase",
			FilesModified: []string{"main.go", "util.go"},
			Commits:       []string{"abc123", "def456"},
		})

	result := builder.Build()

	if !strings.Contains(result, "# Previous Step Context") {
		t.Error("expected previous step context section")
	}

	if !strings.Contains(result, "Previous step: analyze (agent: analyzer)") {
		t.Error("expected step and agent info")
	}

	if !strings.Contains(result, "Analyzed the codebase") {
		t.Error("expected summary in output")
	}

	if !strings.Contains(result, "main.go") {
		t.Error("expected files modified in output")
	}

	if !strings.Contains(result, "abc123") {
		t.Error("expected commits in output")
	}
}

func TestBuilder_Build_WithCustomSection(t *testing.T) {
	builder := NewBuilder().
		WithWorkspace("/workspace", "host").
		WithCustomSection("Project Rules", "Always use gofmt before committing.")

	result := builder.Build()

	if !strings.Contains(result, "## Project Rules") {
		t.Error("expected custom section title")
	}

	if !strings.Contains(result, "Always use gofmt before committing.") {
		t.Error("expected custom section content")
	}
}

func TestBuilder_Build_Complete(t *testing.T) {
	builder := NewBuilder().
		WithWorkspace("/workspace", "host").
		WithAgentPrompt("You are an expert coder.").
		WithGit(true, "main").
		WithPreviousContext(&PreviousStepContext{
			StepName:  "setup",
			AgentName: "initializer",
			Summary:   "Set up the project",
		}).
		WithCustomSection("Custom", "Custom content")

	result := builder.Build()

	sections := []string{
		"# Environment",
		"# Agent Instructions",
		"# Tool Guidelines",
		"# Previous Step Context",
		"## Custom",
	}

	for _, section := range sections {
		if !strings.Contains(result, section) {
			t.Errorf("expected section %q in output", section)
		}
	}
}

func TestGetTemplate(t *testing.T) {
	template, ok := GetTemplate("coder")
	if !ok {
		t.Fatal("expected to find coder template")
	}

	if template.Name != "coder" {
		t.Errorf("expected template name 'coder', got %q", template.Name)
	}

	if !strings.Contains(template.Content, "expert software engineer") {
		t.Error("expected coder template to mention software engineer")
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	_, ok := GetTemplate("nonexistent")
	if ok {
		t.Error("expected template not to be found")
	}
}

func TestListTemplates(t *testing.T) {
	names := ListTemplates()

	if len(names) == 0 {
		t.Error("expected at least one template")
	}

	found := false
	for _, name := range names {
		if name == "coder" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find 'coder' in template list")
	}
}

func TestDefaultToolDescriptions(t *testing.T) {
	tools := DefaultToolDescriptions()

	if len(tools) == 0 {
		t.Error("expected at least one tool description")
	}

	foundBash := false
	for _, tool := range tools {
		if tool.Name == "bash" {
			foundBash = true
			if tool.Description == "" {
				t.Error("expected bash tool to have description")
			}
			break
		}
	}

	if !foundBash {
		t.Error("expected to find 'bash' in tool descriptions")
	}
}

func TestBuilder_WithTools(t *testing.T) {
	tools := []ToolDescription{
		{Name: "custom", Description: "Custom tool", Parameters: "arg1, arg2"},
	}

	builder := NewBuilder().
		WithWorkspace("/workspace", "host").
		WithTools(tools)

	if len(builder.toolDescriptions) != 1 {
		t.Errorf("expected 1 tool description, got %d", len(builder.toolDescriptions))
	}

	if builder.toolDescriptions[0].Name != "custom" {
		t.Error("expected custom tool")
	}
}
