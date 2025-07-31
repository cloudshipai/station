package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "stn",
		Short: "Station - AI Agent Management Platform",
		Long: `Station is a secure, self-hosted platform for managing AI agents with MCP tool integration.
It provides a retro terminal interface for system administration and agent management.`,
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/station/config.yaml)")
	
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(keyCmd)
	rootCmd.AddCommand(loadCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(blastoffCmd)
	rootCmd.AddCommand(bannerCmd)
	
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	
	keyCmd.AddCommand(keyGenerateCmd)
	keyCmd.AddCommand(keySetCmd)
	keyCmd.AddCommand(keyRotateCmd)
	keyCmd.AddCommand(keyStatusCmd)
	keyCmd.AddCommand(keyFinishRotationCmd)

	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	
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