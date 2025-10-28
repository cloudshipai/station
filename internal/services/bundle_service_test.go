package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"
)

// TestNewBundleService tests service creation without repos
func TestNewBundleService(t *testing.T) {
	service := NewBundleService()

	if service == nil {
		t.Fatal("NewBundleService() returned nil")
	}

	if service.repos != nil {
		t.Error("Service repos should be nil for backwards compatibility")
	}
}

// TestNewBundleServiceWithRepos tests service creation with database support
func TestNewBundleServiceWithRepos(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewBundleServiceWithRepos(repos)

	if service == nil {
		t.Fatal("NewBundleServiceWithRepos() returned nil")
	}

	if service.repos == nil {
		t.Error("Service repos should not be nil")
	}

	if service.repos != repos {
		t.Error("Service repos not set correctly")
	}
}

// TestCreateBundle tests bundle creation from environment directory
func TestCreateBundle(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name        string
		setupFunc   func(string)
		expectError bool
		description string
	}{
		{
			name: "Create bundle from empty directory",
			setupFunc: func(dir string) {
				// Empty directory
			},
			expectError: false,
			description: "Should create bundle from empty environment",
		},
		{
			name: "Create bundle with agent files",
			setupFunc: func(dir string) {
				agentsDir := filepath.Join(dir, "agents")
				_ = os.MkdirAll(agentsDir, 0755)

				agentContent := `---
metadata:
  name: "test-agent"
  description: "Test agent"
  tags: ["test", "demo"]
model: gpt-4o-mini
max_steps: 5
tools:
  - "__read_text_file"
  - "__list_directory"
---

{{role "system"}}
You are a test agent.

{{role "user"}}
{{userInput}}
`
				os.WriteFile(filepath.Join(agentsDir, "test-agent.prompt"), []byte(agentContent), 0644)
			},
			expectError: false,
			description: "Should create bundle with agent files",
		},
		{
			name: "Create bundle with template.json",
			setupFunc: func(dir string) {
				templateJSON := `{
  "name": "test-template",
  "description": "Test template",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    }
  }
}`
				_ = os.WriteFile(filepath.Join(dir, "template.json"), []byte(templateJSON), 0644)
			},
			expectError: false,
			description: "Should create bundle with MCP template",
		},
		{
			name: "Non-existent directory",
			setupFunc: func(dir string) {
				// Don't create directory
			},
			expectError: true,
			description: "Should fail for non-existent directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.name != "Non-existent directory" {
				envDir := filepath.Join(tmpDir, "test-env")
				os.MkdirAll(envDir, 0755)
				if tt.setupFunc != nil {
					tt.setupFunc(envDir)
				}
				tmpDir = envDir
			} else {
				tmpDir = filepath.Join(tmpDir, "nonexistent")
			}

			bundle, err := service.CreateBundle(tmpDir)

			if (err != nil) != tt.expectError {
				t.Errorf("CreateBundle() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if len(bundle) == 0 {
					t.Error("CreateBundle() returned empty bundle")
				}

				// Verify it's valid gzip
				_, err := gzip.NewReader(bytes.NewReader(bundle))
				if err != nil {
					t.Errorf("Bundle is not valid gzip: %v", err)
				}
			}
		})
	}
}

// TestGenerateManifest tests manifest generation from environment
func TestGenerateManifest(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name              string
		setupFunc         func(string)
		expectAgents      int
		expectMCPServers  int
		expectTags        int
		expectError       bool
		description       string
	}{
		{
			name: "Empty environment",
			setupFunc: func(dir string) {
				// Empty directory
			},
			expectAgents:     0,
			expectMCPServers: 0,
			expectTags:       0,
			expectError:      false,
			description:      "Should generate empty manifest",
		},
		{
			name: "Environment with single agent",
			setupFunc: func(dir string) {
				agentsDir := filepath.Join(dir, "agents")
				_ = os.MkdirAll(agentsDir, 0755)

				agentContent := `---
metadata:
  name: "single-agent"
  description: "Single agent"
  tags: ["test"]
model: gpt-4o-mini
max_steps: 3
tools: []
---
Test content
`
				os.WriteFile(filepath.Join(agentsDir, "single-agent.prompt"), []byte(agentContent), 0644)
			},
			expectAgents:     1,
			expectMCPServers: 0,
			expectTags:       1,
			expectError:      false,
			description:      "Should include agent in manifest",
		},
		{
			name: "Environment with MCP servers",
			setupFunc: func(dir string) {
				templateJSON := `{
  "name": "mcp-template",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github@latest"]
    }
  }
}`
				_ = os.WriteFile(filepath.Join(dir, "template.json"), []byte(templateJSON), 0644)
			},
			expectAgents:     0,
			expectMCPServers: 2,
			expectTags:       0,
			expectError:      false,
			description:      "Should parse MCP servers from template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setupFunc != nil {
				tt.setupFunc(tmpDir)
			}

			manifest, err := service.generateManifest(tmpDir)

			if (err != nil) != tt.expectError {
				t.Errorf("generateManifest() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if manifest == nil {
					t.Fatal("generateManifest() returned nil manifest")
				}

				if len(manifest.Agents) != tt.expectAgents {
					t.Errorf("Agents count = %d, want %d", len(manifest.Agents), tt.expectAgents)
				}

				if len(manifest.MCPServers) != tt.expectMCPServers {
					t.Errorf("MCP servers count = %d, want %d", len(manifest.MCPServers), tt.expectMCPServers)
				}

				if len(manifest.Bundle.Tags) != tt.expectTags {
					t.Errorf("Tags count = %d, want %d", len(manifest.Bundle.Tags), tt.expectTags)
				}

				if manifest.Version != "1.0" {
					t.Errorf("Version = %s, want 1.0", manifest.Version)
				}

				if manifest.Bundle.Name == "" {
					t.Error("Bundle name should not be empty")
				}
			}
		})
	}
}

// TestParseAgentFile tests agent file parsing
func TestParseAgentFile(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name            string
		agentContent    string
		expectError     bool
		expectedName    string
		expectedModel   string
		expectedMaxSteps int
		expectedToolCount int
		description     string
	}{
		{
			name: "Valid agent file",
			agentContent: `---
metadata:
  name: "valid-agent"
  description: "A valid agent"
  tags: ["test", "valid"]
model: gpt-4o-mini
max_steps: 5
tools:
  - "__read_text_file"
  - "__list_directory"
---
Agent content here
`,
			expectError:      false,
			expectedName:     "valid-agent",
			expectedModel:    "gpt-4o-mini",
			expectedMaxSteps: 5,
			expectedToolCount: 2,
			description:      "Should parse valid agent file",
		},
		{
			name: "Agent with no tools",
			agentContent: `---
metadata:
  name: "no-tools-agent"
  description: "Agent without tools"
  tags: []
model: gpt-4o-mini
max_steps: 3
tools: []
---
Content
`,
			expectError:      false,
			expectedName:     "no-tools-agent",
			expectedModel:    "gpt-4o-mini",
			expectedMaxSteps: 3,
			expectedToolCount: 0,
			description:      "Should handle agent without tools",
		},
		{
			name: "Invalid format - no frontmatter",
			agentContent: `Just plain text content
without any YAML frontmatter
`,
			expectError:  true,
			description:  "Should fail on invalid format",
		},
		{
			name: "Invalid YAML",
			agentContent: `---
invalid: yaml: content: here
	tabs and bad indentation
---
Content
`,
			expectError:  true,
			description:  "Should fail on invalid YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "agent.prompt")
			err := os.WriteFile(tmpFile, []byte(tt.agentContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			agentInfo, err := service.parseAgentFile(tmpFile)

			if (err != nil) != tt.expectError {
				t.Errorf("parseAgentFile() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if agentInfo == nil {
					t.Fatal("parseAgentFile() returned nil")
				}

				if agentInfo.Name != tt.expectedName {
					t.Errorf("Name = %s, want %s", agentInfo.Name, tt.expectedName)
				}

				if agentInfo.Model != tt.expectedModel {
					t.Errorf("Model = %s, want %s", agentInfo.Model, tt.expectedModel)
				}

				if agentInfo.MaxSteps != tt.expectedMaxSteps {
					t.Errorf("MaxSteps = %d, want %d", agentInfo.MaxSteps, tt.expectedMaxSteps)
				}

				if len(agentInfo.Tools) != tt.expectedToolCount {
					t.Errorf("Tools count = %d, want %d", len(agentInfo.Tools), tt.expectedToolCount)
				}
			}
		})
	}
}

// TestParseMCPConfigFile tests MCP config parsing
func TestParseMCPConfigFile(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name                string
		configContent       string
		expectError         bool
		expectedServers     int
		expectedVariables   int
		description         string
	}{
		{
			name: "Valid MCP config",
			configContent: `{
  "name": "test-config",
  "description": "Test configuration",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "{{ .PROJECT_ROOT }}"]
    }
  }
}`,
			expectError:       false,
			expectedServers:   1,
			expectedVariables: 1,
			description:       "Should parse valid MCP config",
		},
		{
			name: "Multiple MCP servers",
			configContent: `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github@latest"]
    },
    "postgres": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres@latest"]
    }
  }
}`,
			expectError:       false,
			expectedServers:   3,
			expectedVariables: 0,
			description:       "Should parse multiple servers",
		},
		{
			name: "Empty MCP servers",
			configContent: `{
  "mcpServers": {}
}`,
			expectError:       false,
			expectedServers:   0,
			expectedVariables: 0,
			description:       "Should handle empty servers",
		},
		{
			name:          "Invalid JSON",
			configContent: `{ invalid json content }`,
			expectError:   true,
			description:   "Should fail on invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "config.json")
			err := os.WriteFile(tmpFile, []byte(tt.configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			servers, variables, err := service.parseMCPConfigFile(tmpFile)

			if (err != nil) != tt.expectError {
				t.Errorf("parseMCPConfigFile() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if len(servers) != tt.expectedServers {
					t.Errorf("Servers count = %d, want %d", len(servers), tt.expectedServers)
				}

				if len(variables) != tt.expectedVariables {
					t.Errorf("Variables count = %d, want %d", len(variables), tt.expectedVariables)
				}
			}
		})
	}
}

// TestValidateEnvironment tests environment validation
func TestValidateEnvironment(t *testing.T) {
	service := NewBundleService()

	t.Run("Validate valid environment", func(t *testing.T) {
		// Create valid environment directory
		sourceDir := t.TempDir()
		agentsDir := filepath.Join(sourceDir, "agents")
		os.MkdirAll(agentsDir, 0755)

		agentContent := `---
metadata:
  name: "test-agent"
model: gpt-4o-mini
max_steps: 3
tools: []
---
Content`
		os.WriteFile(filepath.Join(agentsDir, "test.prompt"), []byte(agentContent), 0644)

		err := service.ValidateEnvironment(sourceDir)
		if err != nil {
			t.Errorf("ValidateEnvironment() error = %v", err)
		}
	})

	t.Run("Validate non-existent environment", func(t *testing.T) {
		err := service.ValidateEnvironment("/nonexistent/path")
		if err == nil {
			t.Error("ValidateEnvironment() should fail for non-existent path")
		}
	})
}

// TestGetBundleInfo tests bundle info retrieval
func TestGetBundleInfo(t *testing.T) {
	service := NewBundleService()

	t.Run("Get info from valid environment", func(t *testing.T) {
		sourceDir := t.TempDir()
		agentsDir := filepath.Join(sourceDir, "agents")
		os.MkdirAll(agentsDir, 0755)

		agentContent := `---
metadata:
  name: "info-agent"
  description: "Test agent"
model: gpt-4o-mini
max_steps: 3
tools: []
---
Content`
		os.WriteFile(filepath.Join(agentsDir, "info-agent.prompt"), []byte(agentContent), 0644)

		info, err := service.GetBundleInfo(sourceDir)
		if err != nil {
			t.Errorf("GetBundleInfo() error = %v", err)
			return
		}

		if info == nil {
			t.Fatal("GetBundleInfo() returned nil")
		}

		t.Logf("Bundle info: %d agent files, %d MCP configs", len(info.AgentFiles), len(info.MCPConfigs))
	})

	t.Run("Get info from non-existent path", func(t *testing.T) {
		_, err := service.GetBundleInfo("/nonexistent/path")
		if err == nil {
			t.Error("GetBundleInfo() should fail for non-existent path")
		}
	})
}

// TestBundleManifest tests manifest structure
func TestBundleManifest(t *testing.T) {
	manifest := &BundleManifest{
		Version: "1.0",
		Bundle: BundleMetadata{
			Name:        "test-bundle",
			Description: "Test bundle",
			Tags:        []string{"test"},
		},
		Agents: []AgentManifestInfo{
			{
				Name:     "agent1",
				Model:    "gpt-4o-mini",
				MaxSteps: 5,
			},
		},
		MCPServers: []MCPServerManifestInfo{
			{
				Name:    "filesystem",
				Command: "npx",
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Errorf("Failed to marshal manifest: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled BundleManifest
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal manifest: %v", err)
	}

	if unmarshaled.Version != manifest.Version {
		t.Errorf("Version = %s, want %s", unmarshaled.Version, manifest.Version)
	}

	if len(unmarshaled.Agents) != len(manifest.Agents) {
		t.Errorf("Agents count mismatch after unmarshal")
	}
}

// TestExtractManifestFromTarGz tests manifest extraction
func TestExtractManifestFromTarGz(t *testing.T) {
	service := NewBundleService()

	t.Run("Extract manifest from valid bundle", func(t *testing.T) {
		// Create valid bundle with manifest
		sourceDir := t.TempDir()
		os.MkdirAll(filepath.Join(sourceDir, "agents"), 0755)

		bundle, err := service.CreateBundle(sourceDir)
		if err != nil {
			t.Fatalf("Failed to create bundle: %v", err)
		}

		// Extract manifest
		manifest, err := service.ExtractManifestFromTarGz(bundle)
		if err != nil {
			t.Errorf("ExtractManifestFromTarGz() error = %v", err)
			return
		}

		if manifest == nil {
			t.Error("ExtractManifestFromTarGz() returned nil manifest")
		}
	})

	t.Run("Extract from invalid bundle", func(t *testing.T) {
		invalidBundle := []byte("not valid data")

		_, err := service.ExtractManifestFromTarGz(invalidBundle)
		if err == nil {
			t.Error("ExtractManifestFromTarGz() should fail for invalid data")
		}
	})
}

// TestReadTarGz tests tar.gz reading
func TestReadTarGz(t *testing.T) {
	// Create a simple tar.gz for testing
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add a test file
	header := &tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len("test content")),
	}
	tw.WriteHeader(header)
	tw.Write([]byte("test content"))

	tw.Close()
	gw.Close()

	// Read the tar.gz
	gr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	foundFile := false

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Error reading tar: %v", err)
		}

		if header.Name == "test.txt" {
			foundFile = true
			content := make([]byte, header.Size)
			_, err := tr.Read(content)
			if err != nil && err != io.EOF {
				t.Fatalf("Error reading file content: %v", err)
			}
			if string(content) != "test content" {
				t.Errorf("Content = %s, want 'test content'", string(content))
			}
		}
	}

	if !foundFile {
		t.Error("Test file not found in tar.gz")
	}
}

// Benchmark tests
func BenchmarkCreateBundle(b *testing.B) {
	service := NewBundleService()
	tmpDir := b.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	os.MkdirAll(agentsDir, 0755)

	// Create test agent
	agentContent := `---
metadata:
  name: "bench-agent"
model: gpt-4o-mini
max_steps: 5
tools: []
---
Content`
	os.WriteFile(filepath.Join(agentsDir, "agent.prompt"), []byte(agentContent), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.CreateBundle(tmpDir)
	}
}

func BenchmarkParseAgentFile(b *testing.B) {
	service := NewBundleService()
	tmpFile := filepath.Join(b.TempDir(), "agent.prompt")

	agentContent := `---
metadata:
  name: "bench-agent"
  description: "Benchmark agent"
  tags: ["bench"]
model: gpt-4o-mini
max_steps: 5
tools: ["__tool1", "__tool2"]
---
Content`
	os.WriteFile(tmpFile, []byte(agentContent), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.parseAgentFile(tmpFile)
	}
}
