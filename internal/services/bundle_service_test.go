package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	defer func() { _ = testDB.Close() }()

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
		name             string
		setupFunc        func(string)
		expectAgents     int
		expectMCPServers int
		expectTags       int
		expectError      bool
		description      string
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
		name              string
		agentContent      string
		expectError       bool
		expectedName      string
		expectedModel     string
		expectedMaxSteps  int
		expectedToolCount int
		description       string
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
			expectError:       false,
			expectedName:      "valid-agent",
			expectedModel:     "gpt-4o-mini",
			expectedMaxSteps:  5,
			expectedToolCount: 2,
			description:       "Should parse valid agent file",
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
			expectError:       false,
			expectedName:      "no-tools-agent",
			expectedModel:     "gpt-4o-mini",
			expectedMaxSteps:  3,
			expectedToolCount: 0,
			description:       "Should handle agent without tools",
		},
		{
			name: "Invalid format - no frontmatter",
			agentContent: `Just plain text content
without any YAML frontmatter
`,
			expectError: true,
			description: "Should fail on invalid format",
		},
		{
			name: "Invalid YAML",
			agentContent: `---
invalid: yaml: content: here
	tabs and bad indentation
---
Content
`,
			expectError: true,
			description: "Should fail on invalid YAML",
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
		name              string
		configContent     string
		expectError       bool
		expectedServers   int
		expectedVariables int
		description       string
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
	_ = tw.WriteHeader(header)
	_, _ = tw.Write([]byte("test content"))

	_ = tw.Close()
	_ = gw.Close()

	// Read the tar.gz
	gr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer func() { _ = gr.Close() }()

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

// TestGenerateBundleNameFromURL tests bundle name generation from URLs
func TestGenerateBundleNameFromURL(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name        string
		url         string
		expected    string
		description string
	}{
		{
			name:        "GitHub repo URL",
			url:         "https://github.com/owner/repo/releases/latest/bundle.tar.gz",
			expected:    "bundle.tar.gz",
			description: "Should extract filename from GitHub release URL",
		},
		{
			name:        "GitHub download URL with 'download' in path",
			url:         "https://github.com/owner/repo/releases/download/v1.0/security-bundle.tar.gz",
			expected:    "security-bundle.tar.gz",
			description: "Should skip 'download' and get filename",
		},
		{
			name:        "URL with 'latest' in path",
			url:         "https://registry.station.dev/bundles/latest/finops-bundle.tar.gz",
			expected:    "finops-bundle.tar.gz",
			description: "Should skip 'latest' and get filename",
		},
		{
			name:        "Simple filename URL",
			url:         "https://example.com/my-bundle.tar.gz",
			expected:    "my-bundle.tar.gz",
			description: "Should extract simple filename",
		},
		{
			name:        "URL with spaces in filename",
			url:         "https://example.com/My Bundle Name.tar.gz",
			expected:    "my-bundle-name.tar.gz",
			description: "Should replace spaces with dashes and lowercase",
		},
		{
			name:        "URL with mixed case",
			url:         "https://example.com/MyBundle.TAR.GZ",
			expected:    "mybundle.tar.gz",
			description: "Should convert to lowercase",
		},
		{
			name:        "URL ending with slash",
			url:         "https://example.com/bundles/",
			expected:    "bundles",
			description: "Should handle trailing slash",
		},
		{
			name:        "URL with only empty parts",
			url:         "https://example.com///",
			expected:    "example.com",
			description: "Should extract domain when only slashes after it",
		},
		{
			name:        "Complex GitHub URL",
			url:         "https://github.com/cloudshipai/station-bundles/releases/download/v2.1.0/terraform-security.tar.gz",
			expected:    "terraform-security.tar.gz",
			description: "Should extract from complex GitHub URL",
		},
		{
			name:        "URL with query parameters",
			url:         "https://example.com/bundle.tar.gz?token=abc123",
			expected:    "bundle.tar.gz?token=abc123",
			description: "Should include query parameters in filename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.generateBundleNameFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("%s: got %q, want %q", tt.description, result, tt.expected)
			}
		})
	}
}

// TestGenerateBundleNameFromURLEdgeCases tests edge cases
func TestGenerateBundleNameFromURLEdgeCases(t *testing.T) {
	service := NewBundleService()

	t.Run("Empty URL", func(t *testing.T) {
		result := service.generateBundleNameFromURL("")
		if result != "bundle" {
			t.Errorf("Should return default for empty URL, got %q", result)
		}
	})

	t.Run("Single slash", func(t *testing.T) {
		result := service.generateBundleNameFromURL("/")
		if result != "bundle" {
			t.Errorf("Should return default for single slash, got %q", result)
		}
	})

	t.Run("URL with 'download' and 'latest' both present", func(t *testing.T) {
		result := service.generateBundleNameFromURL("https://example.com/download/latest/mybundle.tar.gz")
		if result != "mybundle.tar.gz" {
			t.Errorf("Should skip both 'download' and 'latest', got %q", result)
		}
	})

	t.Run("URL with multiple spaces", func(t *testing.T) {
		result := service.generateBundleNameFromURL("https://example.com/My   Bundle   Name.tar.gz")
		if result != "my---bundle---name.tar.gz" {
			t.Errorf("Should replace each space with dash, got %q", result)
		}
	})

	t.Run("Filename is 'download'", func(t *testing.T) {
		result := service.generateBundleNameFromURL("https://example.com/path/download")
		if result != "path" {
			t.Errorf("Should skip 'download' and use previous part, got %q", result)
		}
	})

	t.Run("Filename is 'latest'", func(t *testing.T) {
		result := service.generateBundleNameFromURL("https://example.com/bundles/latest")
		if result != "bundles" {
			t.Errorf("Should skip 'latest' and use previous part, got %q", result)
		}
	})

	t.Run("All parts are 'download' or 'latest'", func(t *testing.T) {
		result := service.generateBundleNameFromURL("download/latest/download/latest")
		if result != "bundle" {
			t.Errorf("Should return default when all parts filtered, got %q", result)
		}
	})
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

// TestDownloadBundle tests HTTP bundle downloading
func TestDownloadBundle(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name        string
		setupServer func() (string, func())
		expectError bool
		description string
	}{
		{
			name: "Download valid tar.gz bundle",
			setupServer: func() (string, func()) {
				mux := http.NewServeMux()
				mux.HandleFunc("/test-bundle.tar.gz", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/gzip")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("fake-tar-gz-content"))
				})
				server := httptest.NewServer(mux)
				return server.URL + "/test-bundle.tar.gz", server.Close
			},
			expectError: false,
			description: "Should download bundle from HTTP server",
		},
		{
			name: "Download fails with 404",
			setupServer: func() (string, func()) {
				mux := http.NewServeMux()
				mux.HandleFunc("/missing.tar.gz", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				})
				server := httptest.NewServer(mux)
				return server.URL + "/missing.tar.gz", server.Close
			},
			expectError: true,
			description: "Should fail with 404 status",
		},
		{
			name: "Download fails with 500",
			setupServer: func() (string, func()) {
				mux := http.NewServeMux()
				mux.HandleFunc("/error.tar.gz", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				})
				server := httptest.NewServer(mux)
				return server.URL + "/error.tar.gz", server.Close
			},
			expectError: true,
			description: "Should fail with 500 error",
		},
		{
			name: "Generate filename from URL without .tar.gz",
			setupServer: func() (string, func()) {
				mux := http.NewServeMux()
				mux.HandleFunc("/bundles/latest", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("bundle-content"))
				})
				server := httptest.NewServer(mux)
				return server.URL + "/bundles/latest", server.Close
			},
			expectError: false,
			description: "Should generate filename when URL doesn't end with .tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, cleanup := tt.setupServer()
			defer cleanup()

			bundlesDir := t.TempDir()
			destPath, err := service.downloadBundle(url, bundlesDir)

			if (err != nil) != tt.expectError {
				t.Errorf("downloadBundle() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if destPath == "" {
					t.Error("downloadBundle() returned empty path")
				}

				// Verify file was created
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					t.Errorf("Downloaded file does not exist: %s", destPath)
				}

				// Verify file is in bundles directory
				if !filepath.IsAbs(destPath) || !strings.HasPrefix(destPath, bundlesDir) {
					t.Errorf("File path %s not in bundles directory %s", destPath, bundlesDir)
				}
			}
		})
	}
}

// TestCopyBundle tests bundle copying from filesystem
func TestCopyBundle(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name        string
		setupSource func(string) string
		expectError bool
		description string
	}{
		{
			name: "Copy valid bundle file",
			setupSource: func(dir string) string {
				srcPath := filepath.Join(dir, "source-bundle.tar.gz")
				os.WriteFile(srcPath, []byte("test bundle content"), 0644)
				return srcPath
			},
			expectError: false,
			description: "Should copy bundle from filesystem",
		},
		{
			name: "Copy non-existent file",
			setupSource: func(dir string) string {
				return filepath.Join(dir, "nonexistent.tar.gz")
			},
			expectError: true,
			description: "Should fail for non-existent source file",
		},
		{
			name: "Copy file with spaces in name",
			setupSource: func(dir string) string {
				srcPath := filepath.Join(dir, "my bundle file.tar.gz")
				os.WriteFile(srcPath, []byte("bundle with spaces"), 0644)
				return srcPath
			},
			expectError: false,
			description: "Should handle filenames with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			bundlesDir := t.TempDir()
			srcPath := tt.setupSource(sourceDir)

			destPath, err := service.copyBundle(srcPath, bundlesDir)

			if (err != nil) != tt.expectError {
				t.Errorf("copyBundle() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if destPath == "" {
					t.Error("copyBundle() returned empty path")
				}

				// Verify file was copied
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					t.Errorf("Copied file does not exist: %s", destPath)
				}

				// Verify content matches
				srcContent, _ := os.ReadFile(srcPath)
				destContent, _ := os.ReadFile(destPath)
				if !bytes.Equal(srcContent, destContent) {
					t.Error("Copied file content does not match source")
				}

				// Verify file is in bundles directory
				if !strings.HasPrefix(destPath, bundlesDir) {
					t.Errorf("File path %s not in bundles directory %s", destPath, bundlesDir)
				}
			}
		})
	}
}

// TestExtractBundle tests tar.gz bundle extraction
func TestExtractBundle(t *testing.T) {
	service := NewBundleService()

	tests := []struct {
		name              string
		createBundle      func(string) string
		expectError       bool
		expectedAgents    int
		expectedMCPs      int
		validateExtracted func(*testing.T, string)
		description       string
	}{
		{
			name: "Extract bundle with agents and MCP config",
			createBundle: func(dir string) string {
				bundlePath := filepath.Join(dir, "test-bundle.tar.gz")

				// Create tar.gz with agents and template.json
				var buf bytes.Buffer
				gw := gzip.NewWriter(&buf)
				tw := tar.NewWriter(gw)

				// Add agent file
				agentContent := []byte(`---
metadata:
  name: "test-agent"
model: gpt-4o-mini
max_steps: 3
tools: []
---
Content`)
				agentHeader := &tar.Header{
					Name: "agents/test-agent.prompt",
					Mode: 0644,
					Size: int64(len(agentContent)),
				}
				tw.WriteHeader(agentHeader)
				tw.Write(agentContent)

				// Add MCP config
				mcpContent := []byte(`{"mcpServers": {}}`)
				mcpHeader := &tar.Header{
					Name: "template.json",
					Mode: 0644,
					Size: int64(len(mcpContent)),
				}
				tw.WriteHeader(mcpHeader)
				tw.Write(mcpContent)

				tw.Close()
				gw.Close()

				os.WriteFile(bundlePath, buf.Bytes(), 0644)
				return bundlePath
			},
			expectError:    false,
			expectedAgents: 1,
			expectedMCPs:   1,
			validateExtracted: func(t *testing.T, envDir string) {
				// Verify agent file exists
				agentPath := filepath.Join(envDir, "agents", "test-agent.prompt")
				if _, err := os.Stat(agentPath); os.IsNotExist(err) {
					t.Error("Agent file not extracted")
				}

				// Verify MCP config exists
				mcpPath := filepath.Join(envDir, "template.json")
				if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
					t.Error("MCP config not extracted")
				}
			},
			description: "Should extract agents and MCP configs",
		},
		{
			name: "Extract bundle with multiple agents",
			createBundle: func(dir string) string {
				bundlePath := filepath.Join(dir, "multi-agent.tar.gz")

				var buf bytes.Buffer
				gw := gzip.NewWriter(&buf)
				tw := tar.NewWriter(gw)

				// Add multiple agent files
				for i := 1; i <= 3; i++ {
					content := []byte("agent content")
					header := &tar.Header{
						Name: fmt.Sprintf("agents/agent%d.prompt", i),
						Mode: 0644,
						Size: int64(len(content)),
					}
					tw.WriteHeader(header)
					tw.Write(content)
				}

				tw.Close()
				gw.Close()

				os.WriteFile(bundlePath, buf.Bytes(), 0644)
				return bundlePath
			},
			expectError:    false,
			expectedAgents: 3,
			expectedMCPs:   0,
			description:    "Should count multiple agent files",
		},
		{
			name: "Extract bundle with directory entries",
			createBundle: func(dir string) string {
				bundlePath := filepath.Join(dir, "with-dirs.tar.gz")

				var buf bytes.Buffer
				gw := gzip.NewWriter(&buf)
				tw := tar.NewWriter(gw)

				// Add directory entry
				dirHeader := &tar.Header{
					Name:     "agents/",
					Mode:     0755,
					Typeflag: tar.TypeDir,
				}
				tw.WriteHeader(dirHeader)

				// Add file in directory
				content := []byte("content")
				fileHeader := &tar.Header{
					Name: "agents/test.prompt",
					Mode: 0644,
					Size: int64(len(content)),
				}
				tw.WriteHeader(fileHeader)
				tw.Write(content)

				tw.Close()
				gw.Close()

				os.WriteFile(bundlePath, buf.Bytes(), 0644)
				return bundlePath
			},
			expectError:    false,
			expectedAgents: 1,
			expectedMCPs:   0,
			description:    "Should handle directory entries in tar",
		},
		{
			name: "Extract invalid gzip file",
			createBundle: func(dir string) string {
				bundlePath := filepath.Join(dir, "invalid.tar.gz")
				os.WriteFile(bundlePath, []byte("not a valid gzip file"), 0644)
				return bundlePath
			},
			expectError: true,
			description: "Should fail on invalid gzip",
		},
		{
			name: "Extract empty bundle",
			createBundle: func(dir string) string {
				bundlePath := filepath.Join(dir, "empty.tar.gz")

				var buf bytes.Buffer
				gw := gzip.NewWriter(&buf)
				tw := tar.NewWriter(gw)
				tw.Close()
				gw.Close()

				os.WriteFile(bundlePath, buf.Bytes(), 0644)
				return bundlePath
			},
			expectError:    false,
			expectedAgents: 0,
			expectedMCPs:   0,
			description:    "Should handle empty bundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			bundlePath := tt.createBundle(tmpDir)
			envDir := filepath.Join(tmpDir, "test-env")

			agentCount, mcpCount, err := service.extractBundle(bundlePath, envDir)

			if (err != nil) != tt.expectError {
				t.Errorf("extractBundle() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if agentCount != tt.expectedAgents {
					t.Errorf("Agent count = %d, want %d", agentCount, tt.expectedAgents)
				}

				if mcpCount != tt.expectedMCPs {
					t.Errorf("MCP count = %d, want %d", mcpCount, tt.expectedMCPs)
				}

				// Verify environment directory was created
				if _, err := os.Stat(envDir); os.IsNotExist(err) {
					t.Error("Environment directory not created")
				}

				// Run custom validation if provided
				if tt.validateExtracted != nil {
					tt.validateExtracted(t, envDir)
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{
			name:  "Item exists in slice",
			slice: []string{"apple", "banana", "cherry"},
			item:  "banana",
			want:  true,
		},
		{
			name:  "Item does not exist in slice",
			slice: []string{"apple", "banana", "cherry"},
			item:  "grape",
			want:  false,
		},
		{
			name:  "Empty slice",
			slice: []string{},
			item:  "apple",
			want:  false,
		},
		{
			name:  "Item is empty string and exists",
			slice: []string{"apple", "", "cherry"},
			item:  "",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.slice, tt.item); got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}
