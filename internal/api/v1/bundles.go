package v1

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"station/internal/config"
)

// Bundle request structure
type BundleRequest struct {
	Environment string `json:"environment" binding:"required"`
	Local       bool   `json:"local"`
	Endpoint    string `json:"endpoint"`
}

// Bundle response structure
type BundleResponse struct {
	Success       bool                    `json:"success"`
	Message       string                  `json:"message"`
	LocalPath     string                  `json:"local_path,omitempty"`
	ShareID       string                  `json:"share_id,omitempty"`
	ShareURL      string                  `json:"share_url,omitempty"`
	Expires       string                  `json:"expires,omitempty"`
	CloudShipInfo *map[string]interface{} `json:"cloudship_info,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

// Share server response structure
type ShareServerResponse struct {
	ShareID string `json:"share_id"`
	URL     string `json:"url"`
	Expires string `json:"expires"`
}

// createBundle handles the POST /bundles endpoint
func (h *APIHandlers) createBundle(c *gin.Context) {
	var req BundleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, BundleResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, BundleResponse{
			Success: false,
			Error:   "Failed to get home directory",
		})
		return
	}

	// Environment directory path
	envPath := filepath.Join(homeDir, ".config", "station", "environments", req.Environment)
	
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, BundleResponse{
			Success: false,
			Error:   fmt.Sprintf("Environment '%s' not found", req.Environment),
		})
		return
	}

	// Create tar.gz bundle
	tarData, err := createTarGz(envPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, BundleResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create bundle: %v", err),
		})
		return
	}

	if req.Local {
		// Save locally
		bundlesDir := filepath.Join(homeDir, ".config", "station", "bundles")
		if err := os.MkdirAll(bundlesDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, BundleResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to create bundles directory: %v", err),
			})
			return
		}

		localPath := filepath.Join(bundlesDir, fmt.Sprintf("%s.tar.gz", req.Environment))
		if err := os.WriteFile(localPath, tarData, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, BundleResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to save bundle locally: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, BundleResponse{
			Success:   true,
			Message:   "Bundle created successfully",
			LocalPath: localPath,
		})
	} else {
		// Upload to CloudShip
		cfg, err := config.Load()
		if err != nil {
			c.JSON(http.StatusInternalServerError, BundleResponse{
				Success: false,
				Error:   "Failed to load Station config: " + err.Error(),
			})
			return
		}

		// Check if CloudShip is configured
		if !cfg.CloudShip.Enabled || cfg.CloudShip.RegistrationKey == "" {
			c.JSON(http.StatusBadRequest, BundleResponse{
				Success: false,
				Error:   "CloudShip is not configured. Please enable CloudShip and set registration key in config",
			})
			return
		}

		// Use default CloudShip API URL
		apiURL := "https://api.cloudshipai.com"

		// Upload to CloudShip
		cloudShipResp, err := uploadToCloudShip(tarData, fmt.Sprintf("%s.tar.gz", req.Environment), apiURL, cfg.CloudShip.RegistrationKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, BundleResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to upload to CloudShip: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, BundleResponse{
			Success:       true,
			Message:       fmt.Sprintf("Bundle uploaded to CloudShip (Org: %s)", cloudShipResp["organization"]),
			ShareURL:      fmt.Sprintf("%s%s", apiURL, cloudShipResp["download_url"]),
			CloudShipInfo: &cloudShipResp,
		})
	}
}

// createTarGz creates a tar.gz archive from the environment directory, excluding variables.yml
func createTarGz(sourceDir string) ([]byte, error) {
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

// uploadToCloudShip uploads the bundle to CloudShip's authenticated API
func uploadToCloudShip(tarData []byte, filename, apiURL, registrationKey string) (map[string]interface{}, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create the file field
	part, err := writer.CreateFormFile("bundle", filename)
	if err != nil {
		return nil, err
	}

	// Write the tar data
	if _, err := part.Write(tarData); err != nil {
		return nil, err
	}

	// Close the writer to finalize the multipart data
	if err := writer.Close(); err != nil {
		return nil, err
	}

	// Create the request
	uploadURL := fmt.Sprintf("%s/api/public/bundles/upload", strings.TrimSuffix(apiURL, "/"))
	req, err := http.NewRequest("POST", uploadURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Registration-Key", registrationKey)

	// Send the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("CloudShip upload failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var cloudShipResp map[string]interface{}
	if err := json.Unmarshal(respBody, &cloudShipResp); err != nil {
		return nil, fmt.Errorf("failed to parse CloudShip response: %v", err)
	}

	return cloudShipResp, nil
}

// Bundle install request structure
type BundleInstallRequest struct {
	BundleLocation  string `json:"bundle_location" binding:"required"`
	EnvironmentName string `json:"environment_name" binding:"required"`
	Source          string `json:"source"` // "url" or "file"
}

// Bundle install response structure
type BundleInstallResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	EnvironmentName   string `json:"environment_name,omitempty"`
	EnvironmentID     int64  `json:"environment_id,omitempty"`
	BundlePath        string `json:"bundle_path,omitempty"`
	InstalledAgents   int    `json:"installed_agents,omitempty"`
	InstalledMCPs     int    `json:"installed_mcps,omitempty"`
	SyncCommand       string `json:"sync_command,omitempty"`
	Error             string `json:"error,omitempty"`
}

// installBundle handles the POST /bundles/install endpoint
func (h *APIHandlers) installBundle(c *gin.Context) {
	var req BundleInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, BundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, BundleInstallResponse{
			Success: false,
			Error:   "Failed to get home directory",
		})
		return
	}

	// Create bundles directory if it doesn't exist
	bundlesDir := filepath.Join(homeDir, ".config", "station", "bundles")
	if err := os.MkdirAll(bundlesDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, BundleInstallResponse{
			Success: false,
			Error:   "Failed to create bundles directory",
		})
		return
	}

	// Download or copy the bundle
	var bundlePath string
	if req.Source == "url" {
		// Download from URL
		bundlePath, err = downloadBundle(req.BundleLocation, bundlesDir)
		if err != nil {
			c.JSON(http.StatusBadRequest, BundleInstallResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to download bundle: %v", err),
			})
			return
		}
	} else {
		// Copy from file path
		bundlePath, err = copyBundle(req.BundleLocation, bundlesDir)
		if err != nil {
			c.JSON(http.StatusBadRequest, BundleInstallResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to copy bundle: %v", err),
			})
			return
		}
	}

	// Create new environment
	description := fmt.Sprintf("Environment created from bundle installation")
	environment, err := h.repos.Environments.Create(req.EnvironmentName, &description, 1) // Use default user ID 1
	if err != nil {
		c.JSON(http.StatusInternalServerError, BundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create environment: %v", err),
		})
		return
	}

	// Extract bundle to environment directory
	envDir := filepath.Join(homeDir, ".config", "station", "environments", req.EnvironmentName)
	agentCount, mcpCount, err := extractBundle(bundlePath, envDir)
	if err != nil {
		// Clean up environment if extraction failed
		h.repos.Environments.Delete(environment.ID)
		c.JSON(http.StatusInternalServerError, BundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to extract bundle: %v", err),
		})
		return
	}

	c.JSON(http.StatusCreated, BundleInstallResponse{
		Success:         true,
		Message:         fmt.Sprintf("Bundle installed successfully to environment '%s'", req.EnvironmentName),
		EnvironmentName: req.EnvironmentName,
		EnvironmentID:   environment.ID,
		BundlePath:      bundlePath,
		InstalledAgents: agentCount,
		InstalledMCPs:   mcpCount,
		SyncCommand:     fmt.Sprintf("stn sync %s", req.EnvironmentName),
	})
}

// downloadBundle downloads a bundle from a URL to the bundles directory
func downloadBundle(url, bundlesDir string) (string, error) {
	// Extract filename from URL
	parts := strings.Split(url, "/")
	filename := parts[len(parts)-1]
	if !strings.HasSuffix(filename, ".tar.gz") {
		// Generate meaningful name from URL path
		bundleName := generateBundleNameFromURL(url)
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
func copyBundle(srcPath, bundlesDir string) (string, error) {
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
func extractBundle(bundlePath, envDir string) (int, int, error) {
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

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return 0, 0, fmt.Errorf("failed to extract file %s: %v", destPath, err)
			}
			outFile.Close()

			// Set file permissions
			if err := os.Chmod(destPath, os.FileMode(header.Mode)); err != nil {
				return 0, 0, fmt.Errorf("failed to set permissions for %s: %v", destPath, err)
			}

			// Count agents and MCP configs
			if strings.HasPrefix(header.Name, "agents/") && strings.HasSuffix(header.Name, ".prompt") {
				agentCount++
			} else if strings.HasSuffix(header.Name, ".json") && !strings.Contains(header.Name, "/") {
				mcpCount++
			}

		case tar.TypeDir:
			// Directory
			if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
				return 0, 0, fmt.Errorf("failed to create directory %s: %v", destPath, err)
			}
		}
	}

	return agentCount, mcpCount, nil
}

// Bundle list response structure
type BundleListResponse struct {
	Success bool         `json:"success"`
	Bundles []BundleInfo `json:"bundles"`
	Count   int          `json:"count"`
	Error   string       `json:"error,omitempty"`
}

// BundleInfo represents an installed bundle
type BundleInfo struct {
	Name         string `json:"name"`
	FileName     string `json:"file_name"`
	FilePath     string `json:"file_path"`
	Size         int64  `json:"size"`
	ModifiedTime string `json:"modified_time"`
}

// listBundles handles the GET /bundles endpoint
func (h *APIHandlers) listBundles(c *gin.Context) {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, BundleListResponse{
			Success: false,
			Error:   "Failed to get home directory",
		})
		return
	}

	// Get bundles directory
	bundlesDir := filepath.Join(homeDir, ".config", "station", "bundles")
	
	// Check if bundles directory exists
	if _, err := os.Stat(bundlesDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, BundleListResponse{
			Success: true,
			Bundles: []BundleInfo{},
			Count:   0,
		})
		return
	}

	// Read directory contents
	files, err := os.ReadDir(bundlesDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, BundleListResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to read bundles directory: %v", err),
		})
		return
	}

	// Filter and process bundle files
	var bundles []BundleInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only include .tar.gz files
		if !strings.HasSuffix(file.Name(), ".tar.gz") {
			continue
		}

		// Get file info
		filePath := filepath.Join(bundlesDir, file.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		// Create bundle name by removing .tar.gz extension
		bundleName := strings.TrimSuffix(file.Name(), ".tar.gz")

		bundles = append(bundles, BundleInfo{
			Name:         bundleName,
			FileName:     file.Name(),
			FilePath:     filePath,
			Size:         fileInfo.Size(),
			ModifiedTime: fileInfo.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, BundleListResponse{
		Success: true,
		Bundles: bundles,
		Count:   len(bundles),
	})
}

// CloudShip bundle list response structure
type CloudShipBundleListResponse struct {
	Success bool                     `json:"success"`
	Bundles []map[string]interface{} `json:"bundles"`
	Error   string                   `json:"error,omitempty"`
}

// listCloudShipBundles handles the GET /bundles/cloudship endpoint
func (h *APIHandlers) listCloudShipBundles(c *gin.Context) {
	// Load Station config to get CloudShip registration key
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, CloudShipBundleListResponse{
			Success: false,
			Error:   "Failed to load Station config: " + err.Error(),
		})
		return
	}

	// Check if CloudShip is configured
	if !cfg.CloudShip.Enabled || cfg.CloudShip.RegistrationKey == "" {
		c.JSON(http.StatusBadRequest, CloudShipBundleListResponse{
			Success: false,
			Error:   "CloudShip is not configured. Please enable CloudShip and set registration key in config",
		})
		return
	}

	// Use default CloudShip API URL
	apiURL := "https://api.cloudshipai.com"

	// Fetch bundles from CloudShip
	listURL := fmt.Sprintf("%s/api/public/bundles", strings.TrimSuffix(apiURL, "/"))
	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CloudShipBundleListResponse{
			Success: false,
			Error:   "Failed to create request: " + err.Error(),
		})
		return
	}
	req.Header.Set("X-Registration-Key", cfg.CloudShip.RegistrationKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CloudShipBundleListResponse{
			Success: false,
			Error:   "Failed to fetch bundles from CloudShip: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, CloudShipBundleListResponse{
			Success: false,
			Error:   fmt.Sprintf("CloudShip list failed (HTTP %d): %s", resp.StatusCode, string(bodyBytes)),
		})
		return
	}

	// Parse CloudShip response
	var cloudShipResp struct {
		Bundles []map[string]interface{} `json:"bundles"`
	}
	if err := json.Unmarshal(bodyBytes, &cloudShipResp); err != nil {
		c.JSON(http.StatusInternalServerError, CloudShipBundleListResponse{
			Success: false,
			Error:   "Failed to parse CloudShip response: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CloudShipBundleListResponse{
		Success: true,
		Bundles: cloudShipResp.Bundles,
	})
}

// generateBundleNameFromURL generates a meaningful bundle name from a URL
func generateBundleNameFromURL(url string) string {
	// Parse URL to extract meaningful parts
	parts := strings.Split(url, "/")
	fmt.Printf("DEBUG: URL parts: %v\n", parts) // Debug logging
	
	// Look for repository name in common URL patterns
	var name string
	
	// GitHub pattern: https://github.com/user/repo or https://github.com/user/repo/...
	if strings.Contains(url, "github.com") {
		for i, part := range parts {
			if part == "github.com" && i+2 < len(parts) {
				name = parts[i+2] // repo name
				fmt.Printf("DEBUG: GitHub repo name: %s\n", name)
				break
			}
		}
	}
	
	// If no specific pattern matched, use last meaningful part
	if name == "" {
		fmt.Printf("DEBUG: Looking for meaningful parts...\n")
		for i := len(parts) - 1; i >= 0; i-- {
			fmt.Printf("DEBUG: Checking part[%d]: '%s'\n", i, parts[i])
			if parts[i] != "" && parts[i] != "download" && parts[i] != "releases" && 
			   !strings.Contains(parts[i], ".") {
				name = parts[i]
				fmt.Printf("DEBUG: Selected name: %s\n", name)
				break
			}
		}
	}
	
	// Fallback to generic name with timestamp
	if name == "" {
		name = fmt.Sprintf("bundle-%d", time.Now().Unix())
		fmt.Printf("DEBUG: Using timestamp fallback: %s\n", name)
	}
	
	// Clean up the name
	originalName := name
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(name, "")
	fmt.Printf("DEBUG: Final name: '%s' (from '%s')\n", name, originalName)
	
	return name
}