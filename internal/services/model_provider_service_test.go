package services

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"station/internal/db/repositories"
	"station/pkg/crypto"
)

// setupTestDBForModelProvider creates a test database with model provider tables
func setupTestDBForModelProvider(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create model provider and models tables
	schema := `
	CREATE TABLE model_providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL,
		base_url TEXT NOT NULL,
		api_key TEXT,
		headers TEXT,
		enabled BOOLEAN DEFAULT true,
		is_default BOOLEAN DEFAULT false,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE models (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id INTEGER NOT NULL,
		model_id TEXT NOT NULL,
		name TEXT NOT NULL,
		context_size INTEGER NOT NULL,
		max_tokens INTEGER NOT NULL,
		supports_tools BOOLEAN DEFAULT false,
		input_cost REAL,
		output_cost REAL,
		enabled BOOLEAN DEFAULT true,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (provider_id) REFERENCES model_providers (id),
		UNIQUE (provider_id, model_id)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	return db
}


// createTestKeyManager creates a real KeyManager for testing
func createTestKeyManager(t *testing.T) *crypto.KeyManager {
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	return crypto.NewKeyManager(key)
}

// mockFailingKeyManager creates a mock that can simulate failures
type mockFailingKeyManager struct {
	realKeyManager    *crypto.KeyManager
	shouldFailEncrypt bool
	shouldFailDecrypt bool
}

func (m *mockFailingKeyManager) EncryptWithVersion(data []byte) ([]byte, string, error) {
	if m.shouldFailEncrypt {
		return nil, "", fmt.Errorf("encryption failed")
	}
	return m.realKeyManager.EncryptWithVersion(data)
}

func (m *mockFailingKeyManager) DecryptWithVersion(data []byte, keyID string) ([]byte, error) {
	if m.shouldFailDecrypt {
		return nil, fmt.Errorf("decryption failed")
	}
	return m.realKeyManager.DecryptWithVersion(data, keyID)
}

// GetActiveKey implements the KeyManager interface
func (m *mockFailingKeyManager) GetActiveKey() *crypto.KeyVersion {
	return m.realKeyManager.GetActiveKey()
}

// GetKeyByID implements the KeyManager interface
func (m *mockFailingKeyManager) GetKeyByID(keyID string) (*crypto.KeyVersion, error) {
	return m.realKeyManager.GetKeyByID(keyID)
}

// RotateKey implements the KeyManager interface
func (m *mockFailingKeyManager) RotateKey() (*crypto.KeyVersion, error) {
	return m.realKeyManager.RotateKey()
}

// ListKeys implements the KeyManager interface
func (m *mockFailingKeyManager) ListKeys() []*crypto.KeyVersion {
	return m.realKeyManager.ListKeys()
}

// AddKey implements the KeyManager interface
func (m *mockFailingKeyManager) AddKey(keyVersion *crypto.KeyVersion) {
	m.realKeyManager.AddKey(keyVersion)
}

// MigrateData implements the KeyManager interface
func (m *mockFailingKeyManager) MigrateData(oldCiphertext []byte, oldKeyID string) ([]byte, string, error) {
	return m.realKeyManager.MigrateData(oldCiphertext, oldKeyID)
}

func TestNewModelProviderBootService_Success(t *testing.T) {
	db := setupTestDBForModelProvider(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	keyManager := createTestKeyManager(t)

	service, err := NewModelProviderBootService(repos, keyManager)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if service == nil {
		t.Fatal("Expected service to be created")
	}
	if service.repos != repos {
		t.Error("Expected repos to be set")
	}
	if service.keyManager == nil {
		t.Error("Expected keyManager to be set")
	}
	if service.configPath == "" {
		t.Error("Expected configPath to be set")
	}
}

func TestEncryptSensitiveData_Success(t *testing.T) {
	service := &ModelProviderBootService{
		keyManager: createTestKeyManager(t),
	}

	config := &ProviderConfig{
		APIKey: "test-api-key",
		Headers: map[string]string{
			"Authorization": "Bearer token",
			"X-Custom":      "custom-header",
		},
	}

	encryptedAPIKey, encryptedHeaders, err := service.encryptSensitiveData(config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify API key format: keyID:base64EncodedData
	parts := strings.SplitN(encryptedAPIKey, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("Expected encrypted API key format 'keyID:data', got: %s", encryptedAPIKey)
	}
	if parts[0] == "" {
		t.Error("Expected non-empty key ID")
	}

	// Verify base64 decoding works
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("Failed to decode base64 data: %v", err)
	}

	// Decrypt and verify content
	decrypted, err := service.keyManager.DecryptWithVersion(decoded, parts[0])
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}
	if string(decrypted) != "test-api-key" {
		t.Errorf("Expected decrypted data to be 'test-api-key', got: %s", string(decrypted))
	}

	// Verify headers format
	encryptedValue, exists := encryptedHeaders["encrypted"]
	if !exists {
		t.Fatal("Expected 'encrypted' key in headers")
	}

	parts = strings.SplitN(encryptedValue, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("Expected encrypted headers format 'keyID:data', got: %s", encryptedValue)
	}

	decoded, err = base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("Failed to decode headers base64 data: %v", err)
	}

	// Decrypt and verify headers content
	decryptedHeaders, err := service.keyManager.DecryptWithVersion(decoded, parts[0])
	if err != nil {
		t.Fatalf("Failed to decrypt headers: %v", err)
	}

	var headersMap map[string]string
	if err := json.Unmarshal(decryptedHeaders, &headersMap); err != nil {
		t.Fatalf("Failed to unmarshal decrypted headers: %v", err)
	}

	if headersMap["Authorization"] != "Bearer token" {
		t.Errorf("Expected Authorization header, got: %v", headersMap)
	}
}

func TestEncryptSensitiveData_EmptyData(t *testing.T) {
	service := &ModelProviderBootService{
		keyManager: createTestKeyManager(t),
	}

	config := &ProviderConfig{
		APIKey:  "",
		Headers: nil,
	}

	encryptedAPIKey, encryptedHeaders, err := service.encryptSensitiveData(config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if encryptedAPIKey != "" {
		t.Errorf("Expected empty API key, got: %s", encryptedAPIKey)
	}
	if encryptedHeaders != nil {
		t.Errorf("Expected nil headers, got: %v", encryptedHeaders)
	}
}

// TestEncryptSensitiveData_EncryptionFailure - Skip this test since KeyManager is concrete
// In a real implementation, we would test this by providing invalid keys or simulating disk failures

func TestDecryptString_Success(t *testing.T) {
	keyManager := createTestKeyManager(t)
	service := &ModelProviderBootService{
		keyManager: keyManager,
	}

	// Create properly formatted encrypted string using real encryption
	originalData := "test-secret"
	encrypted, keyID, err := keyManager.EncryptWithVersion([]byte(originalData))
	if err != nil {
		t.Fatalf("Failed to encrypt test data: %v", err)
	}
	encodedData := base64.StdEncoding.EncodeToString(encrypted)
	encryptedString := fmt.Sprintf("%s:%s", keyID, encodedData)

	decrypted, err := service.decryptString(encryptedString)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if decrypted != originalData {
		t.Errorf("Expected '%s', got: '%s'", originalData, decrypted)
	}
}

func TestDecryptString_InvalidFormat(t *testing.T) {
	service := &ModelProviderBootService{
		keyManager: createTestKeyManager(t),
	}

	testCases := []struct {
		name          string
		encryptedData string
		expectedError string
	}{
		{
			name:          "no colon separator",
			encryptedData: "invalidformat",
			expectedError: "invalid encrypted string format",
		},
		{
			name:          "invalid base64",
			encryptedData: "test-key-id:invalid-base64!@#",
			expectedError: "failed to decode base64",
		},
		{
			name:          "empty string",
			encryptedData: "",
			expectedError: "invalid encrypted string format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.decryptString(tc.encryptedData)
			if err == nil {
				t.Fatal("Expected error")
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("Expected error containing '%s', got: %v", tc.expectedError, err)
			}
		})
	}
}

func TestDecryptString_WrongKeyID(t *testing.T) {
	keyManager1 := createTestKeyManager(t)
	keyManager2 := createTestKeyManager(t)
	
	service := &ModelProviderBootService{
		keyManager: keyManager2, // Different key manager
	}

	// Create encrypted string with keyManager1
	encrypted, keyID, err := keyManager1.EncryptWithVersion([]byte("test"))
	if err != nil {
		t.Fatalf("Failed to encrypt test data: %v", err)
	}
	encodedData := base64.StdEncoding.EncodeToString(encrypted)
	encryptedString := fmt.Sprintf("%s:%s", keyID, encodedData)

	// Try to decrypt with keyManager2 (should fail)
	_, err = service.decryptString(encryptedString)

	if err == nil {
		t.Fatal("Expected decryption error for wrong key")
	}
	if !strings.Contains(err.Error(), "failed to decrypt data") {
		t.Errorf("Expected decryption error, got: %v", err)
	}
}

func TestValidateConfiguration_Success(t *testing.T) {
	service := &ModelProviderBootService{}

	config := &ModelProviderConfig{
		Providers: map[string]*ProviderConfig{
			"openai": {
				Name:    "OpenAI",
				BaseURL: "https://api.openai.com/v1",
				Models: map[string]ModelInfo{
					"gpt-4": {
						ID:          "gpt-4",
						Name:        "GPT-4",
						ContextSize: 8192,
						MaxTokens:   4096,
						Enabled:     true,
					},
				},
				Enabled: true,
			},
		},
		Default: "openai",
	}

	err := service.validateConfiguration(config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestValidateConfiguration_Failures(t *testing.T) {
	service := &ModelProviderBootService{}

	testCases := []struct {
		name          string
		config        *ModelProviderConfig
		expectedError string
	}{
		{
			name:          "nil configuration",
			config:        nil,
			expectedError: "configuration is nil",
		},
		{
			name: "empty provider key",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"": {
						Name:    "Test",
						BaseURL: "https://example.com",
						Models:  map[string]ModelInfo{"model1": {ID: "model1", Name: "Model 1", ContextSize: 100}},
					},
				},
			},
			expectedError: "provider key cannot be empty",
		},
		{
			name: "empty provider name",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"test": {
						Name:    "",
						BaseURL: "https://example.com",
						Models:  map[string]ModelInfo{"model1": {ID: "model1", Name: "Model 1", ContextSize: 100}},
					},
				},
			},
			expectedError: "name cannot be empty",
		},
		{
			name: "empty base URL",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"test": {
						Name:    "Test",
						BaseURL: "",
						Models:  map[string]ModelInfo{"model1": {ID: "model1", Name: "Model 1", ContextSize: 100}},
					},
				},
			},
			expectedError: "base URL cannot be empty",
		},
		{
			name: "invalid URL",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"test": {
						Name:    "Test",
						BaseURL: "ht!@#$%tp://invalid-url",
						Models:  map[string]ModelInfo{"model1": {ID: "model1", Name: "Model 1", ContextSize: 100}},
					},
				},
			},
			expectedError: "invalid base URL",
		},
		{
			name: "no models",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"test": {
						Name:    "Test",
						BaseURL: "https://example.com",
						Models:  map[string]ModelInfo{},
					},
				},
			},
			expectedError: "must have at least one model",
		},
		{
			name: "empty model ID",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"test": {
						Name:    "Test",
						BaseURL: "https://example.com",
						Models: map[string]ModelInfo{
							"": {ID: "", Name: "Model", ContextSize: 100},
						},
					},
				},
			},
			expectedError: "model ID cannot be empty",
		},
		{
			name: "empty model name",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"test": {
						Name:    "Test",
						BaseURL: "https://example.com",
						Models: map[string]ModelInfo{
							"model1": {ID: "model1", Name: "", ContextSize: 100},
						},
					},
				},
			},
			expectedError: "name cannot be empty",
		},
		{
			name: "invalid context size",
			config: &ModelProviderConfig{
				Providers: map[string]*ProviderConfig{
					"test": {
						Name:    "Test",
						BaseURL: "https://example.com",
						Models: map[string]ModelInfo{
							"model1": {ID: "model1", Name: "Model 1", ContextSize: 0},
						},
					},
				},
			},
			expectedError: "context size must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.validateConfiguration(tc.config)
			if err == nil {
				t.Fatal("Expected validation error")
			}
			if !strings.Contains(err.Error(), tc.expectedError) {
				t.Errorf("Expected error containing '%s', got: %v", tc.expectedError, err)
			}
		})
	}
}

func TestDetectProvidersFromEnv(t *testing.T) {
	service := &ModelProviderBootService{}

	// Clean environment
	envVars := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY", "OLLAMA_BASE_URL"}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}

	t.Run("OpenAI detection", func(t *testing.T) {
		os.Setenv("OPENAI_API_KEY", "test-openai-key")
		defer os.Unsetenv("OPENAI_API_KEY")

		config := &ModelProviderConfig{
			Providers: make(map[string]*ProviderConfig),
		}

		service.detectProvidersFromEnv(config)

		openaiProvider, exists := config.Providers["openai"]
		if !exists {
			t.Fatal("Expected OpenAI provider to be detected")
		}
		if openaiProvider.Name != "OpenAI" {
			t.Errorf("Expected name 'OpenAI', got: %s", openaiProvider.Name)
		}
		if openaiProvider.APIKey != "test-openai-key" {
			t.Errorf("Expected API key 'test-openai-key', got: %s", openaiProvider.APIKey)
		}
		if !openaiProvider.IsDefault {
			t.Error("Expected OpenAI to be set as default")
		}
	})

	t.Run("Anthropic detection", func(t *testing.T) {
		os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
		defer os.Unsetenv("ANTHROPIC_API_KEY")

		config := &ModelProviderConfig{
			Providers: make(map[string]*ProviderConfig),
		}

		service.detectProvidersFromEnv(config)

		anthropicProvider, exists := config.Providers["anthropic"]
		if !exists {
			t.Fatal("Expected Anthropic provider to be detected")
		}
		if anthropicProvider.Name != "Anthropic" {
			t.Errorf("Expected name 'Anthropic', got: %s", anthropicProvider.Name)
		}
		if anthropicProvider.APIKey != "test-anthropic-key" {
			t.Errorf("Expected API key 'test-anthropic-key', got: %s", anthropicProvider.APIKey)
		}
		if anthropicProvider.Headers["anthropic-version"] != "2023-06-01" {
			t.Error("Expected anthropic-version header to be set")
		}
	})

	t.Run("OpenRouter detection", func(t *testing.T) {
		os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
		defer os.Unsetenv("OPENROUTER_API_KEY")

		config := &ModelProviderConfig{
			Providers: make(map[string]*ProviderConfig),
		}

		service.detectProvidersFromEnv(config)

		openrouterProvider, exists := config.Providers["openrouter"]
		if !exists {
			t.Fatal("Expected OpenRouter provider to be detected")
		}
		if openrouterProvider.Name != "OpenRouter" {
			t.Errorf("Expected name 'OpenRouter', got: %s", openrouterProvider.Name)
		}
		if openrouterProvider.APIKey != "test-openrouter-key" {
			t.Errorf("Expected API key 'test-openrouter-key', got: %s", openrouterProvider.APIKey)
		}
	})

	t.Run("Ollama detection", func(t *testing.T) {
		config := &ModelProviderConfig{
			Providers: make(map[string]*ProviderConfig),
		}

		service.detectProvidersFromEnv(config)

		ollamaProvider, exists := config.Providers["ollama"]
		if !exists {
			t.Fatal("Expected Ollama provider to be detected")
		}
		if ollamaProvider.Name != "Ollama" {
			t.Errorf("Expected name 'Ollama', got: %s", ollamaProvider.Name)
		}
		if ollamaProvider.APIKey != "" {
			t.Errorf("Expected empty API key for Ollama, got: %s", ollamaProvider.APIKey)
		}
		if !strings.Contains(ollamaProvider.BaseURL, "/v1") {
			t.Error("Expected Ollama base URL to contain /v1")
		}
	})

	t.Run("No environment variables", func(t *testing.T) {
		config := &ModelProviderConfig{
			Providers: make(map[string]*ProviderConfig),
		}

		service.detectProvidersFromEnv(config)

		// Only Ollama should be detected (uses default URL)
		if len(config.Providers) != 1 {
			t.Errorf("Expected 1 provider (Ollama), got: %d", len(config.Providers))
		}
		if _, exists := config.Providers["ollama"]; !exists {
			t.Error("Expected Ollama provider to be detected even without env vars")
		}
	})
}

func TestLoadConfiguration_FileAndEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "models.json")

	// Create test config file
	testConfig := `{
		"providers": {
			"custom": {
				"name": "Custom Provider",
				"base_url": "https://custom.example.com",
				"models": {
					"model-1": {
						"id": "model-1",
						"name": "Custom Model",
						"context_size": 4096,
						"max_tokens": 2048,
						"enabled": true
					}
				},
				"enabled": true
			}
		},
		"default": "custom"
	}`

	err := os.WriteFile(configPath, []byte(testConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set environment variable
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	service := &ModelProviderBootService{
		configPath: configPath,
	}

	config, err := service.loadConfiguration()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have both custom provider from file and OpenAI from environment
	if len(config.Providers) < 2 {
		t.Errorf("Expected at least 2 providers, got: %d", len(config.Providers))
	}

	customProvider, exists := config.Providers["custom"]
	if !exists {
		t.Fatal("Expected custom provider from file")
	}
	if customProvider.Name != "Custom Provider" {
		t.Errorf("Expected 'Custom Provider', got: %s", customProvider.Name)
	}

	openaiProvider, exists := config.Providers["openai"]
	if !exists {
		t.Fatal("Expected OpenAI provider from environment")
	}
	if openaiProvider.APIKey != "test-openai-key" {
		t.Errorf("Expected 'test-openai-key', got: %s", openaiProvider.APIKey)
	}

	if config.Default != "custom" {
		t.Errorf("Expected default 'custom', got: %s", config.Default)
	}
}

func TestLoadConfiguration_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "models.json")

	// Create invalid JSON file
	invalidJSON := `{"providers": {"test": invalid json}}`
	err := os.WriteFile(configPath, []byte(invalidJSON), 0600)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	service := &ModelProviderBootService{
		configPath: configPath,
	}

	_, err = service.loadConfiguration()

	if err == nil {
		t.Fatal("Expected JSON parse error")
	}
	if !strings.Contains(err.Error(), "failed to parse config file") {
		t.Errorf("Expected JSON parse error, got: %v", err)
	}
}


func TestLoadConfiguration_MissingFile(t *testing.T) {
	service := &ModelProviderBootService{
		configPath: "/nonexistent/path/models.json",
	}

	// Should not error when config file doesn't exist
	config, err := service.loadConfiguration()

	if err != nil {
		t.Fatalf("Expected no error for missing file, got: %v", err)
	}

	// Should still detect providers from environment (at least Ollama)
	if len(config.Providers) == 0 {
		t.Error("Expected at least one provider from environment detection")
	}

	if config.Default != "openai" {
		t.Errorf("Expected default fallback 'openai', got: %s", config.Default)
	}
}