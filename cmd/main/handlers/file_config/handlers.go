package file_config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/afero"
	
	internalconfig "station/internal/config"
	"station/internal/filesystem"
	"station/internal/template"
	"station/internal/variables"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/config"
)

// FileConfigHandler handles file-based MCP configuration CLI commands
type FileConfigHandler struct {
	// hybridConfigService removed - using file-based configs only
	fileConfigService   *services.FileConfigService
	repos               *repositories.Repositories
}

// NewFileConfigHandler creates a new file config handler
func NewFileConfigHandler() *FileConfigHandler {
	// Get database path directly from config file or use fallback
	databasePath := getConfigDatabasePath()
	
	database, err := db.New(databasePath)
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}

	repos := repositories.New(database)
	
	// Initialize file config components
	fs := afero.NewOsFs()
	fileSystem := filesystem.NewConfigFileSystem(fs, "./config", "./config/vars")
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	// Create file config options
	fileConfigOptions := config.FileConfigOptions{
		ConfigDir:       "./config",
		VariablesDir:    "./config/vars",
		Strategy:        config.StrategyTemplateFirst,
		AutoCreate:      true,
		BackupOnChange:  false,
		ValidateOnLoad:  true,
	}
	
	// Create file config manager
	fileConfigManager := internalconfig.NewFileConfigManager(
		fileSystem,
		templateEngine,
		variableStore,
		fileConfigOptions,
		repos.Environments,
	)
	
	// Initialize services (updated for file-based configs only)
	toolDiscoveryService := services.NewToolDiscoveryService(repos)
	fileConfigService := services.NewFileConfigService(fileConfigManager, toolDiscoveryService, repos)
	
	return &FileConfigHandler{
		fileConfigService:   fileConfigService,
		repos:               repos,
	}
}

// RegisterCommands registers all file config commands (deprecated - use RegisterMCPCommands)
func (h *FileConfigHandler) RegisterCommands(rootCmd *cobra.Command) {
	// Create the file-config command group
	fileConfigCmd := &cobra.Command{
		Use:   "file-config",
		Short: "Manage file-based MCP configurations",
		Long: `File-based configuration management for MCP servers.
Supports GitOps workflows with template-specific variable resolution.`,
	}

	// Add subcommands
	fileConfigCmd.AddCommand(h.listCommand())
	fileConfigCmd.AddCommand(h.createCommand())
	fileConfigCmd.AddCommand(h.updateCommand())
	fileConfigCmd.AddCommand(h.deleteCommand())
	fileConfigCmd.AddCommand(h.validateCommand())
	fileConfigCmd.AddCommand(h.discoverCommand())
	fileConfigCmd.AddCommand(h.statusCommand())
	fileConfigCmd.AddCommand(h.initCommand())
	fileConfigCmd.AddCommand(h.variablesCommand())
	
	rootCmd.AddCommand(fileConfigCmd)
}

// RegisterMCPCommands integrates file-based commands into the mcp command structure
func (h *FileConfigHandler) RegisterMCPCommands(mcpCmd *cobra.Command) {
	// Add file-based subcommands to existing mcp command with single-word names
	createCmd := h.createCommand()
	createCmd.Use = "create <config-name> [environment-name]"
	createCmd.Short = "Create a new MCP configuration from template"
	mcpCmd.AddCommand(createCmd)
	
	mcpCmd.AddCommand(h.updateCommand()) 
	mcpCmd.AddCommand(h.validateCommand())
	
	// Rename discover to single word
	discoverCmd := h.discoverCommand()
	discoverCmd.Use = "discover <config-name> [environment-name]"
	discoverCmd.Short = "Discover and register tools for an MCP configuration"
	mcpCmd.AddCommand(discoverCmd)
	
	// Commented out to avoid conflict with new sync/status commands
	// mcpCmd.AddCommand(h.statusCommand())
	mcpCmd.AddCommand(h.initCommand())
	mcpCmd.AddCommand(h.variablesCommand())
	
	// Add environment management under MCP
	mcpCmd.AddCommand(h.environmentsCommand())
	
	// Update the existing list command to support both file and database configs
	// We'll modify the existing mcp list to include file configs by default
}