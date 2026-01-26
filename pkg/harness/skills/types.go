package skills

// SkillMetadata represents parsed SKILL.md frontmatter
type SkillMetadata struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	License      string   `yaml:"license,omitempty"`
	AllowedTools []string `yaml:"allowed-tools,omitempty"`
	Triggers     []string `yaml:"triggers,omitempty"`
	Path         string   `yaml:"-"` // Set after parsing, not from YAML
}

// SkillsConfig configures the skills middleware from agent frontmatter
type SkillsConfig struct {
	Sources []string `yaml:"sources,omitempty" json:"sources,omitempty"`
}

// DirEntry represents a directory entry from the backend
type DirEntry struct {
	Name  string
	IsDir bool
}

// Backend interface for file operations (can be workspace, sandbox, or direct FS)
type Backend interface {
	ListDir(path string) ([]DirEntry, error)
	ReadFile(path string) ([]byte, error)
}
