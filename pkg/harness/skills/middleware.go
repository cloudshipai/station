package skills

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// SkillsMiddleware loads and injects skills into the system prompt
type SkillsMiddleware struct {
	backend Backend
	sources []string
	logger  *slog.Logger
}

// NewSkillsMiddleware creates a new skills middleware with the given backend and sources
func NewSkillsMiddleware(backend Backend, sources []string) *SkillsMiddleware {
	return &SkillsMiddleware{
		backend: backend,
		sources: sources,
		logger:  slog.Default().With("component", "skills_middleware"),
	}
}

// LoadSkills discovers all skills from configured sources
// Later sources override earlier ones (project > user)
func (m *SkillsMiddleware) LoadSkills() ([]SkillMetadata, error) {
	allSkills := make(map[string]SkillMetadata)

	for _, source := range m.sources {
		expandedPath := expandPath(source)
		skills, err := m.loadSkillsFromSource(expandedPath)
		if err != nil {
			// Optional sources may not exist - just log and continue
			m.logger.Debug("skill source not found or error",
				"source", expandedPath,
				"error", err)
			continue
		}
		for _, skill := range skills {
			allSkills[skill.Name] = skill // Later wins
		}
	}

	result := make([]SkillMetadata, 0, len(allSkills))
	for _, skill := range allSkills {
		result = append(result, skill)
	}
	return result, nil
}

func (m *SkillsMiddleware) loadSkillsFromSource(basePath string) ([]SkillMetadata, error) {
	var skills []SkillMetadata

	entries, err := m.backend.ListDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list skills directory %s: %w", basePath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}

		skillPath := filepath.Join(basePath, entry.Name, "SKILL.md")
		content, err := m.backend.ReadFile(skillPath)
		if err != nil {
			// Skill directories without SKILL.md are skipped
			m.logger.Debug("SKILL.md not found",
				"path", skillPath,
				"error", err)
			continue
		}

		metadata, err := ParseSkillMetadata(content, skillPath, entry.Name)
		if err != nil {
			m.logger.Warn("failed to parse skill",
				"path", skillPath,
				"error", err)
			continue
		}

		skills = append(skills, *metadata)
		m.logger.Debug("loaded skill",
			"name", metadata.Name,
			"path", skillPath)
	}

	return skills, nil
}

// FormatSystemPromptSection generates the skills section for the system prompt
// This implements progressive disclosure - only skill names and descriptions
// are shown; the agent must read SKILL.md files for full instructions
func (m *SkillsMiddleware) FormatSystemPromptSection(skills []SkillMetadata) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Skills System\n\n")
	sb.WriteString("You have access to specialized skills that provide domain-specific workflows.\n\n")

	// Format available skills
	sb.WriteString("**Available Skills:**\n\n")
	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Name, skill.Description))
		if len(skill.AllowedTools) > 0 {
			sb.WriteString(fmt.Sprintf("  → Allowed tools: %s\n",
				strings.Join(skill.AllowedTools, ", ")))
		}
		sb.WriteString(fmt.Sprintf("  → Read `%s` for full instructions\n", skill.Path))
	}

	// Add usage instructions
	sb.WriteString(`
**How to Use Skills (Progressive Disclosure):**

1. **Recognize when a skill applies**: Check if the task matches a skill's description
2. **Read the skill's full instructions**: Use read_file with the path above
3. **Follow the skill's workflow**: SKILL.md contains step-by-step instructions
4. **Only read when needed**: Don't read skills you won't use (saves tokens)

**When to Use Skills:**
- User's request matches a skill's domain
- You need specialized knowledge or structured workflows
- A skill provides proven patterns for complex tasks
`)

	return sb.String()
}

// GetSkillInstructions reads the full instructions from a skill file
func (m *SkillsMiddleware) GetSkillInstructions(skillPath string) (string, error) {
	content, err := m.backend.ReadFile(skillPath)
	if err != nil {
		return "", fmt.Errorf("failed to read skill file: %w", err)
	}

	return GetSkillBody(content), nil
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

// DefaultSkillSources returns the default skill source paths
func DefaultSkillSources(envPath string) []string {
	return []string{
		filepath.Join(envPath, "skills", "user"),
		filepath.Join(envPath, "skills", "project"),
		".station/skills",
	}
}
