package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
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