package services

import (
	"encoding/json"
	"fmt"
	"time"

	"station/internal/db/repositories"
	"station/pkg/crypto"
	"station/pkg/models"
)

// MCPConfigService handles MCP configuration management with encryption
type MCPConfigService struct {
	repos      *repositories.Repositories
	keyManager *crypto.KeyManager
}

// NewMCPConfigService creates a new MCP config service
func NewMCPConfigService(repos *repositories.Repositories, keyManager *crypto.KeyManager) *MCPConfigService {
	return &MCPConfigService{
		repos:      repos,
		keyManager: keyManager,
	}
}

// UploadConfig encrypts and stores an MCP configuration
func (s *MCPConfigService) UploadConfig(environmentID int64, configData *models.MCPConfigData) (*models.MCPConfig, error) {
	// Use the config name from the data, fallback to default if empty
	configName := configData.Name
	if configName == "" {
		configName = "default"
	}

	// Get the next version number for this named config
	nextVersion, err := s.repos.MCPConfigs.GetNextVersion(environmentID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to get next version: %w", err)
	}

	// Serialize the config data
	configJSON, err := json.Marshal(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}

	// Encrypt the configuration
	encryptedConfig, keyID, err := s.keyManager.EncryptWithVersion(configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt config: %w", err)
	}

	// Store the encrypted config
	mcpConfig, err := s.repos.MCPConfigs.Create(
		environmentID,
		configName,
		nextVersion,
		string(encryptedConfig),
		keyID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store config: %w", err)
	}

	// Note: Tool discovery and replacement is now handled by ToolDiscoveryService.ReplaceToolsWithTransaction
	// This is called from the UI after the config is saved to ensure transactional consistency

	return mcpConfig, nil
}

// GetDecryptedConfig retrieves and decrypts an MCP configuration
func (s *MCPConfigService) GetDecryptedConfig(configID int64) (*models.MCPConfigData, error) {
	// Get the encrypted config
	config, err := s.repos.MCPConfigs.GetByID(configID)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// Decrypt the configuration
	decryptedData, err := s.keyManager.DecryptWithVersion(
		[]byte(config.ConfigJSON),
		config.EncryptionKeyID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// Deserialize the config data
	var configData models.MCPConfigData
	if err := json.Unmarshal(decryptedData, &configData); err != nil {
		return nil, fmt.Errorf("failed to deserialize config: %w", err)
	}

	return &configData, nil
}

// GetLatestConfigForEnvironment gets the latest decrypted config for an environment
func (s *MCPConfigService) GetLatestConfigForEnvironment(environmentID int64) (*models.MCPConfigData, *models.MCPConfig, error) {
	// Get the latest config
	config, err := s.repos.MCPConfigs.GetLatest(environmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get latest config: %w", err)
	}

	// Decrypt it
	configData, err := s.GetDecryptedConfig(config.ID)
	if err != nil {
		return nil, nil, err
	}

	return configData, config, nil
}

// RotateEncryptionKeys rotates encryption keys for all MCP configs
func (s *MCPConfigService) RotateEncryptionKeys() error {
	// Get the current active key ID before rotation
	oldKeyVersion := s.keyManager.GetActiveKey()
	if oldKeyVersion == nil {
		return fmt.Errorf("no active key found")
	}
	oldKeyID := oldKeyVersion.ID

	// Rotate to a new key
	newKeyVersion, err := s.keyManager.RotateKey()
	if err != nil {
		return fmt.Errorf("failed to rotate key: %w", err)
	}

	// Define the re-encryption function
	reencryptFunc := func(ciphertext []byte, oldKeyID, newKeyID string) ([]byte, error) {
		newCiphertext, _, err := s.keyManager.MigrateData(ciphertext, oldKeyID)
		return newCiphertext, err
	}

	// Re-encrypt all configs using the old key
	if err := s.repos.MCPConfigs.RotateEncryptionKey(oldKeyID, newKeyVersion.ID, reencryptFunc); err != nil {
		return fmt.Errorf("failed to rotate encryption keys in database: %w", err)
	}

	return nil
}

// processConfig extracts servers and tools from the config and stores them
func (s *MCPConfigService) processConfig(configID int64, configData *models.MCPConfigData) error {
	// Clear existing servers and tools for this config (in case of update)
	if err := s.clearConfigServersAndTools(configID); err != nil {
		return fmt.Errorf("failed to clear existing servers: %w", err)
	}

	// Process each server in the config
	for serverName, serverConfig := range configData.Servers {
		// Create server record
		mcpServer := &models.MCPServer{
			MCPConfigID: configID,
			Name:        serverName,
			Command:     serverConfig.Command,
			Args:        serverConfig.Args,
			Env:         serverConfig.Env,
		}
		
		serverID, err := s.repos.MCPServers.Create(mcpServer)
		if err != nil {
			return fmt.Errorf("failed to create server %s: %w", serverName, err)
		}

		// For now, we'll just create placeholder tools
		// In a real implementation, you would connect to the MCP server
		// and discover its available tools
		if err := s.discoverAndStoreTools(serverID, serverName, serverConfig); err != nil {
			return fmt.Errorf("failed to discover tools for server %s: %w", serverName, err)
		}
	}

	return nil
}

// clearConfigServersAndTools removes existing servers and tools for a config
func (s *MCPConfigService) clearConfigServersAndTools(configID int64) error {
	// Get all servers for this config
	servers, err := s.repos.MCPServers.GetByConfigID(configID)
	if err != nil {
		return err
	}

	// Delete tools for each server
	for _, server := range servers {
		if err := s.repos.MCPTools.DeleteByServerID(server.ID); err != nil {
			return err
		}
	}

	// Delete the servers
	return s.repos.MCPServers.DeleteByConfigID(configID)
}

// discoverAndStoreTools connects to an MCP server and discovers its tools
func (s *MCPConfigService) discoverAndStoreTools(serverID int64, serverName string, serverConfig models.MCPServerConfig) error {
	// This is a placeholder implementation
	// In a real implementation, you would:
	// 1. Start the MCP server process using serverConfig.Command and serverConfig.Args
	// 2. Connect to it via stdin/stdout or HTTP
	// 3. Send a "list_tools" request
	// 4. Parse the response and create tool records

	// For now, create some dummy tools based on common MCP server patterns
	commonTools := []struct {
		name        string
		description string
	}{
		{"read_file", "Read contents of a file"},
		{"write_file", "Write contents to a file"},
		{"list_directory", "List contents of a directory"},
		{"execute_command", "Execute a shell command"},
	}

	for _, tool := range commonTools {
		mcpTool := &models.MCPTool{
			MCPServerID: serverID,
			Name:        tool.name,
			Description: tool.description,
			Schema:      []byte(`{"type":"object"}`), // Basic JSON schema
		}
		
		_, err := s.repos.MCPTools.Create(mcpTool)
		if err != nil {
			return fmt.Errorf("failed to create tool %s: %w", tool.name, err)
		}
	}

	return nil
}

// ListConfigsByEnvironment lists all configurations for an environment
func (s *MCPConfigService) ListConfigsByEnvironment(environmentID int64) ([]*models.MCPConfig, error) {
	return s.repos.MCPConfigs.ListByEnvironment(environmentID)
}

// GetKeyManagerStats returns statistics about the key manager
func (s *MCPConfigService) GetKeyManagerStats() map[string]interface{} {
	keys := s.keyManager.ListKeys()
	activeKey := s.keyManager.GetActiveKey()

	stats := map[string]interface{}{
		"total_keys":     len(keys),
		"active_key_id":  activeKey.ID,
		"active_key_age": time.Since(activeKey.CreatedAt).String(),
	}

	return stats
}

// DecryptConfig decrypts a raw encrypted config string with a specific key ID
func (s *MCPConfigService) DecryptConfigWithKeyID(encryptedConfig string, keyID string) (*models.MCPConfigData, error) {
	// Decrypt the configuration using the specific key ID
	decryptedData, err := s.keyManager.DecryptWithVersion(
		[]byte(encryptedConfig),
		keyID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// Deserialize the config data
	var configData models.MCPConfigData
	if err := json.Unmarshal(decryptedData, &configData); err != nil {
		return nil, fmt.Errorf("failed to deserialize config: %w", err)
	}

	return &configData, nil
}

// DecryptConfig decrypts a raw encrypted config string using active key (deprecated - use DecryptConfigWithKeyID)
func (s *MCPConfigService) DecryptConfig(encryptedConfig string) (*models.MCPConfigData, error) {
	activeKey := s.keyManager.GetActiveKey()
	if activeKey == nil {
		return nil, fmt.Errorf("no active encryption key found")
	}
	
	return s.DecryptConfigWithKeyID(encryptedConfig, activeKey.ID)
}