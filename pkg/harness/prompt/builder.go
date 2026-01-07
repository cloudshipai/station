package prompt

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Builder struct {
	agentPrompt      string
	workspacePath    string
	workspaceMode    string
	gitEnabled       bool
	gitBranch        string
	previousContext  *PreviousStepContext
	toolDescriptions []ToolDescription
	customSections   []CustomSection
}

type ToolDescription struct {
	Name        string
	Description string
	Parameters  string
}

type PreviousStepContext struct {
	StepName      string
	AgentName     string
	Summary       string
	FilesModified []string
	Commits       []string
}

type CustomSection struct {
	Title   string
	Content string
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithAgentPrompt(prompt string) *Builder {
	b.agentPrompt = prompt
	return b
}

func (b *Builder) WithWorkspace(path, mode string) *Builder {
	b.workspacePath = path
	b.workspaceMode = mode
	return b
}

func (b *Builder) WithGit(enabled bool, branch string) *Builder {
	b.gitEnabled = enabled
	b.gitBranch = branch
	return b
}

func (b *Builder) WithPreviousContext(ctx *PreviousStepContext) *Builder {
	b.previousContext = ctx
	return b
}

func (b *Builder) WithTools(tools []ToolDescription) *Builder {
	b.toolDescriptions = tools
	return b
}

func (b *Builder) WithCustomSection(title, content string) *Builder {
	b.customSections = append(b.customSections, CustomSection{
		Title:   title,
		Content: content,
	})
	return b
}

func (b *Builder) Build() string {
	var sections []string

	sections = append(sections, b.buildEnvironmentSection())

	if b.agentPrompt != "" {
		sections = append(sections, b.buildAgentSection())
	}

	sections = append(sections, b.buildToolGuidelines())

	if b.previousContext != nil {
		sections = append(sections, b.buildWorkflowContext())
	}

	for _, custom := range b.customSections {
		sections = append(sections, fmt.Sprintf("## %s\n\n%s", custom.Title, custom.Content))
	}

	return strings.Join(sections, "\n\n")
}

func (b *Builder) buildEnvironmentSection() string {
	var lines []string
	lines = append(lines, "# Environment")
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Working directory: %s", b.workspacePath))
	lines = append(lines, fmt.Sprintf("Workspace mode: %s", b.workspaceMode))
	lines = append(lines, fmt.Sprintf("Platform: %s", getPlatform()))
	lines = append(lines, fmt.Sprintf("Date: %s", time.Now().Format("Mon Jan 02 2006")))

	if b.gitEnabled {
		lines = append(lines, fmt.Sprintf("Git enabled: yes"))
		if b.gitBranch != "" {
			lines = append(lines, fmt.Sprintf("Git branch: %s", b.gitBranch))
		}
		if isGitRepo(b.workspacePath) {
			lines = append(lines, "Is git repo: yes")
		}
	}

	return strings.Join(lines, "\n")
}

func (b *Builder) buildAgentSection() string {
	return fmt.Sprintf("# Agent Instructions\n\n%s", b.agentPrompt)
}

func (b *Builder) buildToolGuidelines() string {
	var lines []string
	lines = append(lines, "# Tool Guidelines")
	lines = append(lines, "")

	lines = append(lines, "## File Operations")
	lines = append(lines, "- Use `read` to view file contents with line numbers")
	lines = append(lines, "- Use `write` to create new files (auto-creates directories)")
	lines = append(lines, "- Use `edit` for precise string replacements in existing files")
	lines = append(lines, "- Use `glob` to find files by pattern (e.g., `**/*.go`)")
	lines = append(lines, "- Use `grep` to search file contents with regex")
	lines = append(lines, "")

	lines = append(lines, "## Bash Commands")
	lines = append(lines, "- Always provide clear descriptions for commands")
	lines = append(lines, "- Commands are executed in the workspace directory")
	lines = append(lines, "- Avoid destructive commands (rm -rf, etc.)")
	lines = append(lines, "- Use && to chain related commands")
	lines = append(lines, "- Maximum timeout: 2 minutes")
	lines = append(lines, "")

	if b.gitEnabled {
		lines = append(lines, "## Git Operations")
		lines = append(lines, "- Check git status before and after changes")
		lines = append(lines, "- NEVER force push or hard reset")
		lines = append(lines, "- Commit messages should be descriptive")
		lines = append(lines, "- Push operations may require approval")
		lines = append(lines, "")
	}

	lines = append(lines, "## Best Practices")
	lines = append(lines, "- Read files before editing to understand context")
	lines = append(lines, "- Make small, incremental changes")
	lines = append(lines, "- Verify changes compile/run before completing")
	lines = append(lines, "- Explain your reasoning when making decisions")

	return strings.Join(lines, "\n")
}

func (b *Builder) buildWorkflowContext() string {
	var lines []string
	lines = append(lines, "# Previous Step Context")
	lines = append(lines, "")

	ctx := b.previousContext
	lines = append(lines, fmt.Sprintf("Previous step: %s (agent: %s)", ctx.StepName, ctx.AgentName))
	lines = append(lines, "")

	if ctx.Summary != "" {
		lines = append(lines, "Summary:")
		lines = append(lines, ctx.Summary)
		lines = append(lines, "")
	}

	if len(ctx.FilesModified) > 0 {
		lines = append(lines, "Files modified:")
		for _, f := range ctx.FilesModified {
			lines = append(lines, fmt.Sprintf("  - %s", f))
		}
		lines = append(lines, "")
	}

	if len(ctx.Commits) > 0 {
		lines = append(lines, "Commits:")
		for _, c := range ctx.Commits {
			lines = append(lines, fmt.Sprintf("  - %s", c))
		}
		lines = append(lines, "")
	}

	lines = append(lines, "Continue from where the previous agent left off. The workspace and git branch are preserved.")

	return strings.Join(lines, "\n")
}

func getPlatform() string {
	return "linux"
}

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	cmd := exec.Command("test", "-d", gitPath)
	return cmd.Run() == nil
}

var defaultToolDescriptions = []ToolDescription{
	{
		Name:        "read",
		Description: "Read file contents with line numbers",
		Parameters:  "path (required), offset (optional), limit (optional)",
	},
	{
		Name:        "write",
		Description: "Create or overwrite a file",
		Parameters:  "path (required), content (required)",
	},
	{
		Name:        "edit",
		Description: "Replace exact string in file",
		Parameters:  "path (required), oldString (required), newString (required), replaceAll (optional)",
	},
	{
		Name:        "bash",
		Description: "Execute shell command",
		Parameters:  "command (required), description (required), workdir (optional), timeout (optional)",
	},
	{
		Name:        "glob",
		Description: "Find files matching pattern",
		Parameters:  "pattern (required), path (optional)",
	},
	{
		Name:        "grep",
		Description: "Search file contents with regex",
		Parameters:  "pattern (required), path (optional), include (optional)",
	},
}

func DefaultToolDescriptions() []ToolDescription {
	return defaultToolDescriptions
}

type PromptTemplate struct {
	Name    string
	Content string
}

var templates = map[string]PromptTemplate{
	"coder": {
		Name: "coder",
		Content: `You are an expert software engineer with deep knowledge of multiple programming languages and best practices.

Your role is to help implement, debug, and improve code. You have access to the workspace filesystem and can read, write, and edit files directly.

Key principles:
1. Write clean, maintainable code following established patterns in the codebase
2. Add appropriate tests for new functionality
3. Handle errors gracefully
4. Document complex logic with clear comments
5. Follow the existing code style and conventions

When making changes:
- First understand the existing code structure
- Make minimal, focused changes
- Verify changes work before completing
- Explain your reasoning for significant decisions`,
	},
	"reviewer": {
		Name: "reviewer",
		Content: `You are an expert code reviewer focused on code quality, security, and best practices.

Your role is to review code changes and provide constructive feedback. You have read access to the codebase to understand context.

Review criteria:
1. Code correctness and logic errors
2. Security vulnerabilities
3. Performance issues
4. Code style and consistency
5. Test coverage
6. Documentation quality

When reviewing:
- Be specific about issues and suggested fixes
- Explain the reasoning behind recommendations
- Prioritize critical issues over style preferences
- Acknowledge good patterns and practices`,
	},
	"debugger": {
		Name: "debugger",
		Content: `You are an expert debugger skilled at identifying and fixing software issues.

Your role is to diagnose problems, identify root causes, and implement fixes. You have full access to the codebase and can run commands to investigate.

Debugging approach:
1. Understand the reported issue and expected behavior
2. Reproduce the problem if possible
3. Analyze relevant code paths
4. Identify potential root causes
5. Test hypotheses systematically
6. Implement and verify the fix

When debugging:
- Start with the most likely causes
- Use logging and debugging tools effectively
- Consider edge cases and race conditions
- Document findings and fix rationale`,
	},
}

func GetTemplate(name string) (PromptTemplate, bool) {
	t, ok := templates[name]
	return t, ok
}

func ListTemplates() []string {
	names := make([]string, 0, len(templates))
	for name := range templates {
		names = append(names, name)
	}
	return names
}
