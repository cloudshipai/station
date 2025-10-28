package v1

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"station/internal/embedded"
	"station/internal/services"
)

// DemoBundleInfo represents information about a demo bundle for the API
type DemoBundleInfo struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Category        string             `json:"category"`
	Size            int                `json:"size"`
	AgentsCount     int                `json:"agents_count"`
	MCPServersCount int                `json:"mcp_servers_count"`
	Tags            []string           `json:"tags"`
	Agents          []AgentSummary     `json:"agents"`
	MCPServers      []MCPServerSummary `json:"mcp_servers"`
	RequiredVars    []string           `json:"required_vars"`
}

// AgentSummary provides basic agent information for bundle cards
type AgentSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ToolsCount  int    `json:"tools_count"`
}

// MCPServerSummary provides basic MCP server information for bundle cards
type MCPServerSummary struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// DemoBundleListResponse represents the list of demo bundles
type DemoBundleListResponse struct {
	Success bool             `json:"success"`
	Bundles []DemoBundleInfo `json:"bundles"`
	Count   int              `json:"count"`
}

// DemoBundleInstallRequest represents a request to install a demo bundle
type DemoBundleInstallRequest struct {
	BundleID        string `json:"bundle_id" binding:"required"`
	EnvironmentName string `json:"environment_name" binding:"required"`
}

// DemoBundleInstallResponse represents the installation response
type DemoBundleInstallResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	EnvironmentName string `json:"environment_name"`
	InstalledAgents int    `json:"installed_agents"`
	InstalledMCPs   int    `json:"installed_mcps"`
	Error           string `json:"error,omitempty"`
	NextStep        string `json:"next_step,omitempty"`
}

// listDemoBundles handles GET /demo-bundles
func (h *APIHandlers) listDemoBundles(c *gin.Context) {
	bundles := embedded.GetDemoBundles()
	var bundleInfos []DemoBundleInfo
	bundleService := services.NewBundleService()

	for _, bundle := range bundles {
		bundleInfo := DemoBundleInfo{
			ID:           bundle.ID,
			Name:         bundle.Name,
			Description:  bundle.Description,
			Category:     bundle.Category,
			Size:         len(bundle.Data),
			Agents:       []AgentSummary{},
			MCPServers:   []MCPServerSummary{},
			Tags:         []string{},
			RequiredVars: []string{},
		}

		// Extract manifest from bundle
		manifest, err := bundleService.ExtractManifestFromTarGz(bundle.Data)
		if err == nil && manifest != nil {
			// Populate metadata from manifest
			bundleInfo.AgentsCount = len(manifest.Agents)
			bundleInfo.MCPServersCount = len(manifest.MCPServers)
			bundleInfo.Tags = manifest.Bundle.Tags

			// Add agent summaries
			for _, agent := range manifest.Agents {
				bundleInfo.Agents = append(bundleInfo.Agents, AgentSummary{
					Name:        agent.Name,
					Description: agent.Description,
					ToolsCount:  len(agent.Tools),
				})
			}

			// Add MCP server summaries
			for _, server := range manifest.MCPServers {
				bundleInfo.MCPServers = append(bundleInfo.MCPServers, MCPServerSummary{
					Name:    server.Name,
					Command: server.Command,
				})
			}

			// Add required variables
			for _, varReq := range manifest.RequiredVariables {
				bundleInfo.RequiredVars = append(bundleInfo.RequiredVars, varReq.Name)
			}
		}

		bundleInfos = append(bundleInfos, bundleInfo)
	}

	c.JSON(http.StatusOK, DemoBundleListResponse{
		Success: true,
		Bundles: bundleInfos,
		Count:   len(bundleInfos),
	})
}

// installDemoBundle handles POST /demo-bundles/install
func (h *APIHandlers) installDemoBundle(c *gin.Context) {
	var req DemoBundleInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, DemoBundleInstallResponse{
			Success: false,
			Error:   "Invalid request: missing bundle_id or environment_name",
		})
		return
	}

	// Use the DemoBundleHandler from the MCP package
	bundleService := services.NewBundleService()

	// Get the demo bundle
	bundle := embedded.GetDemoBundle(req.BundleID)
	if bundle == nil {
		c.JSON(http.StatusNotFound, DemoBundleInstallResponse{
			Success: false,
			Error:   "Demo bundle not found",
		})
		return
	}

	// Create temporary file for the bundle
	tmpFile, err := os.CreateTemp("", "demo-bundle-*.tar.gz")
	if err != nil {
		c.JSON(http.StatusInternalServerError, DemoBundleInstallResponse{
			Success: false,
			Error:   "Failed to create temporary file",
		})
		return
	}
	defer os.Remove(tmpFile.Name())
	defer func() { _ = tmpFile.Close() }()

	// Write bundle data to temp file
	if _, err := tmpFile.Write(bundle.Data); err != nil {
		c.JSON(http.StatusInternalServerError, DemoBundleInstallResponse{
			Success: false,
			Error:   "Failed to write bundle data",
		})
		return
	}
	tmpFile.Close()

	// Install the bundle (creates filesystem structure)
	result, err := bundleService.InstallBundle(tmpFile.Name(), req.EnvironmentName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DemoBundleInstallResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if !result.Success {
		c.JSON(http.StatusBadRequest, DemoBundleInstallResponse{
			Success: false,
			Error:   result.Error,
		})
		return
	}

	// Create environment database entry
	consoleUser, err := h.repos.Users.GetByUsername("console")
	if err != nil {
		// Continue even if console user not found - environment files are already created
		c.JSON(http.StatusOK, DemoBundleInstallResponse{
			Success:         true,
			Message:         result.Message + " (Note: Environment not added to database)",
			EnvironmentName: req.EnvironmentName,
			InstalledAgents: result.InstalledAgents,
			InstalledMCPs:   result.InstalledMCPs,
			NextStep:        "Run 'stn sync " + req.EnvironmentName + "' to load the demo agents and tools",
		})
		return
	}

	// Use environment management service to create DB entry
	envService := services.NewEnvironmentManagementService(h.repos)
	description := "Demo environment for " + bundle.Name
	_, _, err = envService.CreateEnvironment(req.EnvironmentName, &description, consoleUser.ID)
	if err != nil {
		// Environment files are created, just warn about DB
		c.JSON(http.StatusOK, DemoBundleInstallResponse{
			Success:         true,
			Message:         result.Message + " (Note: Environment not added to database)",
			EnvironmentName: req.EnvironmentName,
			InstalledAgents: result.InstalledAgents,
			InstalledMCPs:   result.InstalledMCPs,
			NextStep:        "Run 'stn sync " + req.EnvironmentName + "' to load the demo agents and tools",
		})
		return
	}

	c.JSON(http.StatusOK, DemoBundleInstallResponse{
		Success:         true,
		Message:         result.Message,
		EnvironmentName: req.EnvironmentName,
		InstalledAgents: result.InstalledAgents,
		InstalledMCPs:   result.InstalledMCPs,
		NextStep:        "Run 'stn sync " + req.EnvironmentName + "' to load the demo agents and tools",
	})
}
