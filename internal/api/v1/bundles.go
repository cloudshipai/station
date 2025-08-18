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
	"strings"

	"github.com/gin-gonic/gin"
)

// Bundle request structure
type BundleRequest struct {
	Environment string `json:"environment" binding:"required"`
	Local       bool   `json:"local"`
	Endpoint    string `json:"endpoint"`
}

// Bundle response structure
type BundleResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LocalPath string `json:"local_path,omitempty"`
	ShareID   string `json:"share_id,omitempty"`
	ShareURL  string `json:"share_url,omitempty"`
	Expires   string `json:"expires,omitempty"`
	Error     string `json:"error,omitempty"`
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
		// Upload to endpoint
		if req.Endpoint == "" {
			req.Endpoint = "https://share.cloudshipai.com/upload"
		}

		shareResp, err := uploadBundle(tarData, fmt.Sprintf("%s.tar.gz", req.Environment), req.Endpoint)
		if err != nil {
			c.JSON(http.StatusInternalServerError, BundleResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to upload bundle: %v", err),
			})
			return
		}

		// Calculate share URL
		baseURL := strings.Replace(req.Endpoint, "/upload", "", 1)
		shareURL := fmt.Sprintf("%s%s", baseURL, shareResp.URL)

		c.JSON(http.StatusOK, BundleResponse{
			Success:  true,
			Message:  "Bundle uploaded successfully",
			ShareID:  shareResp.ShareID,
			ShareURL: shareURL,
			Expires:  shareResp.Expires,
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

// uploadBundle uploads the bundle to a remote endpoint using multipart/form-data
func uploadBundle(tarData []byte, filename, endpoint string) (*ShareServerResponse, error) {
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
	req, err := http.NewRequest("POST", endpoint, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	client := &http.Client{}
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var shareResp ShareServerResponse
	if err := json.Unmarshal(respBody, &shareResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &shareResp, nil
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
		filename = fmt.Sprintf("bundle-%d.tar.gz", len(parts))
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