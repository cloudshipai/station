package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"station/internal/services"
	"station/cmd/main/handlers/common"
)

// UnifiedBundleHandler provides MCP tools for the unified bundle system
// This creates API-compatible bundles that can be installed via the bundle API
type UnifiedBundleHandler struct {
	bundleService *services.BundleService
}

// NewUnifiedBundleHandler creates a new unified bundle tools handler
func NewUnifiedBundleHandler() *UnifiedBundleHandler {
	return &UnifiedBundleHandler{
		bundleService: services.NewBundleService(),
	}
}

// Note: BundleEnvironmentRequest is defined in handlers_fixed.go to avoid duplication

// BundleResponse represents the response from bundle operations
type BundleResponse struct {
	Success         bool                      `json:"success"`
	Message         string                    `json:"message"`
	BundlePath      string                    `json:"bundlePath,omitempty"`
	Size            int64                     `json:"size,omitempty"`
	BundleInfo      *services.BundleInfo      `json:"bundleInfo,omitempty"`
	InstallCommand  string                    `json:"installCommand,omitempty"`
	Error           string                    `json:"error,omitempty"`
}

// CreateBundle creates an API-compatible bundle from an environment
func (h *UnifiedBundleHandler) CreateBundle(ctx context.Context, req BundleEnvironmentRequest) (*BundleResponse, error) {
	log.Printf("MCP: Creating bundle from environment: %s", req.EnvironmentName)

	// Get Station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		return &BundleResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get station config root: %s", err.Error()),
			Message: "Cannot locate Station configuration directory",
		}, nil
	}

	// Environment directory path
	envPath := filepath.Join(configRoot, "environments", req.EnvironmentName)
	
	// Validate environment exists and is bundleable
	if err := h.bundleService.ValidateEnvironment(envPath); err != nil {
		return &BundleResponse{
			Success: false,
			Error:   err.Error(),
			Message: fmt.Sprintf("Environment '%s' validation failed", req.EnvironmentName),
		}, nil
	}

	// Get bundle info for analysis
	bundleInfo, err := h.bundleService.GetBundleInfo(envPath)
	if err != nil {
		return &BundleResponse{
			Success: false,
			Error:   err.Error(),
			Message: fmt.Sprintf("Failed to analyze environment '%s'", req.EnvironmentName),
		}, nil
	}

	// Set default output path if not provided
	outputPath := req.OutputPath
	if outputPath == "" {
		outputPath = fmt.Sprintf("%s.tar.gz", req.EnvironmentName)
	}

	// Create the bundle
	tarData, err := h.bundleService.CreateBundle(envPath)
	if err != nil {
		return &BundleResponse{
			Success: false,
			Error:   err.Error(),
			Message: fmt.Sprintf("Failed to create bundle from environment '%s'", req.EnvironmentName),
		}, nil
	}

	// Save to file
	if err := os.WriteFile(outputPath, tarData, 0644); err != nil {
		return &BundleResponse{
			Success: false,
			Error:   err.Error(),
			Message: fmt.Sprintf("Failed to save bundle to '%s'", outputPath),
		}, nil
	}

	// Create install command suggestion
	installCmd := fmt.Sprintf("curl -X POST http://localhost:8080/bundles/install -H \"Content-Type: application/json\" -d '{\"bundle_location\": \"%s\", \"environment_name\": \"new-env\", \"source\": \"file\"}'", outputPath)

	return &BundleResponse{
		Success:         true,
		Message:         fmt.Sprintf("Successfully created bundle from environment '%s'", req.EnvironmentName),
		BundlePath:      outputPath,
		Size:            int64(len(tarData)),
		BundleInfo:      bundleInfo,
		InstallCommand:  installCmd,
	}, nil
}

// GetAvailableTools returns the list of available unified bundle tools
func (h *UnifiedBundleHandler) GetAvailableTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "create_bundle_from_environment",
			Description: "Create an API-compatible bundle (.tar.gz) from a Station environment. The bundle can be installed via the Station Bundle API or UI.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"environmentName": map[string]interface{}{
						"type":        "string",
						"description": "Name of the environment to bundle (e.g., 'default', 'production')",
					},
					"outputPath": map[string]interface{}{
						"type":        "string",
						"description": "Output path for the bundle file (optional, defaults to <environment>.tar.gz)",
					},
				},
				"required": []string{"environmentName"},
			},
		},
	}
}

// HandleToolCall processes a tool call for unified bundle operations
func (h *UnifiedBundleHandler) HandleToolCall(ctx context.Context, name string, arguments map[string]interface{}) (interface{}, error) {
	switch name {
	case "create_bundle_from_environment":
		// Parse arguments
		argBytes, err := json.Marshal(arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		var req BundleEnvironmentRequest
		if err := json.Unmarshal(argBytes, &req); err != nil {
			return nil, fmt.Errorf("failed to parse create_bundle_from_environment arguments: %w", err)
		}

		return h.CreateBundle(ctx, req)

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// ToolDefinition represents a tool definition for MCP
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}