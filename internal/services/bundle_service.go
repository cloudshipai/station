package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"station/internal/db/repositories"
	"station/internal/version"
)

// BundleService handles environment bundling using the same logic as the API
type BundleService struct {
	repos *repositories.Repositories
}

// NewBundleService creates a new bundle service
func NewBundleService() *BundleService {
	return &BundleService{
		repos: nil, // For backwards compatibility - will use filesystem-only mode
	}
}

// NewBundleServiceWithRepos creates a bundle service with database support
func NewBundleServiceWithRepos(repos *repositories.Repositories) *BundleService {
	return &BundleService{
		repos: repos,
	}
}

// CreateBundle creates a tar.gz bundle from an environment directory
// This uses the same logic as the API /bundles endpoint for compatibility
func (s *BundleService) CreateBundle(environmentPath string) ([]byte, error) {
	return s.createTarGz(environmentPath)
}

// generateManifest creates a bundle manifest by analyzing the environment directory
func (s *BundleService) generateManifest(sourceDir string) (*BundleManifest, error) {
	// Extract environment name from path
	envName := filepath.Base(sourceDir)

	manifest := &BundleManifest{
		Version: "1.0",
		Bundle: BundleMetadata{
			Name:           envName,
			Description:    fmt.Sprintf("Station bundle for %s environment", envName),
			Tags:           []string{},
			CreatedAt:      time.Now(),
			StationVersion: version.Version,
		},
		Agents:                []AgentManifestInfo{},
		MCPServers:            []MCPServerManifestInfo{},
		Tools:                 []ToolManifestInfo{},
		AgentMCPRelationships: []AgentMCPRelationship{},
		RequiredVariables:     []VariableRequirement{},
	}

	// Parse agent files
	agentsDir := filepath.Join(sourceDir, "agents")
	if _, err := os.Stat(agentsDir); err == nil {
		agents, err := os.ReadDir(agentsDir)
		if err == nil {
			for _, agent := range agents {
				if strings.HasSuffix(agent.Name(), ".prompt") {
					agentInfo, err := s.parseAgentFile(filepath.Join(agentsDir, agent.Name()))
					if err == nil {
						manifest.Agents = append(manifest.Agents, *agentInfo)

						// Add agent-MCP relationship
						if len(agentInfo.MCPServers) > 0 {
							manifest.AgentMCPRelationships = append(manifest.AgentMCPRelationships, AgentMCPRelationship{
								Agent:      agentInfo.Name,
								MCPServers: agentInfo.MCPServers,
							})
						}

						// Collect tags from agents
						for _, tag := range agentInfo.Tags {
							if !contains(manifest.Bundle.Tags, tag) {
								manifest.Bundle.Tags = append(manifest.Bundle.Tags, tag)
							}
						}
					}
				}
			}
		}
	}

	// Parse MCP server configs
	configs, err := os.ReadDir(sourceDir)
	if err == nil {
		for _, config := range configs {
			if strings.HasSuffix(config.Name(), ".json") && config.Name() != "manifest.json" {
				mcpServers, variables, err := s.parseMCPConfigFile(filepath.Join(sourceDir, config.Name()))
				if err == nil {
					manifest.MCPServers = append(manifest.MCPServers, mcpServers...)
					manifest.RequiredVariables = append(manifest.RequiredVariables, variables...)

					// Build tools list from MCP servers
					for _, server := range mcpServers {
						for _, tool := range server.Tools {
							manifest.Tools = append(manifest.Tools, ToolManifestInfo{
								Name:        tool,
								Server:      server.Name,
								Description: fmt.Sprintf("Tool from %s server", server.Name),
							})
						}
					}
				}
			}
		}
	}

	return manifest, nil
}

// parseAgentFile extracts metadata from a .prompt file
func (s *BundleService) parseAgentFile(filePath string) (*AgentManifestInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Split frontmatter and content
	parts := strings.Split(string(data), "---")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid agent file format")
	}

	// Parse YAML frontmatter
	var frontmatter struct {
		Metadata struct {
			Name        string   `yaml:"name"`
			Description string   `yaml:"description"`
			Tags        []string `yaml:"tags"`
			App         string   `yaml:"app"`
			AppType     string   `yaml:"app_type"`
		} `yaml:"metadata"`
		Model    string   `yaml:"model"`
		MaxSteps int      `yaml:"max_steps"`
		Tools    []string `yaml:"tools"`
	}

	if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err != nil {
		return nil, err
	}

	// Infer MCP servers from tool names
	mcpServers := []string{}
	for _, tool := range frontmatter.Tools {
		// Extract server name from tool (e.g., "__get_cost_and_usage" -> infer server)
		// This is a heuristic - we'll track actual server names from MCP configs
		if strings.HasPrefix(tool, "__") {
			// This is an MCP tool, we'll match it later with servers
		}
	}

	return &AgentManifestInfo{
		Name:        frontmatter.Metadata.Name,
		Description: frontmatter.Metadata.Description,
		Model:       frontmatter.Model,
		MaxSteps:    frontmatter.MaxSteps,
		Tags:        frontmatter.Metadata.Tags,
		Tools:       frontmatter.Tools,
		MCPServers:  mcpServers, // Will be populated when matching tools to servers
		App:         frontmatter.Metadata.App,
		AppType:     frontmatter.Metadata.AppType,
	}, nil
}

// parseMCPConfigFile extracts MCP servers and variables from template.json
func (s *BundleService) parseMCPConfigFile(filePath string) ([]MCPServerManifestInfo, []VariableRequirement, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	var config struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		MCPServers  map[string]struct {
			Command string            `json:"command"`
			Args    []interface{}     `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, nil, err
	}

	servers := []MCPServerManifestInfo{}
	variableSet := make(map[string]bool)

	for serverName, serverConfig := range config.MCPServers {
		// Convert args to strings
		args := []string{}
		for _, arg := range serverConfig.Args {
			args = append(args, fmt.Sprintf("%v", arg))
		}

		servers = append(servers, MCPServerManifestInfo{
			Name:        serverName,
			Command:     serverConfig.Command,
			Args:        args,
			Env:         serverConfig.Env,
			Tools:       []string{}, // Will be populated during actual sync
			Description: fmt.Sprintf("MCP server: %s", serverName),
		})

		// Extract variables from args and env
		for _, arg := range args {
			extractVariables(arg, variableSet)
		}
		for _, envVal := range serverConfig.Env {
			extractVariables(envVal, variableSet)
		}
	}

	// Convert variable set to requirements
	variables := []VariableRequirement{}
	for varName := range variableSet {
		variables = append(variables, VariableRequirement{
			Name:        varName,
			Description: fmt.Sprintf("Required variable: %s", varName),
			Type:        "string",
			Required:    true,
		})
	}

	return servers, variables, nil
}

// extractVariables finds Go template variables like {{ .VAR_NAME }}
func extractVariables(text string, varSet map[string]bool) {
	// Simple regex-like extraction
	start := 0
	for {
		idx := strings.Index(text[start:], "{{")
		if idx == -1 {
			break
		}
		idx += start
		end := strings.Index(text[idx:], "}}")
		if end == -1 {
			break
		}
		end += idx

		// Extract variable name
		varPart := strings.TrimSpace(text[idx+2 : end])
		if strings.HasPrefix(varPart, ".") {
			varName := strings.TrimPrefix(varPart, ".")
			varName = strings.TrimSpace(varName)
			varSet[varName] = true
		}

		start = end + 2
	}
}

// contains checks if a string slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// createTarGz creates a tar.gz archive from the environment directory, excluding variables.yml
// Generates and includes manifest.json with complete bundle metadata
func (s *BundleService) createTarGz(sourceDir string) ([]byte, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Generate manifest
	manifest, err := s.generateManifest(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to generate manifest: %w", err)
	}

	// Add manifest.json to the archive first
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize manifest: %w", err)
	}

	manifestHeader := &tar.Header{
		Name: "manifest.json",
		Mode: 0644,
		Size: int64(len(manifestJSON)),
	}
	if err := tarWriter.WriteHeader(manifestHeader); err != nil {
		return nil, fmt.Errorf("failed to write manifest header: %w", err)
	}
	if _, err := tarWriter.Write(manifestJSON); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	// Add all other files
	err = filepath.Walk(sourceDir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip variables.yml files
		if strings.HasSuffix(file, "variables.yml") || strings.HasSuffix(file, "variables.yaml") {
			return nil
		}

		// Skip the source directory itself
		if file == sourceDir {
			return nil
		}

		// Create relative path
		relPath, err := filepath.Rel(sourceDir, file)
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file content if it's a regular file
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			if _, err := tarWriter.Write(data); err != nil {
				return err
			}
		}

		return nil
	})

	// Close writers in the correct order
	tarWriter.Close()
	gzWriter.Close()

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ValidateEnvironment checks if an environment directory exists and is valid for bundling
func (s *BundleService) ValidateEnvironment(environmentPath string) error {
	// Check if directory exists
	info, err := os.Stat(environmentPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("environment directory does not exist: %s", environmentPath)
	}
	if err != nil {
		return fmt.Errorf("failed to access environment directory: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("environment path is not a directory: %s", environmentPath)
	}

	// Check if it contains typical environment files
	hasAgents := false
	hasMCPConfigs := false

	entries, err := os.ReadDir(environmentPath)
	if err != nil {
		return fmt.Errorf("failed to read environment directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == "agents" {
			hasAgents = true
		}
		if strings.HasSuffix(entry.Name(), ".json") {
			hasMCPConfigs = true
		}
	}

	if !hasAgents && !hasMCPConfigs {
		return fmt.Errorf("environment directory does not contain agents or MCP configs: %s", environmentPath)
	}

	return nil
}

// GetBundleInfo returns information about what would be bundled
func (s *BundleService) GetBundleInfo(environmentPath string) (*BundleInfo, error) {
	if err := s.ValidateEnvironment(environmentPath); err != nil {
		return nil, err
	}

	info := &BundleInfo{
		EnvironmentPath: environmentPath,
		AgentFiles:      []string{},
		MCPConfigs:      []string{},
		OtherFiles:      []string{},
	}

	err := filepath.Walk(environmentPath, func(file string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip variables.yml files
		if strings.HasSuffix(file, "variables.yml") || strings.HasSuffix(file, "variables.yaml") {
			return nil
		}

		// Skip the source directory itself
		if file == environmentPath {
			return nil
		}

		// Skip directories, only count files
		if fileInfo.IsDir() {
			return nil
		}

		// Create relative path
		relPath, err := filepath.Rel(environmentPath, file)
		if err != nil {
			return err
		}

		// Categorize files
		if strings.HasPrefix(relPath, "agents/") && strings.HasSuffix(relPath, ".prompt") {
			info.AgentFiles = append(info.AgentFiles, relPath)
		} else if strings.HasSuffix(relPath, ".json") && !strings.Contains(relPath, "/") {
			info.MCPConfigs = append(info.MCPConfigs, relPath)
		} else {
			info.OtherFiles = append(info.OtherFiles, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze environment: %w", err)
	}

	return info, nil
}

// BundleInfo contains information about a bundle
type BundleInfo struct {
	EnvironmentPath string   `json:"environment_path"`
	AgentFiles      []string `json:"agent_files"`
	MCPConfigs      []string `json:"mcp_configs"`
	OtherFiles      []string `json:"other_files"`
}

// BundleManifest represents the complete metadata for a bundle
type BundleManifest struct {
	Version               string                  `json:"version"`
	Bundle                BundleMetadata          `json:"bundle"`
	Agents                []AgentManifestInfo     `json:"agents"`
	MCPServers            []MCPServerManifestInfo `json:"mcp_servers"`
	Tools                 []ToolManifestInfo      `json:"tools"`
	AgentMCPRelationships []AgentMCPRelationship  `json:"agent_mcp_relationships"`
	RequiredVariables     []VariableRequirement   `json:"required_variables"`
}

// BundleMetadata contains high-level bundle information
type BundleMetadata struct {
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Tags           []string  `json:"tags"`
	CreatedAt      time.Time `json:"created_at"`
	StationVersion string    `json:"station_version"`
}

// AgentManifestInfo contains agent metadata for the manifest
type AgentManifestInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Model       string   `json:"model"`
	MaxSteps    int      `json:"max_steps"`
	Tags        []string `json:"tags"`
	Tools       []string `json:"tools"`
	MCPServers  []string `json:"mcp_servers"`
	App         string   `json:"app,omitempty"`
	AppType     string   `json:"app_type,omitempty"`
}

// MCPServerManifestInfo contains MCP server metadata
type MCPServerManifestInfo struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Tools       []string          `json:"tools"`
	Description string            `json:"description"`
}

// ToolManifestInfo contains tool metadata
type ToolManifestInfo struct {
	Name        string `json:"name"`
	Server      string `json:"server"`
	Description string `json:"description"`
}

// AgentMCPRelationship maps agents to their MCP servers
type AgentMCPRelationship struct {
	Agent      string   `json:"agent"`
	MCPServers []string `json:"mcp_servers"`
}

// VariableRequirement describes a required template variable
type VariableRequirement struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// BundleInstallResult contains the result of bundle installation
type BundleInstallResult struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	EnvironmentName string `json:"environment_name"`
	BundlePath      string `json:"bundle_path"`
	InstalledAgents int    `json:"installed_agents"`
	InstalledMCPs   int    `json:"installed_mcps"`
	Error           string `json:"error,omitempty"`
}

// InstallBundle installs a bundle from URL or file path to create a new environment
// Uses EnvironmentManagementService to properly create both database and filesystem
func (s *BundleService) InstallBundle(bundleLocation, environmentName string) (*BundleInstallResult, error) {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &BundleInstallResult{
			Success: false,
			Error:   "Failed to get home directory",
		}, err
	}

	// Create bundles directory if it doesn't exist
	bundlesDir := filepath.Join(homeDir, ".config", "station", "bundles")
	if err := os.MkdirAll(bundlesDir, 0755); err != nil {
		return &BundleInstallResult{
			Success: false,
			Error:   "Failed to create bundles directory",
		}, err
	}

	// Determine source type and download/copy bundle
	var bundlePath string
	if strings.HasPrefix(bundleLocation, "http") {
		// Download from URL
		bundlePath, err = s.downloadBundle(bundleLocation, bundlesDir)
		if err != nil {
			return &BundleInstallResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to download bundle: %v", err),
			}, err
		}
	} else {
		// Copy from file path
		bundlePath, err = s.copyBundle(bundleLocation, bundlesDir)
		if err != nil {
			return &BundleInstallResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to copy bundle: %v", err),
			}, err
		}
	}

	// Environment directory path
	envDir := filepath.Join(homeDir, ".config", "station", "environments", environmentName)

	// Check if environment already exists (filesystem check)
	if _, err := os.Stat(envDir); !os.IsNotExist(err) {
		return &BundleInstallResult{
			Success: false,
			Error:   fmt.Sprintf("Environment '%s' already exists", environmentName),
		}, fmt.Errorf("environment already exists: %s", environmentName)
	}

	// Create environment using proper service layer if repos are available
	if s.repos != nil {
		// Use EnvironmentManagementService for full database + filesystem creation
		envService := NewEnvironmentManagementService(s.repos)
		description := fmt.Sprintf("Environment created from bundle installation")

		environment, result, err := envService.CreateEnvironment(environmentName, &description, 1)
		if err != nil || !result.Success {
			errorMsg := result.Message
			if errorMsg == "" && err != nil {
				errorMsg = err.Error()
			}
			return &BundleInstallResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to create environment: %s", errorMsg),
			}, err
		}

		// Extract bundle to environment directory
		agentCount, mcpCount, err := s.extractBundle(bundlePath, envDir)
		if err != nil {
			// Clean up environment if extraction failed
			envService.DeleteEnvironmentByID(environment.ID)
			return &BundleInstallResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to extract bundle: %v", err),
			}, err
		}

		return &BundleInstallResult{
			Success:         true,
			Message:         fmt.Sprintf("Bundle installed successfully to environment '%s'", environmentName),
			EnvironmentName: environmentName,
			BundlePath:      bundlePath,
			InstalledAgents: agentCount,
			InstalledMCPs:   mcpCount,
		}, nil
	} else {
		// Fallback to filesystem-only mode (no database)
		// This is for backwards compatibility when service is used without repos

		// Extract bundle to environment directory
		agentCount, mcpCount, err := s.extractBundle(bundlePath, envDir)
		if err != nil {
			// Clean up environment directory on failure
			os.RemoveAll(envDir)
			return &BundleInstallResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to extract bundle: %v", err),
			}, err
		}

		return &BundleInstallResult{
			Success:         true,
			Message:         fmt.Sprintf("Bundle installed successfully to environment '%s' (filesystem only - run 'stn sync' to register in database)", environmentName),
			EnvironmentName: environmentName,
			BundlePath:      bundlePath,
			InstalledAgents: agentCount,
			InstalledMCPs:   mcpCount,
		}, nil
	}
}

// downloadBundle downloads a bundle from a URL to the bundles directory
func (s *BundleService) downloadBundle(url, bundlesDir string) (string, error) {
	// Extract filename from URL
	parts := strings.Split(url, "/")
	filename := parts[len(parts)-1]
	if !strings.HasSuffix(filename, ".tar.gz") {
		// Generate meaningful name from URL path
		bundleName := s.generateBundleNameFromURL(url)
		filename = fmt.Sprintf("%s.tar.gz", bundleName)
	}

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download bundle: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	destPath := filepath.Join(bundlesDir, filename)
	outFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer func() { _ = outFile.Close() }()

	// Copy the response body to file
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	return destPath, nil
}

// copyBundle copies a bundle from a file path to the bundles directory
func (s *BundleService) copyBundle(srcPath, bundlesDir string) (string, error) {
	// Check if source file exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return "", fmt.Errorf("bundle file does not exist: %s", srcPath)
	}

	// Extract filename
	filename := filepath.Base(srcPath)
	destPath := filepath.Join(bundlesDir, filename)

	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %v", err)
	}
	defer func() { _ = srcFile.Close() }()

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %v", err)
	}
	defer func() { _ = destFile.Close() }()

	// Copy the file
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy file: %v", err)
	}

	return destPath, nil
}

// extractBundle extracts a tar.gz bundle to the environment directory
func (s *BundleService) extractBundle(bundlePath, envDir string) (int, int, error) {
	// Create environment directory
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create environment directory: %v", err)
	}

	// Open the bundle file
	file, err := os.Open(bundlePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open bundle: %v", err)
	}
	defer func() { _ = file.Close() }()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer func() { _ = gzipReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	agentCount := 0
	mcpCount := 0

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("failed to read tar entry: %v", err)
		}

		// Create the full file path
		destPath := filepath.Join(envDir, header.Name)

		// Ensure the directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return 0, 0, fmt.Errorf("failed to create directory: %v", err)
		}

		// Extract based on type
		switch header.Typeflag {
		case tar.TypeReg:
			// Regular file
			outFile, err := os.Create(destPath)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to create file %s: %v", destPath, err)
			}

			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return 0, 0, fmt.Errorf("failed to write file %s: %v", destPath, err)
			}

			// Count agents and MCP configs
			if (strings.HasPrefix(header.Name, "agents/") || strings.HasPrefix(header.Name, "./agents/")) && strings.HasSuffix(header.Name, ".prompt") {
				agentCount++
			} else if strings.HasSuffix(header.Name, ".json") {
				mcpCount++
			}

		case tar.TypeDir:
			// Directory
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return 0, 0, fmt.Errorf("failed to create directory %s: %v", destPath, err)
			}
		}
	}

	return agentCount, mcpCount, nil
}

// generateBundleNameFromURL generates a meaningful filename from URL
func (s *BundleService) generateBundleNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if part != "" && part != "download" && part != "latest" {
			// Clean the part to be filename-safe
			cleaned := strings.ReplaceAll(part, " ", "-")
			cleaned = strings.ToLower(cleaned)
			return cleaned
		}
	}
	return "bundle"
}

// ExtractManifestFromTarGz extracts manifest.json from a tar.gz bundle
func (s *BundleService) ExtractManifestFromTarGz(bundleData []byte) (*BundleManifest, error) {
	// Create a reader from the bundle data
	reader := bytes.NewReader(bundleData)

	// Create gzip reader
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Find and extract manifest.json
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Check if this is the manifest file
		if header.Name == "manifest.json" || header.Name == "./manifest.json" {
			// Read the manifest content
			manifestData, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest: %w", err)
			}

			// Parse the manifest
			var manifest BundleManifest
			if err := json.Unmarshal(manifestData, &manifest); err != nil {
				return nil, fmt.Errorf("failed to parse manifest: %w", err)
			}

			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("manifest.json not found in bundle")
}
