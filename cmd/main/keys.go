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

// Key management command definitions
var (
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

	keyFinishRotationCmd = &cobra.Command{
		Use:   "finish-rotation",
		Short: "Complete encryption key rotation",
		Long:  "Complete the encryption key rotation by re-encrypting all data with the new key",
		RunE:  runKeyFinishRotation,
	}
)

// generateEncryptionKey generates a new 32-byte encryption key
func generateEncryptionKey() ([]byte, string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, "", err
	}
	encryptionKey := hex.EncodeToString(key)
	return key, encryptionKey, nil
}

func runKeyGenerate(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔐 Generating new encryption key...\n")
	
	_, encryptionKey, err := generateEncryptionKey()
	if err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}
	
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
	configDir := getWorkspacePath()
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
	_, newKey, err := generateEncryptionKey()
	if err != nil {
		return fmt.Errorf("failed to generate new encryption key: %w", err)
	}
	
	// Load current config
	configDir := getWorkspacePath()
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

func runKeyFinishRotation(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔄 Completing encryption key rotation...\n")
	
	// Load current config
	configDir := getWorkspacePath()
	configFile := filepath.Join(configDir, "config.yaml")
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("configuration not found. Please run 'station init' first")
	}
	
	currentKey := viper.GetString("encryption_key")
	previousKey := viper.GetString("previous_encryption_key")
	rotationStarted := viper.GetBool("key_rotation_started")
	
	if !rotationStarted {
		fmt.Printf("✅ No key rotation in progress\n")
		return nil
	}
	
	if previousKey == "" {
		return fmt.Errorf("no previous encryption key found - rotation state is invalid")
	}
	
	if currentKey == "" {
		return fmt.Errorf("no current encryption key found - rotation state is invalid")
	}
	
	fmt.Printf("Previous Key: %s...%s\n", previousKey[:8], previousKey[56:])
	fmt.Printf("Current Key:  %s...%s\n", currentKey[:8], currentKey[56:])
	fmt.Printf("\n🔐 Re-encrypting all data with new key...\n")
	
	// Set environment variables for the rotation process
	os.Setenv("ENCRYPTION_KEY", currentKey)
	os.Setenv("PREVIOUS_ENCRYPTION_KEY", previousKey)
	os.Setenv("DATABASE_URL", viper.GetString("database_url"))
	
	// Call the rotation function
	if err := runEncryptionRotation(); err != nil {
		return fmt.Errorf("failed to complete rotation: %w", err)
	}
	
	// Clear rotation flags
	viper.Set("key_rotation_started", false)
	viper.Set("previous_encryption_key", "")
	
	// Write config
	if err := viper.WriteConfig(); err != nil {
		fmt.Printf("⚠️  Data rotation completed, but failed to clear rotation flags: %v\n", err)
		fmt.Printf("   You may need to manually edit the config file: %s\n", configFile)
		return nil
	}
	
	fmt.Printf("✅ Encryption key rotation completed successfully!\n")
	fmt.Printf("📁 Config file updated: %s\n", configFile)
	fmt.Printf("🔐 All data is now encrypted with the new key\n")
	
	return nil
}