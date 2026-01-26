package memory

// MemoryConfig configures the memory middleware from agent frontmatter
type MemoryConfig struct {
	Sources []string `yaml:"sources,omitempty" json:"sources,omitempty"`
}

// Backend interface for file operations
type Backend interface {
	ReadFile(path string) ([]byte, error)
}
