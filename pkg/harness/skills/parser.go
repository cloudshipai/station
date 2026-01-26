package skills

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseSkillMetadata extracts YAML frontmatter from SKILL.md content
func ParseSkillMetadata(content []byte, path, dirName string) (*SkillMetadata, error) {
	contentStr := string(content)

	// Extract YAML frontmatter between --- markers
	if !strings.HasPrefix(contentStr, "---") {
		return nil, fmt.Errorf("no frontmatter found in %s", path)
	}

	// Find the closing ---
	rest := contentStr[3:]
	parts := strings.SplitN(rest, "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("frontmatter not closed in %s", path)
	}

	frontmatter := strings.TrimSpace(parts[0])

	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter in %s: %w", path, err)
	}

	// Validate name matches directory if name is specified
	if metadata.Name != "" && metadata.Name != dirName {
		return nil, fmt.Errorf("skill name %q must match directory %q in %s", metadata.Name, dirName, path)
	}

	// Use directory name if name not specified
	if metadata.Name == "" {
		metadata.Name = dirName
	}

	metadata.Path = path
	return &metadata, nil
}

// GetSkillBody returns the content after the frontmatter (the actual instructions)
func GetSkillBody(content []byte) string {
	contentStr := string(content)

	if !strings.HasPrefix(contentStr, "---") {
		return contentStr
	}

	rest := contentStr[3:]
	parts := strings.SplitN(rest, "---", 2)
	if len(parts) < 2 {
		return contentStr
	}

	return strings.TrimSpace(parts[1])
}
