package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// BundleService handles environment bundling using the same logic as the API
type BundleService struct{}

// NewBundleService creates a new bundle service
func NewBundleService() *BundleService {
	return &BundleService{}
}

// CreateBundle creates a tar.gz bundle from an environment directory
// This uses the same logic as the API /bundles endpoint for compatibility
func (s *BundleService) CreateBundle(environmentPath string) ([]byte, error) {
	return s.createTarGz(environmentPath)
}

// createTarGz creates a tar.gz archive from the environment directory, excluding variables.yml
// This is identical to internal/api/v1/bundles.go:createTarGz for compatibility
func (s *BundleService) createTarGz(sourceDir string) ([]byte, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	err := filepath.Walk(sourceDir, func(file string, info os.FileInfo, err error) error {
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
// This replicates the same logic as the API handler but works without server dependency
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

	// Create environment directory (filesystem-based, no database)
	envDir := filepath.Join(homeDir, ".config", "station", "environments", environmentName)
	if _, err := os.Stat(envDir); !os.IsNotExist(err) {
		return &BundleInstallResult{
			Success: false,
			Error:   fmt.Sprintf("Environment '%s' already exists", environmentName),
		}, fmt.Errorf("environment already exists: %s", environmentName)
	}

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
		Message:         fmt.Sprintf("Bundle installed successfully to environment '%s'", environmentName),
		EnvironmentName: environmentName,
		BundlePath:      bundlePath,
		InstalledAgents: agentCount,
		InstalledMCPs:   mcpCount,
	}, nil
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	destPath := filepath.Join(bundlesDir, filename)
	outFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer outFile.Close()

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
	defer srcFile.Close()

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

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
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzipReader.Close()

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