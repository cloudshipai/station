package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/crypto"
)

// runEncryptionRotation performs the actual encryption key rotation process
func runEncryptionRotation() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Create repositories
	repos := repositories.New(database)

	// Create key managers for both keys
	currentKeyManager, err := crypto.NewKeyManagerFromEnv()
	if err != nil {
		return fmt.Errorf("failed to create current key manager: %w", err)
	}

	// Create a key manager that can handle both current and previous keys
	previousKeyHex := os.Getenv("PREVIOUS_ENCRYPTION_KEY")
	if previousKeyHex == "" {
		return fmt.Errorf("PREVIOUS_ENCRYPTION_KEY environment variable is required")
	}

	// Decode the previous key
	previousKeyBytes, err := hex.DecodeString(previousKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode previous key: %w", err)
	}

	if len(previousKeyBytes) != 32 {
		return fmt.Errorf("previous key must be 32 bytes (64 hex characters), got %d bytes", len(previousKeyBytes))
	}

	previousKey := &crypto.Key{}
	copy(previousKey[:], previousKeyBytes)

	// Add the previous key to the current key manager
	previousKeyVersion := &crypto.KeyVersion{
		ID:        generateKeyID(previousKey[:]),
		Key:       previousKey,
		IsActive:  false,
	}
	currentKeyManager.AddKey(previousKeyVersion)

	fmt.Printf("🔍 Scanning for encrypted data to migrate...\n")

	// Get all MCP configs that need rotation
	mcpConfigs, err := repos.MCPConfigs.ListAll()
	if err != nil {
		return fmt.Errorf("failed to list MCP configs: %w", err)
	}

	fmt.Printf("   Generated previous key ID: %s\n", previousKeyVersion.ID)
	fmt.Printf("   Found %d total MCP configs\n", len(mcpConfigs))

	configsToMigrate := 0
	for _, config := range mcpConfigs {
		fmt.Printf("   Config %d has key ID: %s\n", config.ID, config.EncryptionKeyID)
		if config.EncryptionKeyID == previousKeyVersion.ID {
			configsToMigrate++
		}
	}

	if configsToMigrate == 0 {
		fmt.Printf("✅ No MCP configs need migration\n")
		return nil
	}

	fmt.Printf("📋 Found %d MCP configs to migrate\n", configsToMigrate)

	// Migrate each config
	migratedCount := 0
	for _, config := range mcpConfigs {
		if config.EncryptionKeyID == previousKeyVersion.ID {
			fmt.Printf("   🔄 Migrating config %d (name: %s)...\n", config.ID, config.ConfigName)

			// Re-encrypt the data
			newCiphertext, newKeyID, err := currentKeyManager.MigrateData(
				[]byte(config.ConfigJSON),
				config.EncryptionKeyID,
			)
			if err != nil {
				return fmt.Errorf("failed to migrate config %d: %w", config.ID, err)
			}

			// Update the database record
			if err := repos.MCPConfigs.UpdateEncryption(config.ID, string(newCiphertext), newKeyID); err != nil {
				return fmt.Errorf("failed to update config %d encryption: %w", config.ID, err)
			}

			migratedCount++
			fmt.Printf("   ✅ Config %d migrated successfully\n", config.ID)
		}
	}

	fmt.Printf("🎉 Successfully migrated %d MCP configurations\n", migratedCount)
	return nil
}

// generateKeyID creates a deterministic identifier for a key version based on the key content
func generateKeyID(keyContent []byte) string {
	// This matches the logic in pkg/crypto/keymanager.go
	hash := sha256.Sum256(keyContent)
	return fmt.Sprintf("key_%02x%02x%02x%02x%02x%02x%02x%02x",
		hash[0], hash[1], hash[2], hash[3],
		hash[4], hash[5], hash[6], hash[7])
}