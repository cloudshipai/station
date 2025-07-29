package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "station",
		Short: "Station - AI Agent Management Platform",
		Long: `Station is a secure, self-hosted platform for managing AI agents with MCP tool integration.
It provides a retro terminal interface for system administration and agent management.`,
	}

	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the Station server",
		Long:  "Start all Station services: SSH admin interface, MCP server, and REST API",
		RunE:  runServe,
	}

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize Station configuration",
		Long:  "Generate encryption keys and create configuration files in XDG config directory",
		RunE:  runInit,
	}

	configCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage Station configuration",
		Long:  "View and edit Station configuration settings",
	}

	configShowCmd = &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE:  runConfigShow,
	}

	configEditCmd = &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration file",
		RunE:  runConfigEdit,
	}

	keyCmd = &cobra.Command{
		Use:   "key",
		Short: "Manage encryption keys",
		Long:  "Generate, set, and rotate encryption keys for Station",
	}

	keyGenerateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate a new encryption key",
		Long:  "Generate a new 32-byte encryption key and display it",
		RunE:  runKeyGenerate,
	}

	keySetCmd = &cobra.Command{
		Use:   "set [key]",
		Short: "Set a specific encryption key",
		Long:  "Set a specific encryption key (64 hex characters)",
		Args:  cobra.ExactArgs(1),
		RunE:  runKeySet,
	}

	keyRotateCmd = &cobra.Command{
		Use:   "rotate",
		Short: "Rotate encryption key",
		Long:  "Generate a new encryption key and begin rotation process",
		RunE:  runKeyRotate,
	}

	keyStatusCmd = &cobra.Command{
		Use:   "status", 
		Short: "Show encryption key status",
		Long:  "Show current encryption key status and rotation state",
		RunE:  runKeyStatus,
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
	
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	
	keyCmd.AddCommand(keyGenerateCmd)
	keyCmd.AddCommand(keySetCmd)
	keyCmd.AddCommand(keyRotateCmd)
	keyCmd.AddCommand(keyStatusCmd)
	
	// Serve command flags
	serveCmd.Flags().Int("ssh-port", 2222, "SSH server port")
	serveCmd.Flags().Int("mcp-port", 3000, "MCP server port") 
	serveCmd.Flags().Int("api-port", 8080, "API server port")
	serveCmd.Flags().String("database", "station.db", "Database file path")
	serveCmd.Flags().Bool("debug", false, "Enable debug logging")
	
	// Bind flags to viper
	viper.BindPFlag("ssh_port", serveCmd.Flags().Lookup("ssh-port"))
	viper.BindPFlag("mcp_port", serveCmd.Flags().Lookup("mcp-port"))
	viper.BindPFlag("api_port", serveCmd.Flags().Lookup("api-port"))
	viper.BindPFlag("database_url", serveCmd.Flags().Lookup("database"))
	viper.BindPFlag("debug", serveCmd.Flags().Lookup("debug"))
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

func runServe(cmd *cobra.Command, args []string) error {
	// Check if configuration exists
	configDir := getXDGConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Configuration not found. Please run 'station init' first.\n")
		fmt.Printf("Expected config file: %s\n", configFile)
		return fmt.Errorf("configuration not initialized")
	}

	// Validate encryption key
	encryptionKey := viper.GetString("encryption_key")
	if encryptionKey == "" {
		return fmt.Errorf("encryption key not found in configuration. Please run 'station init' to generate keys")
	}

	fmt.Printf("🚀 Starting Station...\n")
	fmt.Printf("SSH Port: %d\n", viper.GetInt("ssh_port"))
	fmt.Printf("MCP Port: %d\n", viper.GetInt("mcp_port"))
	fmt.Printf("API Port: %d\n", viper.GetInt("api_port"))
	fmt.Printf("Database: %s\n", viper.GetString("database_url"))
	
	// Set environment variables for the main application to use
	os.Setenv("ENCRYPTION_KEY", encryptionKey)
	os.Setenv("SSH_PORT", fmt.Sprintf("%d", viper.GetInt("ssh_port")))
	os.Setenv("MCP_PORT", fmt.Sprintf("%d", viper.GetInt("mcp_port")))
	os.Setenv("API_PORT", fmt.Sprintf("%d", viper.GetInt("api_port")))
	os.Setenv("DATABASE_URL", viper.GetString("database_url"))
	if viper.GetBool("debug") {
		os.Setenv("STATION_DEBUG", "true")
	}

	// Import and run the main server code
	return runMainServer()
}

func runInit(cmd *cobra.Command, args []string) error {
	configDir := getXDGConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")

	fmt.Printf("🔧 Initializing Station configuration...\n")
	fmt.Printf("Config directory: %s\n", configDir)

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate encryption key
	fmt.Printf("🔐 Generating encryption key...\n")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}
	encryptionKey := hex.EncodeToString(key)

	// Set default configuration
	viper.Set("encryption_key", encryptionKey)
	viper.Set("ssh_port", 2222)
	viper.Set("mcp_port", 3000)
	viper.Set("api_port", 8080)
	viper.Set("database_url", "station.db")
	viper.Set("ssh_host_key_path", "./ssh_host_key")
	viper.Set("admin_username", "admin")
	viper.Set("debug", false)

	// Write configuration file
	viper.SetConfigFile(configFile)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Configuration initialized successfully!\n")
	fmt.Printf("📁 Config file: %s\n", configFile)
	fmt.Printf("🔑 Encryption key generated and saved securely\n")
	fmt.Printf("\n🚀 You can now run 'station serve' to launch the server\n")
	fmt.Printf("🔗 Connect via SSH: ssh admin@localhost -p 2222\n")

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no configuration file found. Run 'station init' first")
	}

	fmt.Printf("📋 Station Configuration\n")
	fmt.Printf("======================\n")
	fmt.Printf("Config file: %s\n\n", configFile)
	
	fmt.Printf("Server Ports:\n")
	fmt.Printf("  SSH Port: %d\n", viper.GetInt("ssh_port"))
	fmt.Printf("  MCP Port: %d\n", viper.GetInt("mcp_port"))
	fmt.Printf("  API Port: %d\n", viper.GetInt("api_port"))
	
	fmt.Printf("\nDatabase:\n")
	fmt.Printf("  Database URL: %s\n", viper.GetString("database_url"))
	
	fmt.Printf("\nSSH Configuration:\n")
	fmt.Printf("  Host Key Path: %s\n", viper.GetString("ssh_host_key_path"))
	fmt.Printf("  Admin Username: %s\n", viper.GetString("admin_username"))
	
	fmt.Printf("\nSecurity:\n")
	if viper.GetString("encryption_key") != "" {
		fmt.Printf("  Encryption Key: [CONFIGURED]\n")
	} else {
		fmt.Printf("  Encryption Key: [NOT SET]\n")
	}
	
	fmt.Printf("\nDebug Mode: %v\n", viper.GetBool("debug"))

	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no configuration file found. Run 'station init' first")
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano" // fallback editor
	}

	fmt.Printf("Opening config file with %s: %s\n", editor, configFile)
	
	// Execute editor command
	command := fmt.Sprintf("%s %s", editor, configFile)
	return runCommand(command)
}

func runCommand(command string) error {
	// This is a simplified version - in production you'd want proper command execution
	fmt.Printf("Would execute: %s\n", command)
	fmt.Printf("For now, manually edit: %s\n", viper.ConfigFileUsed())
	return nil
}

func runKeyGenerate(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔐 Generating new encryption key...\n")
	
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}
	encryptionKey := hex.EncodeToString(key)
	
	fmt.Printf("✅ New encryption key generated:\n")
	fmt.Printf("%s\n\n", encryptionKey)
	fmt.Printf("💡 To use this key, run: station key set %s\n", encryptionKey)
	fmt.Printf("⚠️  Keep this key secure - it encrypts all sensitive data!\n")
	
	return nil
}

func runKeySet(cmd *cobra.Command, args []string) error {
	newKey := args[0]
	
	// Validate key format
	if len(newKey) != 64 {
		return fmt.Errorf("encryption key must be exactly 64 hexadecimal characters (32 bytes)")
	}
	
	// Validate it's valid hex
	if _, err := hex.DecodeString(newKey); err != nil {
		return fmt.Errorf("encryption key must be valid hexadecimal: %w", err)
	}
	
	// Load current config
	configDir := getXDGConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("configuration not found. Please run 'station init' first")
	}
	
	// Backup current key for rotation
	currentKey := viper.GetString("encryption_key")
	if currentKey != "" && currentKey != newKey {
		viper.Set("previous_encryption_key", currentKey)
		viper.Set("key_rotation_started", true)
		fmt.Printf("🔄 Key rotation initiated - previous key backed up\n")
	}
	
	// Set new key
	viper.Set("encryption_key", newKey)
	
	// Write config
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}
	
	fmt.Printf("✅ Encryption key updated successfully!\n")
	fmt.Printf("📁 Config file: %s\n", configFile)
	
	if viper.GetBool("key_rotation_started") {
		fmt.Printf("\n⚠️  Key rotation in progress!\n")
		fmt.Printf("   - Existing encrypted data will be re-encrypted on next startup\n")
		fmt.Printf("   - Run 'station key status' to check rotation progress\n")
	}
	
	return nil
}

func runKeyRotate(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔄 Starting encryption key rotation...\n")
	
	// Generate new key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate new encryption key: %w", err)
	}
	newKey := hex.EncodeToString(key)
	
	// Load current config
	configDir := getXDGConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("configuration not found. Please run 'station init' first")
	}
	
	currentKey := viper.GetString("encryption_key")
	if currentKey == "" {
		return fmt.Errorf("no current encryption key found. Run 'station init' first")
	}
	
	// Backup current key and set rotation flag
	viper.Set("previous_encryption_key", currentKey)
	viper.Set("encryption_key", newKey)
	viper.Set("key_rotation_started", true)
	
	// Write config
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}
	
	fmt.Printf("✅ Key rotation initiated!\n")
	fmt.Printf("🔑 New key: %s\n", newKey)
	fmt.Printf("📁 Config file: %s\n", configFile)
	fmt.Printf("\n⚠️  Next steps:\n")
	fmt.Printf("   1. Restart Station to begin re-encryption process\n")
	fmt.Printf("   2. Monitor with 'station key status'\n")
	fmt.Printf("   3. All MCP configs and sensitive data will be re-encrypted\n")
	
	return nil
}

func runKeyStatus(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no configuration file found. Run 'station init' first")
	}
	
	fmt.Printf("🔐 Encryption Key Status\n")
	fmt.Printf("=======================\n")
	fmt.Printf("Config file: %s\n\n", configFile)
	
	currentKey := viper.GetString("encryption_key")
	previousKey := viper.GetString("previous_encryption_key")
	rotationStarted := viper.GetBool("key_rotation_started")
	
	if currentKey != "" {
		fmt.Printf("Current Key: %s...%s ✅\n", currentKey[:8], currentKey[56:])
	} else {
		fmt.Printf("Current Key: [NOT SET] ❌\n")
	}
	
	if rotationStarted {
		fmt.Printf("Rotation Status: 🔄 IN PROGRESS\n")
		if previousKey != "" {
			fmt.Printf("Previous Key: %s...%s (backed up)\n", previousKey[:8], previousKey[56:])
		}
		fmt.Printf("\n⚠️  Action Required:\n")
		fmt.Printf("   - Restart Station to complete rotation\n")
		fmt.Printf("   - All encrypted data will be migrated to new key\n")
	} else {
		fmt.Printf("Rotation Status: ✅ STABLE\n")
	}
	
	// Show what data uses encryption
	fmt.Printf("\n🔒 Encrypted Data:\n")
	fmt.Printf("   - MCP server configurations\n")
	fmt.Printf("   - Model provider API keys\n")
	fmt.Printf("   - Agent system prompts (if sensitive)\n")
	fmt.Printf("   - User SSH keys and tokens\n")
	
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}