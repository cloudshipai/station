package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"station/internal/db"
	"station/internal/theme"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile      string
	themeManager *theme.ThemeManager
	rootCmd      = &cobra.Command{
		Use:   "stn",
		Short: "Station - AI Agent Management Platform",
		Long: `Station is a secure, self-hosted platform for managing AI agents with MCP tool integration.
It provides a retro terminal interface for system administration and agent management.`,
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initTheme)

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/station/config.yaml)")
	
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(keyCmd)
	rootCmd.AddCommand(loadCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(blastoffCmd)
	rootCmd.AddCommand(bannerCmd)
	
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(themeCmd)
	
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeSetCmd)
	themeCmd.AddCommand(themePreviewCmd)
	themeCmd.AddCommand(themeSelectCmd)
	
	keyCmd.AddCommand(keyGenerateCmd)
	keyCmd.AddCommand(keySetCmd)
	keyCmd.AddCommand(keyRotateCmd)
	keyCmd.AddCommand(keyStatusCmd)
	keyCmd.AddCommand(keyFinishRotationCmd)

	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envGetCmd)
	envCmd.AddCommand(envUpdateCmd)
	envCmd.AddCommand(envDeleteCmd)
	
	// Serve command flags
	serveCmd.Flags().Int("ssh-port", 2222, "SSH server port")
	serveCmd.Flags().Int("mcp-port", 3000, "MCP server port") 
	serveCmd.Flags().Int("api-port", 8080, "API server port")
	serveCmd.Flags().String("database", "station.db", "Database file path")
	serveCmd.Flags().Bool("debug", false, "Enable debug logging")
	serveCmd.Flags().Bool("local", false, "Run in local mode (single user, no authentication)")
	
	// Load command flags
	loadCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	loadCmd.Flags().String("environment", "default", "Environment name to upload to")
	loadCmd.Flags().String("config-name", "", "Name for the MCP configuration")
	
	// MCP Add command flags
	mcpAddCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpAddCmd.Flags().String("environment", "default", "Environment name")
	mcpAddCmd.Flags().String("config-id", "", "Config ID to add server to (format: id or configname)")
	mcpAddCmd.Flags().String("server-name", "", "Name for the new MCP server")
	mcpAddCmd.Flags().String("command", "", "Command to run the MCP server")
	mcpAddCmd.Flags().StringSlice("args", []string{}, "Arguments for the MCP server command")
	mcpAddCmd.Flags().StringToString("env", map[string]string{}, "Environment variables (key=value)")
	mcpAddCmd.Flags().BoolP("interactive", "i", false, "Interactive mode with forms")
	
	// MCP command flags
	mcpListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpListCmd.Flags().String("environment", "default", "Environment to list configs from")
	
	mcpToolsCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpToolsCmd.Flags().String("environment", "default", "Environment to list tools from")
	mcpToolsCmd.Flags().String("filter", "", "Filter tools by name or description")
	
	// Environment command flags
	envListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	envCreateCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	envCreateCmd.Flags().String("description", "", "Description for the environment")
	envGetCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	envUpdateCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	envUpdateCmd.Flags().String("name", "", "New name for the environment")
	envUpdateCmd.Flags().String("description", "", "New description for the environment")
	envDeleteCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	envDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion without prompt")
	
	// Bind flags to viper
	viper.BindPFlag("ssh_port", serveCmd.Flags().Lookup("ssh-port"))
	viper.BindPFlag("mcp_port", serveCmd.Flags().Lookup("mcp-port"))
	viper.BindPFlag("api_port", serveCmd.Flags().Lookup("api-port"))
	viper.BindPFlag("database_url", serveCmd.Flags().Lookup("database"))
	viper.BindPFlag("debug", serveCmd.Flags().Lookup("debug"))
	viper.BindPFlag("local_mode", serveCmd.Flags().Lookup("local"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Use XDG config directory
		configDir := getXDGConfigDir()
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("STATION")

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())
	}
}

func initTheme() {
	// Try to initialize theme manager with database
	// For CLI commands, we'll use fallback themes if database is not available
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		databasePath = "station.db"
	}
	
	// Check if database file exists and is accessible
	if _, err := os.Stat(databasePath); err == nil {
		// Database exists, try to connect
		if database, err := db.New(databasePath); err == nil {
			themeManager = theme.NewThemeManager(database)
			// Try to initialize built-in themes and load default theme
			ctx := context.Background()
			themeManager.InitializeBuiltInThemes(ctx)
			themeManager.LoadDefaultTheme(ctx)
		}
	}
	
	// If themeManager is still nil, commands will use fallback themes
}

func getXDGConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, "station")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}