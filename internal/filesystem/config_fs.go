package filesystem

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
	"station/pkg/config"
)

// ConfigFileSystem implements the FileSystem interface using afero
type ConfigFileSystem struct {
	afero.Fs
	configDir string
	varsDir   string
}

// NewConfigFileSystem creates a new config filesystem with the given base filesystem
func NewConfigFileSystem(fs afero.Fs, configDir, varsDir string) *ConfigFileSystem {
	return &ConfigFileSystem{
		Fs:        fs,
		configDir: configDir,
		varsDir:   varsDir,
	}
}

// NewDefaultConfigFileSystem creates a filesystem using the OS filesystem
func NewDefaultConfigFileSystem(configDir, varsDir string) *ConfigFileSystem {
	return NewConfigFileSystem(afero.NewOsFs(), configDir, varsDir)
}

// NewMemoryConfigFileSystem creates an in-memory filesystem for testing
func NewMemoryConfigFileSystem(configDir, varsDir string) *ConfigFileSystem {
	return NewConfigFileSystem(afero.NewMemMapFs(), configDir, varsDir)
}

// SetBasePaths updates the base paths for config and variables
func (cfs *ConfigFileSystem) SetBasePaths(configDir, varsDir string) {
	cfs.configDir = configDir
	cfs.varsDir = varsDir
}

// EnsureConfigDir ensures the configuration directory structure exists for an environment
func (cfs *ConfigFileSystem) EnsureConfigDir(envName string) error {
	envDir := filepath.Join(cfs.configDir, "environments", envName)
	
	// Create main environment directory
	if err := cfs.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}
	
	// Create template-vars subdirectory
	templateVarsDir := filepath.Join(envDir, "template-vars")
	if err := cfs.MkdirAll(templateVarsDir, 0755); err != nil {
		return fmt.Errorf("failed to create template-vars directory: %w", err)
	}
	
	// Create variables directory (if different from config dir)
	if cfs.varsDir != "" && cfs.varsDir != cfs.configDir {
		varsEnvDir := filepath.Join(cfs.varsDir, "environments", envName)
		if err := cfs.MkdirAll(varsEnvDir, 0700); err != nil { // More restrictive for secrets
			return fmt.Errorf("failed to create variables directory: %w", err)
		}
	}
	
	return nil
}

// GetConfigPath returns the path to a specific configuration template
func (cfs *ConfigFileSystem) GetConfigPath(envName, configName string) string {
	return filepath.Join(cfs.configDir, "environments", envName, configName+".json")
}

// GetVariablesPath returns the path to the main variables file for an environment
func (cfs *ConfigFileSystem) GetVariablesPath(envName string) string {
	if cfs.varsDir != "" {
		return filepath.Join(cfs.varsDir, "environments", envName, "variables.env")
	}
	return filepath.Join(cfs.configDir, "environments", envName, "variables.env")
}

// GetTemplateVariablesPath returns the path to template-specific variables
func (cfs *ConfigFileSystem) GetTemplateVariablesPath(envName, templateName string) string {
	if cfs.varsDir != "" {
		return filepath.Join(cfs.varsDir, "environments", envName, "template-vars", templateName+".env")  
	}
	return filepath.Join(cfs.configDir, "environments", envName, "template-vars", templateName+".env")
}

// ListTemplates lists all template files in an environment
func (cfs *ConfigFileSystem) ListTemplates(envName string) ([]config.TemplateInfo, error) {
	envDir := filepath.Join(cfs.configDir, "environments", envName)
	
	exists, err := afero.DirExists(cfs, envDir)
	if err != nil {
		return nil, fmt.Errorf("failed to check environment directory: %w", err)
	}
	if !exists {
		return []config.TemplateInfo{}, nil
	}
	
	files, err := afero.ReadDir(cfs, envDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment directory: %w", err)
	}
	
	var templates []config.TemplateInfo
	for _, file := range files {
		if file.IsDir() || !isTemplateFile(file.Name()) {
			continue
		}
		
		templateName := getTemplateNameFromFile(file.Name())
		templateInfo := config.TemplateInfo{
			Name:    templateName,
			Path:    filepath.Join(envDir, file.Name()),
			Size:    file.Size(),
			ModTime: file.ModTime(),
		}
		
		// Check if template-specific variables exist
		templateVarsPath := cfs.GetTemplateVariablesPath(envName, templateName)
		if exists, _ := afero.Exists(cfs, templateVarsPath); exists {
			templateInfo.HasVars = true
			templateInfo.VarsPath = templateVarsPath
		}
		
		templates = append(templates, templateInfo)
	}
	
	return templates, nil
}

// CreateGitignore creates a .gitignore file to exclude secrets
func (cfs *ConfigFileSystem) CreateGitignore() error {
	gitignorePath := filepath.Join(cfs.configDir, ".gitignore")
	
	gitignoreContent := `# Station Configuration - Exclude secrets and variables
environments/*/variables.env
environments/*/template-vars/*.env
secrets/
*.env
*.key
*.pem

# Allow template files
!*.json
!*.yaml
!*.yml
`
	
	return afero.WriteFile(cfs, gitignorePath, []byte(gitignoreContent), 0644)
}

// CreateDirectoryStructure creates the recommended directory structure
func (cfs *ConfigFileSystem) CreateDirectoryStructure() error {
	// Create base directories
	dirs := []string{
		filepath.Join(cfs.configDir, "environments"),
		filepath.Join(cfs.configDir, "templates"),  // For shared templates
		filepath.Join(cfs.configDir, "schemas"),    // For validation schemas
	}
	
	if cfs.varsDir != "" && cfs.varsDir != cfs.configDir {
		dirs = append(dirs, 
			filepath.Join(cfs.varsDir, "environments"),
			filepath.Join(cfs.varsDir, "secrets"),
		)
	}
	
	for _, dir := range dirs {
		if err := cfs.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Create .gitignore
	if err := cfs.CreateGitignore(); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}
	
	return nil
}

// Helper functions

func isTemplateFile(filename string) bool {
	ext := filepath.Ext(filename)
	return ext == ".json" || ext == ".yaml" || ext == ".yml"
}

func getTemplateNameFromFile(filename string) string {
	return filename[:len(filename)-len(filepath.Ext(filename))]
}

// Ensure we implement the interface
var _ config.FileSystem = (*ConfigFileSystem)(nil)