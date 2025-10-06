package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"station/cmd/main/handlers/common"
	"station/internal/embedded"
	"station/internal/services"
)

// DemoBundleHandler provides MCP tools for embedded demo bundles
type DemoBundleHandler struct {
	bundleService *services.BundleService
}

// NewDemoBundleHandler creates a new demo bundle tools handler
func NewDemoBundleHandler() *DemoBundleHandler {
	return &DemoBundleHandler{
		bundleService: services.NewBundleService(),
	}
}

// DemoBundleInfo represents information about an available demo bundle
type DemoBundleInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Size        int    `json:"size"`
}

// DemoBundleListResponse represents the response from listing demo bundles
type DemoBundleListResponse struct {
	Success bool             `json:"success"`
	Bundles []DemoBundleInfo `json:"bundles"`
	Count   int              `json:"count"`
}

// DemoBundleInstallRequest represents a request to install a demo bundle
type DemoBundleInstallRequest struct {
	BundleID        string `json:"bundle_id"`
	EnvironmentName string `json:"environment_name"`
}

// DemoBundleInstallResponse represents the response from installing a demo bundle
type DemoBundleInstallResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	EnvironmentName string `json:"environment_name"`
	InstalledAgents int    `json:"installed_agents"`
	InstalledMCPs   int    `json:"installed_mcps"`
	Error           string `json:"error,omitempty"`
	NextStep        string `json:"next_step,omitempty"`
}

// ListDemoBundles lists all available embedded demo bundles
func (h *DemoBundleHandler) ListDemoBundles(ctx context.Context) (*DemoBundleListResponse, error) {
	log.Println("MCP: Listing available demo bundles")

	bundles := embedded.GetDemoBundles()
	var bundleInfos []DemoBundleInfo

	for _, bundle := range bundles {
		bundleInfos = append(bundleInfos, DemoBundleInfo{
			ID:          bundle.ID,
			Name:        bundle.Name,
			Description: bundle.Description,
			Category:    bundle.Category,
			Size:        len(bundle.Data),
		})
	}

	return &DemoBundleListResponse{
		Success: true,
		Bundles: bundleInfos,
		Count:   len(bundleInfos),
	}, nil
}

// InstallDemoBundle installs an embedded demo bundle to a new environment
func (h *DemoBundleHandler) InstallDemoBundle(ctx context.Context, req DemoBundleInstallRequest) (*DemoBundleInstallResponse, error) {
	log.Printf("MCP: Installing demo bundle '%s' to environment '%s'", req.BundleID, req.EnvironmentName)

	// Get the demo bundle
	bundle := embedded.GetDemoBundle(req.BundleID)
	if bundle == nil {
		return &DemoBundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Demo bundle '%s' not found", req.BundleID),
			Message: "Invalid demo bundle ID",
		}, nil
	}

	// Get Station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		return &DemoBundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get station config root: %s", err.Error()),
			Message: "Cannot locate Station configuration directory",
		}, nil
	}

	// Create temporary file for the bundle
	tmpDir := filepath.Join(os.TempDir(), "station-demo-bundles")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return &DemoBundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create temp directory: %s", err.Error()),
			Message: "Cannot create temporary bundle storage",
		}, nil
	}

	tmpBundlePath := filepath.Join(tmpDir, fmt.Sprintf("%s.tar.gz", req.BundleID))
	if err := os.WriteFile(tmpBundlePath, bundle.Data, 0644); err != nil {
		return &DemoBundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to write bundle file: %s", err.Error()),
			Message: "Cannot save demo bundle to temporary location",
		}, nil
	}
	defer os.Remove(tmpBundlePath) // Clean up temp file after installation

	// Check if environment already exists
	envPath := filepath.Join(configRoot, "environments", req.EnvironmentName)
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		return &DemoBundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Environment '%s' already exists", req.EnvironmentName),
			Message: "Choose a different environment name",
		}, nil
	}

	// Install the bundle using BundleService
	result, err := h.bundleService.InstallBundle(tmpBundlePath, req.EnvironmentName)
	if err != nil {
		return &DemoBundleInstallResponse{
			Success: false,
			Error:   fmt.Sprintf("Installation failed: %s", err.Error()),
			Message: "Failed to install demo bundle",
		}, nil
	}

	if !result.Success {
		return &DemoBundleInstallResponse{
			Success: false,
			Error:   result.Error,
			Message: result.Message,
		}, nil
	}

	return &DemoBundleInstallResponse{
		Success:         true,
		Message:         fmt.Sprintf("Demo bundle '%s' installed successfully", bundle.Name),
		EnvironmentName: req.EnvironmentName,
		InstalledAgents: result.InstalledAgents,
		InstalledMCPs:   result.InstalledMCPs,
		NextStep:        fmt.Sprintf("Run 'stn sync %s' to load the demo agents and tools", req.EnvironmentName),
	}, nil
}
