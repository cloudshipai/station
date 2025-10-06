package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleListDemoBundles lists all available embedded demo bundles
func (s *Server) handleListDemoBundles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	handler := NewDemoBundleHandler()
	response, err := handler.ListDemoBundles(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to marshal response"), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleInstallDemoBundle installs an embedded demo bundle
func (s *Server) handleInstallDemoBundle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	bundleID, err := request.RequireString("bundle_id")
	if err != nil {
		return mcp.NewToolResultError("missing bundle_id parameter"), nil
	}

	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError("missing environment_name parameter"), nil
	}

	req := DemoBundleInstallRequest{
		BundleID:        bundleID,
		EnvironmentName: environmentName,
	}

	handler := NewDemoBundleHandler()
	response, err := handler.InstallDemoBundle(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to marshal response"), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
