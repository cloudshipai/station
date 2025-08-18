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