package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
)

// TestNewEnvironmentManagementService tests service creation
func TestNewEnvironmentManagementService(t *testing.T) {
	service := NewEnvironmentManagementService(nil)

	if service == nil {
		t.Fatal("NewEnvironmentManagementService returned nil")
	}
}

// TestCreateEnvironment tests environment creation with DB
func TestCreateEnvironment(t *testing.T) {
	// Setup test database
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	// Use temp directory instead of home directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	tests := []struct {
		name        string
		envName     string
		description *string
		userID      int64
		wantErr     bool
	}{
		{
			name:        "Valid environment creation",
			envName:     "test-env-1",
			description: stringPtr("Test environment"),
			userID:      1,
			wantErr:     false,
		},
		{
			name:        "Environment without description",
			envName:     "test-env-2",
			description: nil,
			userID:      1,
			wantErr:     false,
		},
		{
			name:        "Environment with empty name",
			envName:     "",
			description: nil,
			userID:      1,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, result, err := service.CreateEnvironment(tt.envName, tt.description, tt.userID)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if env == nil {
					t.Error("CreateEnvironment() returned nil environment")
				}
				if result == nil {
					t.Error("CreateEnvironment() returned nil result")
				}
				if !result.Success {
					t.Errorf("CreateEnvironment() result.Success = false, message: %s", result.Message)
				}
				if env != nil && env.Name != tt.envName {
					t.Errorf("Environment name = %s, want %s", env.Name, tt.envName)
				}

				// Verify directory was created
				if result != nil && result.DirectoryPath != "" {
					if _, err := os.Stat(result.DirectoryPath); os.IsNotExist(err) {
						t.Errorf("Environment directory not created: %s", result.DirectoryPath)
					}

					// Verify variables.yml was created
					if result.VariablesPath != "" {
						if _, err := os.Stat(result.VariablesPath); os.IsNotExist(err) {
							t.Errorf("Variables file not created: %s", result.VariablesPath)
						}
					}
				}
			}
		})
	}
}

// TestDeleteEnvironment tests environment deletion
func TestDeleteEnvironment(t *testing.T) {
	// Setup test database
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create environment first
	env, _, err := service.CreateEnvironment("test-delete-env", stringPtr("Test"), 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	tests := []struct {
		name        string
		envName     string
		wantSuccess bool
		wantMsg     string
	}{
		{
			name:        "Delete existing environment",
			envName:     env.Name,
			wantSuccess: true,
		},
		{
			name:        "Delete non-existent environment",
			envName:     "nonexistent",
			wantSuccess: false,
		},
		{
			name:        "Attempt to delete default environment",
			envName:     "default",
			wantSuccess: false,
			wantMsg:     "Cannot delete the default environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.DeleteEnvironment(tt.envName)

			if result == nil {
				t.Fatal("DeleteEnvironment() returned nil result")
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("DeleteEnvironment() success = %v, want %v. Message: %s", result.Success, tt.wantSuccess, result.Message)
			}

			if tt.wantMsg != "" && result.Message != tt.wantMsg {
				t.Errorf("DeleteEnvironment() message = %q, want %q", result.Message, tt.wantMsg)
			}
		})
	}
}

// TestDeleteEnvironmentByID tests deletion by ID
func TestDeleteEnvironmentByID(t *testing.T) {
	// Setup test database
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create environment first
	env, _, err := service.CreateEnvironment("test-delete-by-id", stringPtr("Test"), 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	result := service.DeleteEnvironmentByID(env.ID)
	if result == nil {
		t.Fatal("DeleteEnvironmentByID() returned nil result")
	}

	if !result.Success {
		t.Errorf("DeleteEnvironmentByID() failed: %s", result.Message)
	}
}

// TestGetEnvironmentFileConfig tests reading file config
func TestGetEnvironmentFileConfig(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create environment
	env, _, err := service.CreateEnvironment("test-get-config", stringPtr("Test"), 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	tests := []struct {
		name    string
		envName string
		wantErr bool
	}{
		{
			name:    "Get config for existing environment",
			envName: env.Name,
			wantErr: false,
		},
		{
			name:    "Get config for non-existent environment",
			envName: "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := service.GetEnvironmentFileConfig(tt.envName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetEnvironmentFileConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg == nil {
					t.Error("GetEnvironmentFileConfig() returned nil config")
				}
				if cfg["directory_path"] == "" {
					t.Error("Config missing directory_path")
				}
				if cfg["variables_yml"] == nil {
					t.Error("Config missing variables_yml")
				}
			}
		})
	}
}

// TestUpdateEnvironmentFileConfig tests file config updates
func TestUpdateEnvironmentFileConfig(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create environment
	env, _, err := service.CreateEnvironment("test-update-config", stringPtr("Test"), 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	tests := []struct {
		name     string
		envName  string
		filename string
		content  string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Update variables.yml",
			envName:  env.Name,
			filename: "variables.yml",
			content:  "TEST_VAR: value\n",
			wantErr:  false,
		},
		{
			name:     "Update template.json",
			envName:  env.Name,
			filename: "template.json",
			content:  `{"name": "test"}`,
			wantErr:  false,
		},
		{
			name:     "Update non-allowed file",
			envName:  env.Name,
			filename: "malicious.sh",
			content:  "rm -rf /",
			wantErr:  true,
			errMsg:   "not allowed",
		},
		{
			name:     "Directory traversal attempt",
			envName:  env.Name,
			filename: "../../../etc/passwd",
			content:  "malicious",
			wantErr:  true,
			errMsg:   "invalid filename",
		},
		{
			name:     "Update non-existent environment",
			envName:  "nonexistent",
			filename: "variables.yml",
			content:  "test",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.UpdateEnvironmentFileConfig(tt.envName, tt.filename, tt.content)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateEnvironmentFileConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("UpdateEnvironmentFileConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			}

			// Verify content was written for successful cases
			if !tt.wantErr {
				envDir := config.GetEnvironmentDir(tt.envName)
				filePath := filepath.Join(envDir, tt.filename)
				data, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("Failed to read updated file: %v", err)
				} else if string(data) != tt.content {
					t.Errorf("File content = %q, want %q", string(data), tt.content)
				}
			}
		})
	}
}

// TestEnvironmentOperationsEdgeCases tests edge cases
func TestEnvironmentOperationsEdgeCases(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	t.Run("Create environment with very long name", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("HOME", tmpDir)
		defer os.Unsetenv("HOME")

		longName := "this-is-a-very-long-environment-name-that-might-cause-filesystem-issues-or-database-constraints-to-trigger"
		_, result, err := service.CreateEnvironment(longName, nil, 1)

		// This might succeed or fail depending on constraints
		if err != nil {
			t.Logf("Long name rejected: %v", err)
		} else if result != nil && !result.Success {
			t.Logf("Long name rejected: %s", result.Message)
		}
	})

	t.Run("Create environment with special characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("HOME", tmpDir)
		defer os.Unsetenv("HOME")

		specialNames := []string{
			"test@env",
			"test env",
			"test/env",
			"test\\env",
			"test..env",
		}

		for _, name := range specialNames {
			_, result, err := service.CreateEnvironment(name, nil, 1)
			// Some should fail, some might succeed depending on OS
			if err != nil || (result != nil && !result.Success) {
				t.Logf("Special character name %q handled: err=%v, result=%v", name, err, result)
			}
		}
	})

	t.Run("Delete environment that doesn't exist in DB but has files", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("HOME", tmpDir)
		defer os.Unsetenv("HOME")

		// Create directory manually without DB entry
		envDir := filepath.Join(tmpDir, ".config", "station", "environments", "orphan-env")
		os.MkdirAll(envDir, 0755)

		result := service.DeleteEnvironment("orphan-env")
		if result.Success {
			t.Error("Should not succeed deleting environment not in database")
		}
	})

	t.Run("Concurrent environment creation", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("HOME", tmpDir)
		defer os.Unsetenv("HOME")

		done := make(chan bool, 3)
		errors := make(chan error, 3)

		for i := 0; i < 3; i++ {
			go func(n int) {
				_, _, err := service.CreateEnvironment(fmt.Sprintf("concurrent-%d", n), nil, 1)
				if err != nil {
					errors <- err
				}
				done <- true
			}(i)
		}

		// Wait for all
		for i := 0; i < 3; i++ {
			<-done
		}

		close(errors)
		errorCount := 0
		for err := range errors {
			t.Logf("Concurrent creation error: %v", err)
			errorCount++
		}

		t.Logf("Concurrent creation: %d errors out of 3 attempts", errorCount)
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

// Benchmark tests
func BenchmarkCreateEnvironment(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	tmpDir := b.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = service.CreateEnvironment(fmt.Sprintf("bench-env-%d", i), nil, 1)
	}
}

func BenchmarkGetEnvironmentFileConfig(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewEnvironmentManagementService(repos)

	tmpDir := b.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	env, _, err := service.CreateEnvironment("bench-env", nil, 1)
	if err != nil {
		b.Fatalf("Failed to create test environment: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetEnvironmentFileConfig(env.Name)
	}
}
